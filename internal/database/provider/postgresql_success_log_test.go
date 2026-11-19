package provider

import (
	"encoding/binary"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePostgres accepts connections and speaks just enough of the wire protocol
// for a pgx connect and a ping: it accepts any startup, answers a simple query
// with an empty response, and answers anything else with an error. It exists so
// a test can reach the provider's success path without a database server.
func fakePostgres(t *testing.T) (port int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serveFakePostgresOn(t, listener, false)
	return listener.Addr().(*net.TCPAddr).Port
}

// serveFakePostgresOn serves listener until the test ends. With resetOnQuery
// the server drops the connection at the first extended-protocol message, which
// is the version probe, after startup and the pings have completed.
func serveFakePostgresOn(t *testing.T, listener net.Listener, resetOnQuery bool) {
	t.Helper()
	serveFakePostgresWith(t, listener, resetOnQuery, 0)
}

// serveFakePostgresWith is serveFakePostgresOn with failFrom: where it is above
// zero, the failFrom-th simple query the server receives, counted over every
// connection, and every later one is answered with an error. The pings of a pgx
// pool are simple queries, so 2 lets the first ping pass and fails the next.
func serveFakePostgresWith(t *testing.T, listener net.Listener, resetOnQuery bool, failFrom int32) {
	t.Helper()
	t.Cleanup(func() { _ = listener.Close() })

	var queries atomic.Int32
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go serveFakePostgres(conn, resetOnQuery, failFrom, &queries)
		}
	}()
}

func serveFakePostgres(conn net.Conn, resetOnQuery bool, failFrom int32, queries *atomic.Int32) {
	defer func() { _ = conn.Close() }()

	// startup message: int32 length (self included), then the payload
	var length [4]byte
	if _, err := io.ReadFull(conn, length[:]); err != nil {
		return
	}
	if _, err := io.CopyN(io.Discard, conn, int64(binary.BigEndian.Uint32(length[:]))-4); err != nil {
		return
	}
	message := func(kind byte, body ...byte) []byte {
		out := []byte{kind, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(out[1:], uint32(len(body)+4))
		return append(out, body...)
	}
	ready := message('Z', 'I')
	if _, err := conn.Write(append(message('R', 0, 0, 0, 0), ready...)); err != nil {
		return
	}

	errorSent := false
	for {
		var header [5]byte
		if _, err := io.ReadFull(conn, header[:]); err != nil {
			return
		}
		if _, err := io.CopyN(io.Discard, conn, int64(binary.BigEndian.Uint32(header[1:]))-4); err != nil {
			return
		}
		var reply []byte
		switch header[0] {
		case 'X':
			return
		case 'Q':
			if failFrom > 0 && queries.Add(1) >= failFrom {
				reply = append(message('E', []byte("SERROR\x00C57P01\x00Mdown for the test\x00\x00")...), ready...)
				break
			}
			reply = append(message('I'), ready...)
		case 'S': // Sync ends an extended-protocol batch
			reply = ready
			errorSent = false
		default:
			if resetOnQuery {
				return
			}
			if !errorSent {
				errorSent = true
				reply = message('E', []byte("SERROR\x00C0A000\x00Mnot supported by the test server\x00\x00")...)
			}
		}
		if len(reply) > 0 {
			if _, err := conn.Write(reply); err != nil {
				return
			}
		}
	}
}

// pgx skips a first host it cannot reach (here a socket directory that does not
// exist, so no name is resolved) and connects to the next, but keeps the
// first in its parsed config. The success log must not print that field: a
// password typed into a host list lands there.
func TestPostgreSQLConnect_SuccessLogHoldsNoDSNText(t *testing.T) {
	port := fakePostgres(t)
	logger, readLog := fileLogger(t)
	p := NewPostgreSQLProvider(logger)
	t.Cleanup(func() { _ = p.Close() })

	err := p.Connect(DatabaseConfig{
		Provider: "postgresql",
		DSN: "postgresql://app@127.0.0.1:" + strconv.Itoa(port) +
			"/shelly?host=/" + leakWord + "-missing,127.0.0.1&sslmode=disable&connect_timeout=2",
		LogLevel: "info",
	})

	require.NoError(t, err, "the second host answers")
	logged := readLog()
	assert.Contains(t, logged, "Connected to PostgreSQL database", "the success path ran, or this proves nothing")
	assert.NotContains(t, logged, leakWord)
}

// An error of an open connection is not a connect error: pgx reports a reset on
// a unix socket with the socket path in it, and the version probe of Connect
// used to print it in the application log, GORM's log and the warning.
func TestPostgreSQLConnect_ProbeErrorHoldsNoDSNText(t *testing.T) {
	dir, err := os.MkdirTemp("", "sk")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketDir := filepath.Join(dir, leakWord)
	require.NoError(t, os.Mkdir(socketDir, 0o700))
	socket := filepath.Join(socketDir, ".s.PGSQL.5432")
	require.Less(t, len(socket), 100, "a unix socket path is limited to 108 bytes")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	serveFakePostgresOn(t, listener, true)

	logger, readLog := fileLogger(t)
	p := NewPostgreSQLProvider(logger)
	t.Cleanup(func() { _ = p.Close() })

	err = p.Connect(DatabaseConfig{
		Provider: "postgresql",
		DSN:      "postgresql://app@localhost:5432/shelly?host=" + url.QueryEscape(socketDir) + "&sslmode=disable&connect_timeout=2",
		LogLevel: "info",
	})

	require.NoError(t, err, "a failed version probe is a warning, not a failed connect")
	logged := readLog()
	assert.Contains(t, logged, "Failed to get PostgreSQL version", "the probe failed, or this proves nothing")
	assert.NotContains(t, logged, leakWord)
}
