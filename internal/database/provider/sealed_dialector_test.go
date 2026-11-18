package provider

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// poolFailure is a database/sql pool whose every statement and every
// transaction fails with the configured error, and whose connections fail to
// close with closeErr. It lets a test hand GORM the exact driver error a real
// server or socket would, of the real driver types, on the PostgreSQL and the
// MySQL dialector alike, with no server.
type poolFailure struct {
	stmtErr, beginErr, closeErr error
}

func (c poolFailure) Connect(context.Context) (driver.Conn, error) { return poolFailureConn{c}, nil }
func (c poolFailure) Driver() driver.Driver                        { return poolFailureDriver{} }

type poolFailureDriver struct{}

func (poolFailureDriver) Open(string) (driver.Conn, error) { return nil, errors.New("unused") }

type poolFailureConn struct{ c poolFailure }

func (f poolFailureConn) Prepare(string) (driver.Stmt, error) { return nil, f.c.stmtErr }
func (f poolFailureConn) Close() error                        { return f.c.closeErr }
func (f poolFailureConn) Begin() (driver.Tx, error)           { return nil, f.c.beginErr }

func poolFailing(t *testing.T, c poolFailure) *sql.DB {
	t.Helper()
	pool := sql.OpenDB(c)
	t.Cleanup(func() { _ = pool.Close() })
	return pool
}

type fakeDialect struct {
	name string
	new  func(pool *sql.DB) gorm.Dialector
}

var fakeDialects = []fakeDialect{
	{"postgres", func(p *sql.DB) gorm.Dialector { return postgres.New(postgres.Config{Conn: p}) }},
	{"mysql", func(p *sql.DB) gorm.Dialector {
		return gormmysql.New(gormmysql.Config{Conn: p, SkipInitializeWithVersion: true})
	}},
}

func fakeConfig() *gorm.Config {
	return &gorm.Config{DisableAutomaticPing: true, Logger: logger.Discard}
}

type opener func(gorm.Dialector, *gorm.Config) (*gorm.DB, error)

// execError opens GORM over a pool that fails with err and returns what a
// statement and a transaction report.
func execError(t *testing.T, d fakeDialect, open opener, err error) (stmt, begin error) {
	t.Helper()
	pool := poolFailing(t, poolFailure{stmtErr: err, beginErr: err})
	db, openErr := open(d.new(pool), fakeConfig())
	require.NoError(t, openErr)
	return db.Exec("SELECT 1").Error, db.Begin().Error
}

func plainOpen(dialector gorm.Dialector, config *gorm.Config) (*gorm.DB, error) {
	return gorm.Open(dialector, config)
}

// The driver errors a pool returns after Connect, each with its own address,
// path or value in the text. leak is the part of the raw text that must never
// reach an output.
func openConnectionErrors() []struct {
	name, leak, want string
	err              error
} {
	netErr := &net.OpError{
		Op: "write", Net: "tcp",
		Addr: &net.TCPAddr{IP: net.IPv4(10, 9, 8, 7), Port: 5432},
		Err:  os.NewSyscallError("write", syscall.EPIPE),
	}
	return []struct {
		name, leak, want string
		err              error
	}{
		{
			"PostgreSQL error with a quoted identifier", leakWord,
			"database operation failed: the database rejected the statement (SQLSTATE 42P01, undefined table)",
			&pgconn.PgError{Code: "42P01", Message: `relation "` + leakWord + `" does not exist`},
		},
		{
			"MySQL error with a quoted identifier", leakWord,
			"database operation failed: the database rejected the statement (MySQL error 1146, table does not exist)",
			&mysqldriver.MySQLError{Number: 1146, Message: "Table '" + leakWord + "' doesn't exist"},
		},
		{
			"transport error naming the server address", "10.9.8.7",
			"database operation failed: connection reset",
			netErr,
		},
		{
			"the same wrapped with a value", leakWord,
			"database operation failed: connection reset",
			fmt.Errorf("saving %s: %w", leakWord, netErr),
		},
		{
			"an error of a type nobody listed", leakWord,
			"database operation failed: unclassified error",
			errors.New("the driver says " + leakWord),
		},
	}
}

