package provider

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mysqlReproDSNs hold operator text the MySQL driver quotes in its own errors:
// its DSN parser names the value it refused (go-sql-driver dsn.go), and its dial
// error names the address it tried.
var mysqlReproDSNs = map[string]string{
	"an unknown tls name":                          "app:pw@tcp(127.0.0.1:1)/shelly?tls=LEAKWORD7",
	"a bad boolean value":                          "app:pw@tcp(127.0.0.1:1)/shelly?parseTime=LEAKWORD7",
	"a bad bool, second place":                     "app:pw@tcp(127.0.0.1:1)/shelly?tls=false&interpolateParams=LEAKWORD7",
	"a refused port with the password in the user": "LEAKWORD7:LEAKWORD7@tcp(127.0.0.1:1)/shelly?timeout=1s",
	"an address that never resolves":               "app:pw@tcp(LEAKWORD7.invalid:3306)/shelly?timeout=1s",
}

// recordingLogger stands for the MySQL driver's default logger, which writes to
// stderr before any connector sees the error.
type recordingLogger struct {
	mu   sync.Mutex
	text strings.Builder
}

func (r *recordingLogger) Print(v ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprint(&r.text, v...)
}

func (r *recordingLogger) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.text.String()
}

// A server that accepts and never sends its handshake makes the driver log the
// socket read timeout, with the socket path in it, through its package logger.
func TestMySQLConnectError_DriverLoggerHoldsNoDSNText(t *testing.T) {
	dir, err := os.MkdirTemp("", "sk")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, leakWord+".sock")
	require.Less(t, len(socket), 100, "a unix socket path is limited to 108 bytes")

	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			defer func() { _ = conn.Close() }()
		}
	}()

	recorded := &recordingLogger{}
	require.NoError(t, mysqldriver.SetLogger(recorded))
	t.Cleanup(func() {
		_ = mysqldriver.SetLogger(log.New(os.Stderr, "[mysql] ", log.Ldate|log.Ltime))
	})

	logger, readLog := fileLogger(t)
	p := NewMySQLProvider(logger)
	err = p.Connect(DatabaseConfig{
		Provider: "mysql",
		DSN:      "app:pw@unix(" + socket + ")/shelly?tls=false&timeout=1s&readTimeout=100ms",
		LogLevel: "info",
	})

	require.Error(t, err)
	assert.NotContains(t, err.Error(), leakWord)
	assert.NotContains(t, readLog(), leakWord)
	assert.NotContains(t, recorded.String(), leakWord, "the driver's package logger")
}

func TestMySQLConnectError_HoldsNoDSNText(t *testing.T) {
	for name, dsn := range mysqlReproDSNs {
		t.Run(name, func(t *testing.T) {
			logger, readLog := fileLogger(t)
			p := NewMySQLProvider(logger)

			err := p.Connect(DatabaseConfig{Provider: "mysql", DSN: dsn, LogLevel: "info"})

			require.Error(t, err, "nothing listens on these addresses")
			assert.NotContains(t, err.Error(), leakWord, "the returned error")
			assert.NotContains(t, fmt.Sprintf("%+v", err), leakWord, "the error with %+v")
			assert.NotContains(t, readLog(), leakWord, "the captured log")
		})
	}
}
