package provider

import (
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// The MySQL counterpart of postgresql_connect_close_test.go: Connect closes the pool
// it opened on every branch that fails after the open (ginsys/shelly-manager#602).
// A failed configureConnectionPool has no test, for the reason given there.

func mysqlDSN(host string, port int) string {
	return "app@tcp(" + net.JoinHostPort(host, strconv.Itoa(port)) + ")/shelly?tls=false&timeout=2s&readTimeout=2s"
}

// The dialector reads the server version when it initialises, so a refused port fails
// openGORM. GORM closes the pool itself when its initialisation fails (gorm.go, Open),
// so this test guards the outcome and not the sqlDB.Close line of Connect: removing
// that line leaves it green. The Ping test below is the one that fails without its Close.
func TestMySQLConnect_ClosesThePoolWhenOpenFails(t *testing.T) {
	host, port := refusedAddress(t)
	logger, _ := fileLogger(t)
	m := NewMySQLProvider(logger)
	before := openerGoroutineIDs(t)

	err := m.Connect(DatabaseConfig{Provider: "mysql", DSN: mysqlDSN(host, port), LogLevel: "info"})

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to connect to MySQL database", "the failing branch is openGORM")
	requirePoolClosed(t, before)
}

// The version query and GORM's own ping pass, and the provider's second ping fails.
func TestMySQLConnect_ClosesThePoolWhenPingFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serveFakeMySQL(t, listener, 2)
	logger, _ := fileLogger(t)
	m := NewMySQLProvider(logger)
	before := openerGoroutineIDs(t)

	err = m.Connect(DatabaseConfig{
		Provider: "mysql",
		DSN:      mysqlDSN("127.0.0.1", listener.Addr().(*net.TCPAddr).Port),
		LogLevel: "info",
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to ping database", "the failing branch is Ping, after the version query and GORM's ping passed")
	requirePoolClosed(t, before)
}

// serveFakeMySQL speaks just enough of the MySQL protocol for a connect, the
// version query and pings: any login is accepted, SELECT VERSION() is answered
// with a one-row result, any other query with OK, and the pingFailFrom-th ping,
// counted over every connection, and every later one with an error.
func serveFakeMySQL(t *testing.T, listener net.Listener, pingFailFrom int32) {
	t.Helper()
	t.Cleanup(func() { _ = listener.Close() })

	var pings atomic.Int32
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go serveFakeMySQLConn(conn, pingFailFrom, &pings)
		}
	}()
}

func serveFakeMySQLConn(conn net.Conn, pingFailFrom int32, pings *atomic.Int32) {
	defer func() { _ = conn.Close() }()

	write := func(seq byte, payload ...byte) bool {
		out := []byte{byte(len(payload)), byte(len(payload) >> 8), byte(len(payload) >> 16), seq}
		_, err := conn.Write(append(out, payload...))
		return err == nil
	}
	read := func() (seq byte, payload []byte, ok bool) {
		var header [4]byte
		if _, err := io.ReadFull(conn, header[:]); err != nil {
			return 0, nil, false
		}
		payload = make([]byte, int(binary.LittleEndian.Uint32(append(header[:3:3], 0))))
		if _, err := io.ReadFull(conn, payload); err != nil {
			return 0, nil, false
		}
		return header[3], payload, true
	}
	cstr := func(s string) []byte { return append([]byte(s), 0) }
	lenenc := func(s string) []byte { return append([]byte{byte(len(s))}, s...) }
	ok := []byte{0x00, 0, 0, 0x02, 0x00, 0, 0}
	eof := []byte{0xfe, 0, 0, 0x02, 0x00}
	failure := append([]byte{0xff, 0x51, 0x04, '#'}, "HY000down for the test"...)

	// Initial handshake, protocol 10, offering protocol 4.1, secure connection and
	// plugin authentication, the three flags the driver requires.
	greeting := []byte{10}
	greeting = append(greeting, cstr("8.0.0-fake")...)
	greeting = append(greeting, 1, 0, 0, 0)                       // connection id
	greeting = append(greeting, "abcdefgh"...)                    // auth data, part 1
	greeting = append(greeting, 0)                                // filler
	greeting = append(greeting, 0x0d, 0xa2)                       // capabilities, low: long password, long flag, connect with db, protocol 4.1, secure connection
	greeting = append(greeting, 0x21, 0x02, 0x00)                 // charset, status
	greeting = append(greeting, 0x08, 0x00)                       // capabilities, high: plugin auth
	greeting = append(greeting, 21)                               // auth data length
	greeting = append(greeting, make([]byte, 10)...)              // reserved
	greeting = append(greeting, cstr("ijklmnopqrst")...)          // auth data, part 2
	greeting = append(greeting, cstr("mysql_native_password")...) // plugin
	if !write(0, greeting...) {
		return
	}
	if _, _, got := read(); !got { // the client's login
		return
	}
	if !write(2, ok...) {
		return
	}

	for {
		_, command, got := read()
		if !got || len(command) == 0 {
			return
		}
		switch command[0] {
		case 0x01: // COM_QUIT
			return
		case 0x0e: // COM_PING
			if pingFailFrom > 0 && pings.Add(1) >= pingFailFrom {
				if !write(1, failure...) {
					return
				}
				continue
			}
			if !write(1, ok...) {
				return
			}
		case 0x03: // COM_QUERY
			if string(command[1:]) != "SELECT VERSION()" {
				if !write(1, ok...) {
					return
				}
				continue
			}
			column := []byte{}
			for _, s := range []string{"def", "", "", "", "VERSION()", ""} {
				column = append(column, lenenc(s)...)
			}
			column = append(column, 0x0c, 0x21, 0x00, 0x20, 0, 0, 0, 0xfd, 0, 0, 0, 0, 0)
			if write(1, 1) && write(2, column...) && write(3, eof...) && write(4, lenenc("8.0.0-fake")...) && write(5, eof...) {
				continue
			}
			return
		default: // COM_RESET_CONNECTION and anything else
			if !write(1, ok...) {
				return
			}
		}
	}
}