func TestSealedDialector_StatementAndTransactionErrorsHoldNoDriverText(t *testing.T) {
	for _, d := range fakeDialects {
		for _, tc := range openConnectionErrors() {
			t.Run(d.name+"/"+tc.name, func(t *testing.T) {
				// Control: without the wrapper the same pool prints the leak, so the
				// harness reaches the sites it claims to.
				rawStmt, rawBegin := execError(t, d, plainOpen, tc.err)
				require.Error(t, rawStmt)
				require.Error(t, rawBegin)
				assert.Contains(t, rawStmt.Error(), tc.leak, "control: statement")
				assert.Contains(t, rawBegin.Error(), tc.leak, "control: transaction")

				stmt, begin := execError(t, d, openGORM, tc.err)
				for site, got := range map[string]error{"statement": stmt, "transaction": begin} {
					require.Error(t, got, site)
					assert.NotContains(t, got.Error(), tc.leak, site)
					assert.Equal(t, tc.want, got.Error(), site)
				}
			})
		}
	}
}

func TestSealedDialector_KeepsTheErrorsCallersTest(t *testing.T) {
	t.Run("constraint errors the dialector translates", func(t *testing.T) {
		cases := []struct {
			dialect fakeDialect
			err     error
			want    error
		}{
			{fakeDialects[0], &pgconn.PgError{Code: "23505", Message: "duplicate key " + leakWord}, gorm.ErrDuplicatedKey},
			{fakeDialects[0], &pgconn.PgError{Code: "23503", Message: "foreign key " + leakWord}, gorm.ErrForeignKeyViolated},
			{fakeDialects[1], &mysqldriver.MySQLError{Number: 1062, Message: "Duplicate entry '" + leakWord + "'"}, gorm.ErrDuplicatedKey},
			{fakeDialects[1], &mysqldriver.MySQLError{Number: 1452, Message: "foreign key " + leakWord}, gorm.ErrForeignKeyViolated},
		}
		for _, tc := range cases {
			stmt, _ := execError(t, tc.dialect, openGORM, tc.err)
			require.ErrorIs(t, stmt, tc.want, tc.dialect.name)
			assert.NotContains(t, stmt.Error(), leakWord)
		}
	})

	t.Run("sentinels keep their identity", func(t *testing.T) {
		for _, sentinel := range []error{
			gorm.ErrRecordNotFound, gorm.ErrInvalidTransaction, gorm.ErrMissingWhereClause,
			gorm.ErrDuplicatedKey, context.Canceled, context.DeadlineExceeded,
			driver.ErrBadConn, sql.ErrNoRows, sql.ErrTxDone, sql.ErrConnDone,
		} {
			assert.True(t, sealOpenError(sentinel) == sentinel, "%v", sentinel)
		}
		assert.NoError(t, sealOpenError(nil))
	})

	t.Run("a context error through a pool still answers errors.Is", func(t *testing.T) {
		stmt, _ := execError(t, fakeDialects[0], openGORM, fmt.Errorf("%s: %w", leakWord, context.DeadlineExceeded))
		require.ErrorIs(t, stmt, context.DeadlineExceeded)
		assert.NotContains(t, stmt.Error(), leakWord)
	})

	t.Run("an error already sealed is not sealed twice", func(t *testing.T) {
		sealed := classifyConnectError(syscall.ECONNREFUSED)
		assert.True(t, sealOpenError(sealed) == error(sealed))
	})

	t.Run("a real SQLite unique violation is still a duplicated key", func(t *testing.T) {
		s, _ := sqliteSiteProvider(t)
		require.NoError(t, s.Migrate(&siteProbeRow{}))
		require.NoError(t, s.GetDB().Create(&siteProbeRow{Name: "a"}).Error)

		err := s.GetDB().Create(&siteProbeRow{Name: "a"}).Error

		require.ErrorIs(t, err, gorm.ErrDuplicatedKey)
	})
}

