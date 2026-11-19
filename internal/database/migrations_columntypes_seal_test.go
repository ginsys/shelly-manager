package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	columnTypesLeakWord = "LEAKWORD624"
	columnTypesLeakHost = "10.9.8.7"
)

// closeFailsPool answers every statement with one row of one column, and fails
// the Close of the migrator's plain "SELECT * FROM <table> LIMIT ?" read. GORM's
// ColumnTypes returns that Close error as it is: it passes neither the dialector
// nor the row callback, so only the caller can seal it.
type closeFailsPool struct{ err error }

func (p closeFailsPool) Connect(context.Context) (driver.Conn, error) { return closeFailsConn(p), nil }
func (p closeFailsPool) Driver() driver.Driver                        { return closeFailsDriver{} }

type closeFailsDriver struct{}

func (closeFailsDriver) Open(string) (driver.Conn, error) { return nil, errors.New("unused") }

type closeFailsConn struct{ err error }

func (c closeFailsConn) Prepare(query string) (driver.Stmt, error) {
	return closeFailsStmt{err: c.err, query: query}, nil
}
func (c closeFailsConn) Close() error              { return nil }
func (c closeFailsConn) Begin() (driver.Tx, error) { return nil, c.err }

type closeFailsStmt struct {
	err   error
	query string
}

func (s closeFailsStmt) Close() error  { return nil }
func (s closeFailsStmt) NumInput() int { return -1 }
func (s closeFailsStmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, s.err
}
func (s closeFailsStmt) Query([]driver.Value) (driver.Rows, error) {
	failClose := strings.Contains(s.query, "LIMIT") && !strings.Contains(s.query, "information_schema") &&
		strings.Contains(s.query, configTemplatesTable)
	r := &closeFailsRows{}
	if failClose {
		r.closeErr = s.err
	}
	return r, nil
}

type closeFailsRows struct {
	done     bool
	closeErr error
}

func (r *closeFailsRows) Columns() []string { return []string{"v"} }
func (r *closeFailsRows) Close() error      { return r.closeErr }
func (r *closeFailsRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = "x"
	return nil
}

// openCloseFailsDB is a plain GORM DB (no sealed dialector) over that pool, the
// way the start-up fixups see the database before the provider's Migrate runs.
func openCloseFailsDB(t *testing.T) *gorm.DB {
	t.Helper()
	driverErr := fmt.Errorf("read tcp %s:3306: %s", columnTypesLeakHost, columnTypesLeakWord)
	pool := sql.OpenDB(closeFailsPool{err: driverErr})
	t.Cleanup(func() { _ = pool.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: pool, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	return db
}

// The migrator's ColumnTypes returns the driver's rows.Close error as it is, so
// a MySQL server's text could reach the manager's start-up error. The fixup
// seals it at the call site.
func TestConfigTemplateColumnNullable_SealsTheMigratorCloseError(t *testing.T) {
	db := openCloseFailsDB(t)

	t.Run("control: the migrator's own error carries the driver text", func(t *testing.T) {
		_, err := db.Migrator().ColumnTypes(&ConfigTemplate{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), columnTypesLeakWord, "without this the test proves nothing about the seal")
	})

	t.Run("the fixup's error carries none of it", func(t *testing.T) {
		_, _, err := configTemplateColumnNullable(db, "scope")
		require.Error(t, err)
		assert.NotContains(t, err.Error(), columnTypesLeakWord)
		assert.NotContains(t, err.Error(), columnTypesLeakHost)
		assert.Contains(t, err.Error(), "database operation failed")
		assert.Contains(t, err.Error(), "failed to inspect "+configTemplatesTable+" columns")
	})
}
