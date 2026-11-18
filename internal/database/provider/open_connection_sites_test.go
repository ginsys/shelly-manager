package provider

import (
	"encoding/binary"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// What an error of an open connection can print, site by site. The sites are the
// ones ginsys/shelly-manager#601 names: Migrate, BeginTransaction, the GORM log
// and a query made through GetDB. Each test is also the measurement: it fails
// where the driver's sentence reaches the output, and the log lines of its
// control show what the sentence is.

// sqliteSiteProvider connects a SQLite provider whose log is read back.
func sqliteSiteProvider(t *testing.T) (*SQLiteProvider, func() string) {
	t.Helper()
	logger, readLog := fileLogger(t)
	s := NewSQLiteProvider(logger)
	require.NoError(t, s.Connect(DatabaseConfig{
		Provider: "sqlite",
		DSN:      filepath.Join(t.TempDir(), "site.db"),
		LogLevel: "info",
	}))
	t.Cleanup(func() { _ = s.Close() })
	return s, readLog
}

type siteProbeRow struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"uniqueIndex"`
}

func (siteProbeRow) TableName() string { return "site_probe_rows" }

// SQLite quotes the identifiers of the failing statement and nothing else.
func TestSQLiteSites_QueryErrorHoldsNoDriverSentence(t *testing.T) {
	s, readLog := sqliteSiteProvider(t)

	err := s.GetDB().Exec("SELECT * FROM " + leakWord + "_missing").Error

	require.Error(t, err)
	t.Logf("GetDB query error today: %q", err.Error())
	assert.NotContains(t, err.Error(), "no such table", "the driver's sentence")
	assert.Contains(t, err.Error(), "database operation failed")
	assert.NotContains(t, readLog(), "no such table", "the GORM trace line carries the error too")
}

func TestSQLiteSites_MigrateErrorHoldsNoDriverSentence(t *testing.T) {
	s, readLog := sqliteSiteProvider(t)
	// A view in the table's place: AutoMigrate finds no table and creates one.
	require.NoError(t, s.GetDB().Exec("CREATE TABLE site_base (id INTEGER PRIMARY KEY, name TEXT)").Error)
	require.NoError(t, s.GetDB().Exec("CREATE VIEW site_probe_rows AS SELECT id, name FROM site_base").Error)

	err := s.Migrate(&siteProbeRow{})

	require.Error(t, err)
	t.Logf("Migrate error today: %q", err.Error())
	assert.NotContains(t, err.Error(), "already exists", "the driver's sentence")
	assert.Contains(t, err.Error(), "database operation failed")
	assert.NotContains(t, readLog(), "already exists", "the Migrate log line and the trace line")
}

func TestSQLiteSites_BeginTransactionErrorIsSealed(t *testing.T) {
	s, _ := sqliteSiteProvider(t)
	sqlDB, err := s.GetDB().DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close(), "closing the pool makes the next transaction fail")

	_, err = s.BeginTransaction()

	require.Error(t, err)
	t.Logf("BeginTransaction error today: %q", err.Error())
	assert.Contains(t, err.Error(), "database operation failed")
}

// droppablePostgres is the fake PostgreSQL on a unix socket whose directory is
// named leakWord. While drop is set it accepts a connection, completes the
// startup, and closes the connection at the first statement: what a server
// restart or a firewall does to a pool.
type droppablePostgres struct {
	drop  atomic.Bool
	mu    sync.Mutex
	conns []net.Conn
}

// cut closes every connection the server holds, as a restart would.
func (d *droppablePostgres) cut() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, c := range d.conns {
		_ = c.Close()
	}
	d.conns = nil
}

func serveDroppablePostgres(t *testing.T, listener net.Listener) *droppablePostgres {
	t.Helper()
	d := &droppablePostgres{}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			d.mu.Lock()
			d.conns = append(d.conns, conn)
			d.mu.Unlock()
			go d.serve(conn)
		}
	}()
	return d
}

func (d *droppablePostgres) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	message := func(kind byte, body ...byte) []byte {
		out := []byte{kind, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(out[1:], uint32(len(body)+4))
		return append(out, body...)
	}
	var length [4]byte
	if _, err := io.ReadFull(conn, length[:]); err != nil {
		return
	}
	if _, err := io.CopyN(io.Discard, conn, int64(binary.BigEndian.Uint32(length[:]))-4); err != nil {
		return
	}
	ready := message('Z', 'I')
	if _, err := conn.Write(append(message('R', 0, 0, 0, 0), ready...)); err != nil {
		return
	}
	for {
		var header [5]byte
		if _, err := io.ReadFull(conn, header[:]); err != nil {
			return
		}
		if _, err := io.CopyN(io.Discard, conn, int64(binary.BigEndian.Uint32(header[1:]))-4); err != nil {
			return
		}
		if header[0] == 'X' || (d.drop.Load() && header[0] != 'S') {
			return
		}
		var reply []byte
		switch header[0] {
		case 'Q':
			reply = append(message('I'), ready...)
		case 'S':
			reply = ready
		default:
			reply = message('E', []byte("SERROR\x00C0A000\x00Mnot supported by the test server\x00\x00")...)
		}
		if _, err := conn.Write(reply); err != nil {
			return
		}
	}
}

// pgx names the socket in the error of a cut connection; the socket directory
// stands for the address or path an operator would not want printed.
func TestPostgreSQLSites_OpenConnectionErrorsHoldNoSocketPath(t *testing.T) {
	dir, err := os.MkdirTemp("", "sk")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketDir := filepath.Join(dir, leakWord)
	require.NoError(t, os.Mkdir(socketDir, 0o700))
	socket := filepath.Join(socketDir, ".s.PGSQL.5432")
	require.Less(t, len(socket), 100, "a unix socket path is limited to 108 bytes")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	server := serveDroppablePostgres(t, listener)

	logger, readLog := fileLogger(t)
	p := NewPostgreSQLProvider(logger)
	t.Cleanup(func() { _ = p.Close() })
	require.NoError(t, p.Connect(DatabaseConfig{
		Provider: "postgresql",
		DSN:      "postgresql://app@localhost:5432/shelly?host=" + url.QueryEscape(socketDir) + "&sslmode=disable&connect_timeout=2",
		LogLevel: "info",
	}))

	// The pool holds a live idle connection, which the server then cuts: the
	// first site to use it writes to a dead socket (pgx prints the socket path).
	// Every later connection is dropped at its first statement (pgx prints
	// "conn closed").
	require.NoError(t, p.Ping())
	server.cut()
	server.drop.Store(true)

	var seen []string
	_, beginErr := p.BeginTransaction()
	seen = append(seen, "BeginTransaction: "+errText(beginErr))

	execErr := p.GetDB().Exec("UPDATE site_probe_rows SET name = 'x'").Error
	seen = append(seen, "GetDB query: "+errText(execErr))

	migrateErr := p.Migrate(&siteProbeRow{})
	seen = append(seen, "Migrate: "+errText(migrateErr))

	for _, line := range seen {
		t.Log(line)
	}
	logged := readLog()
	assert.Contains(t, logged, "Database migration failed", "the Migrate site ran, or this proves nothing")
	for _, line := range seen {
		assert.NotContains(t, line, leakWord)
	}
	assert.NotContains(t, logged, leakWord, "the application log and GORM's trace line")
}

func errText(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}