// The wrapper hides what the inner dialector configures unless it delegates, and
// GORM finds those behaviours by type assertion, so a missing delegation fails
// nothing until a deployment meets it. These assert the effect.
func TestSealedDialector_KeepsWhatTheInnerDialectorConfigures(t *testing.T) {
	pool := poolFailing(t, poolFailure{})

	t.Run("PostgreSQL identifier length", func(t *testing.T) {
		db, err := openGORM(postgres.New(postgres.Config{Conn: pool}), fakeConfig())
		require.NoError(t, err)
		naming, ok := db.NamingStrategy.(schema.NamingStrategy)
		require.True(t, ok, "the naming strategy GORM holds")
		assert.Equal(t, 63, naming.IdentifierMaxLength)
	})

	t.Run("MySQL clock rounds to the datetime precision", func(t *testing.T) {
		db, err := openGORM(gormmysql.New(gormmysql.Config{Conn: pool, SkipInitializeWithVersion: true}), fakeConfig())
		require.NoError(t, err)
		require.NotNil(t, db.NowFunc)
		assert.Zero(t, db.NowFunc().Nanosecond()%int(time.Millisecond))
	})

	t.Run("a nested transaction rolls back to its savepoint", func(t *testing.T) {
		s, _ := sqliteSiteProvider(t)
		require.NoError(t, s.Migrate(&siteProbeRow{}))
		db := s.GetDB()

		var nestedErr error
		err := db.Transaction(func(tx *gorm.DB) error {
			require.NoError(t, tx.Create(&siteProbeRow{Name: "a"}).Error)
			nestedErr = tx.Transaction(func(inner *gorm.DB) error {
				// A row that does insert, then an error: only a real rollback to the
				// savepoint removes "b" (a duplicate insert changes nothing either way).
				require.NoError(t, inner.Create(&siteProbeRow{Name: "b"}).Error)
				return errors.New("abort the nested transaction")
			})
			return nil
		})

		require.NoError(t, err)
		require.Error(t, nestedErr)
		var names []string
		require.NoError(t, db.Model(&siteProbeRow{}).Order("name").Pluck("name", &names).Error)
		assert.Equal(t, []string{"a"}, names, "the nested insert was rolled back, the outer one kept")
	})
}

// A pool whose first statement answers one row (the migrator's "table exists"
// count) and whose later statements fail: the migrator then reads the table's
// columns with Rows, which stores the driver's error without passing the
// dialector.
type countThenFail struct {
	calls *atomic.Int32
	err   error
}

func (c countThenFail) Connect(context.Context) (driver.Conn, error) {
	return countThenFailConn{c}, nil
}
func (c countThenFail) Driver() driver.Driver { return poolFailureDriver{} }

type countThenFailConn struct{ c countThenFail }

func (f countThenFailConn) Prepare(string) (driver.Stmt, error) { return countThenFailStmt(f), nil }
func (f countThenFailConn) Close() error                        { return nil }
func (f countThenFailConn) Begin() (driver.Tx, error)           { return nil, f.c.err }

type countThenFailStmt struct{ c countThenFail }

func (s countThenFailStmt) Close() error  { return nil }
func (s countThenFailStmt) NumInput() int { return -1 }
func (s countThenFailStmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, s.c.err
}
func (s countThenFailStmt) Query([]driver.Value) (driver.Rows, error) {
	if s.c.calls.Add(1) == 1 {
		return &oneRow{}, nil
	}
	return nil, s.c.err
}

type oneRow struct{ done bool }

func (r *oneRow) Columns() []string { return []string{"count"} }
func (r *oneRow) Close() error      { return nil }
func (r *oneRow) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = int64(1)
	return nil
}

