package provider

import (
	"encoding/binary"
	"io"
	"net"
	"strconv"
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
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go serveFakePostgres(conn)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

func serveFakePostgres(conn net.Conn) {
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
			reply = append(message('I'), ready...)
		case 'S': // Sync ends an extended-protocol batch
			reply = ready
			errorSent = false
		default:
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

// pgx skips an unresolvable first host and connects to the next, but keeps the
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
			"/shelly?host=" + leakWord + ".invalid,127.0.0.1&sslmode=disable&connect_timeout=2",
		LogLevel: "info",
	})

	require.NoError(t, err, "the second host answers")
	logged := readLog()
	assert.Contains(t, logged, "Connected to PostgreSQL database", "the success path ran, or this proves nothing")
	assert.NotContains(t, logged, leakWord)
}
