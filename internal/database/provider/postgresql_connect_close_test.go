package provider

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// Connect opens a pool, and closes it again on every branch that fails after the
// open (ginsys/shelly-manager#602). The observer is in pool_opener_test.go.
//
// The third branch, a failed configureConnectionPool, has no test: it fails only
// when the GORM handle cannot return its *sql.DB, and Connect always hands GORM one,
// so no input reaches it. Reaching it would take a seam in production code.

func postgresDSN(host string, port int) string {
	return "postgresql://app@" + net.JoinHostPort(host, strconv.Itoa(port)) +
		"/shelly?sslmode=disable&connect_timeout=2"
}

// GORM pings the database inside Open, so a refused port fails openGORM. GORM closes
// the pool itself when that ping fails (gorm.go, Open), so this test guards the
// outcome and not the sqlDB.Close line of Connect: removing that line leaves it
// green. The Ping test below is the one that fails without its Close.
func TestPostgreSQLConnect_ClosesThePoolWhenOpenFails(t *testing.T) {
	host, port := refusedAddress(t)
	logger, _ := fileLogger(t)
	p := NewPostgreSQLProvider(logger)
	before := openerGoroutineIDs(t)

	err := p.Connect(DatabaseConfig{Provider: "postgresql", DSN: postgresDSN(host, port), LogLevel: "info"})

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to connect to PostgreSQL database", "the failing branch is openGORM")
	requirePoolClosed(t, before)
}

// The first ping, GORM's own, passes and the provider's second one fails, with
// every retry of database/sql failing too.
func TestPostgreSQLConnect_ClosesThePoolWhenPingFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serveFakePostgresWith(t, listener, false, 2)
	logger, _ := fileLogger(t)
	p := NewPostgreSQLProvider(logger)
	before := openerGoroutineIDs(t)

	err = p.Connect(DatabaseConfig{
		Provider: "postgresql",
		DSN:      postgresDSN("127.0.0.1", listener.Addr().(*net.TCPAddr).Port),
		LogLevel: "info",
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to ping database", "the failing branch is Ping, after GORM's own ping passed")
	requirePoolClosed(t, before)
}