func TestSealedDialector_MigrateSealsAnErrorTheDialectorNeverSees(t *testing.T) {
	driverErr := fmt.Errorf("read tcp 10.9.8.7:5432: %s: %w", leakWord, syscall.ECONNRESET)
	logger, readLog := fileLogger(t)
	pool := sql.OpenDB(countThenFail{calls: &atomic.Int32{}, err: driverErr})
	t.Cleanup(func() { _ = pool.Close() })
	db, err := openGORM(postgres.New(postgres.Config{Conn: pool}), fakeConfig())
	require.NoError(t, err)
	p := NewPostgreSQLProvider(logger)
	p.db, p.connected = db, true

	err = p.Migrate(&siteProbeRow{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "migration failed: database operation failed: connection reset")
	assert.NotContains(t, err.Error(), leakWord)
	assert.NotContains(t, readLog(), leakWord, "the Migrate log line")
}

// Scan, Rows and the migrator's column read store the driver's error on the
// statement without AddError, so the dialector never sees it. Each is a path the
// application uses (the metrics service scans, the start-up fixups read columns).
func TestSealedDialector_RowPathErrorsAreSealed(t *testing.T) {
	driverErr := fmt.Errorf("read tcp 10.9.8.7:5432: %s: %w", leakWord, syscall.ECONNRESET)
	sites := map[string]func(db *gorm.DB) error{
		"Scan": func(db *gorm.DB) error {
			var n int
			return db.Raw("SELECT 1").Scan(&n).Error
		},
		"Rows": func(db *gorm.DB) error {
			rows, err := db.Raw("SELECT 1").Rows()
			if rows != nil {
				_ = rows.Close()
			}
			return err
		},
		"Migrator.ColumnTypes": func(db *gorm.DB) error {
			_, err := db.Migrator().ColumnTypes(&siteProbeRow{})
			return err
		},
	}
	for _, d := range fakeDialects {
		for name, site := range sites {
			t.Run(d.name+"/"+name, func(t *testing.T) {
				open := func(o opener) error {
					pool := poolFailing(t, poolFailure{stmtErr: driverErr})
					db, err := o(d.new(pool), fakeConfig())
					require.NoError(t, err)
					return site(db)
				}

				raw := open(plainOpen)
				require.Error(t, raw)
				assert.Contains(t, raw.Error(), leakWord, "control: the site shows the leak without the seal")

				sealed := open(openGORM)
				require.Error(t, sealed)
				assert.NotContains(t, sealed.Error(), leakWord)
				assert.NotContains(t, sealed.Error(), "10.9.8.7")
				assert.Contains(t, sealed.Error(), "database operation failed: connection reset")
			})
		}
	}
}

// Mutants: the checks above must fail on a wrapper that is wrong in the way each
// one guards against, or they prove nothing.
type withoutApply struct{ gorm.Dialector }

type withoutInnerTranslate struct{ sealedDialector }

func (d withoutInnerTranslate) Translate(err error) error { return sealOpenError(err) }

func TestSealedDialector_ChecksFailOnWrongWrappers(t *testing.T) {
	pool := poolFailing(t, poolFailure{})

	t.Run("a wrapper that drops Apply loses the PostgreSQL identifier length", func(t *testing.T) {
		db, err := gorm.Open(withoutApply{postgres.New(postgres.Config{Conn: pool})}, fakeConfig())
		require.NoError(t, err)
		naming, ok := db.NamingStrategy.(schema.NamingStrategy)
		require.True(t, ok)
		assert.NotEqual(t, 63, naming.IdentifierMaxLength)
	})

	t.Run("a wrapper that drops Apply loses the MySQL clock rounding", func(t *testing.T) {
		db, err := gorm.Open(withoutApply{gormmysql.New(gormmysql.Config{Conn: pool, SkipInitializeWithVersion: true})}, fakeConfig())
		require.NoError(t, err)
		assert.NotZero(t, db.NowFunc().Nanosecond()%int(time.Millisecond))
	})

	t.Run("a wrapper that skips the inner Translate loses the duplicated key", func(t *testing.T) {
		mutant := func(dialector gorm.Dialector, config *gorm.Config) (*gorm.DB, error) {
			config.TranslateError = true
			return gorm.Open(withoutInnerTranslate{sealedDialector{Dialector: dialector}}, config)
		}
		stmt, _ := execError(t, fakeDialects[0], mutant, &pgconn.PgError{Code: "23505"})
		require.Error(t, stmt)
		assert.NotErrorIs(t, stmt, gorm.ErrDuplicatedKey)
	})
}

// Translate must not change what Migrate writes: the dialector is the same, only
// its errors are sealed.
func TestSealedDialector_MigrateWritesTheSameSchemaTwice(t *testing.T) {
	schemaOf := func(db *gorm.DB) []string {
		var out []string
		require.NoError(t, db.Raw("SELECT sql FROM sqlite_master WHERE sql IS NOT NULL ORDER BY name").Scan(&out).Error)
		return out
	}
	s, _ := sqliteSiteProvider(t)
	require.NoError(t, s.Migrate(&siteProbeRow{}))
	first := schemaOf(s.GetDB())
	require.NoError(t, s.Migrate(&siteProbeRow{}))

	plain, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "plain.db")), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, plain.AutoMigrate(&siteProbeRow{}))

	assert.NotEmpty(t, first)
	assert.Equal(t, first, schemaOf(s.GetDB()), "a second migrate changed the schema")
	assert.Equal(t, schemaOf(plain), first, "the wrapper changed the DDL")
}

func TestSealedDialector_CloseErrorsHoldNoDriverText(t *testing.T) {
	closeErr := fmt.Errorf("closing %s: %w", leakWord, &net.OpError{
		Op: "close", Net: "tcp",
		Addr: &net.TCPAddr{IP: net.IPv4(10, 9, 8, 7), Port: 5432},
		Err:  os.NewSyscallError("close", syscall.ECONNRESET),
	})
	newDB := func(t *testing.T, d fakeDialect) *gorm.DB {
		pool := poolFailing(t, poolFailure{stmtErr: errors.New("unused"), closeErr: closeErr})
		conn, err := pool.Conn(context.Background())
		require.NoError(t, err)
		require.NoError(t, conn.Close(), "returns the connection to the pool, where Close meets it")
		db, err := openGORM(d.new(pool), fakeConfig())
		require.NoError(t, err)
		return db
	}
	logger, readLog := fileLogger(t)

	closers := map[string]func(t *testing.T) error{
		"postgresql": func(t *testing.T) error {
			p := NewPostgreSQLProvider(logger)
			p.db, p.connected = newDB(t, fakeDialects[0]), true
			return p.Close()
		},
		"mysql": func(t *testing.T) error {
			m := NewMySQLProvider(logger)
			m.db, m.connected = newDB(t, fakeDialects[1]), true
			return m.Close()
		},
		"sqlite": func(t *testing.T) error {
			s := NewSQLiteProvider(logger)
			s.db, s.connected = newDB(t, fakeDialects[0]), true
			return s.Close()
		},
	}
	for name, closeProvider := range closers {
		t.Run(name, func(t *testing.T) {
			err := closeProvider(t)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to close database: database operation failed: connection reset")
			assert.NotContains(t, err.Error(), leakWord)
			assert.NotContains(t, err.Error(), "10.9.8.7")
		})
	}
	assert.NotContains(t, readLog(), "10.9.8.7")
}

// GORM prints its trace line after every callback, so the error it prints is the
// sealed one, on each path: a statement, Scan and Rows.
func TestSealedDialector_TraceLineHoldsNoDriverText(t *testing.T) {
	driverErr := fmt.Errorf("read tcp 10.9.8.7:5432: %s: %w", leakWord, syscall.ECONNRESET)
	sites := map[string]func(db *gorm.DB){
		"Exec": func(db *gorm.DB) { _ = db.Exec("SELECT 1").Error },
		"Scan": func(db *gorm.DB) {
			var n int
			_ = db.Raw("SELECT 1").Scan(&n).Error
		},
		"Rows": func(db *gorm.DB) {
			if rows, err := db.Raw("SELECT 1").Rows(); err == nil {
				_ = rows.Close()
			}
		},
	}
	for name, site := range sites {
		t.Run(name, func(t *testing.T) {
			var out []string
			config := fakeConfig()
			config.Logger = logger.New(testLogWriter(func(line string) { out = append(out, line) }),
				logger.Config{LogLevel: logger.Info, ParameterizedQueries: true})
			pool := poolFailing(t, poolFailure{stmtErr: driverErr})
			db, err := openGORM(postgres.New(postgres.Config{Conn: pool}), config)
			require.NoError(t, err)

			site(db)

			require.NotEmpty(t, out, "the trace line was printed, or this proves nothing")
			for _, line := range out {
				assert.NotContains(t, line, leakWord)
				assert.NotContains(t, line, "10.9.8.7")
			}
		})
	}
}

type testLogWriter func(string)

func (w testLogWriter) Printf(format string, args ...interface{}) { w(fmt.Sprintf(format, args...)) }

// GORM must be opened in one place only: a call that skips openGORM drops the
// seal, and nothing else notices.
func TestSealedDialector_OpenGORMIsTheOnlyGormOpen(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	checked := 0
	for _, file := range files {
		if filepath.Base(file) == "sealed_dialector.go" || filepath.Ext(file) != ".go" || len(file) > 8 && file[len(file)-8:] == "_test.go" {
			continue
		}
		body, err := os.ReadFile(file)
		require.NoError(t, err)
		assert.NotContains(t, string(body), "gorm.Open(", file)
		checked++
	}
	assert.Greater(t, checked, 5, "the scan saw the package's files")
}
