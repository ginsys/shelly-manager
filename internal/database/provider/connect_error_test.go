package provider

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every case wraps leakWord the way a driver does: inside its own text.
func TestClassifyConnectError(t *testing.T) {
	leak := func(format string, args ...any) error { return fmt.Errorf(format, args...) }

	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "a server error is its code, never its message",
			err:  &pgconn.PgError{Severity: "FATAL", Code: "28P01", Message: `password authentication failed for user "` + leakWord + `"`},
			want: "database connection failed: the server rejected the connection (SQLSTATE 28P01, invalid password)",
		},
		{
			name: "a server error with no name for its code",
			err:  &pgconn.PgError{Code: "XX000", Message: "database " + leakWord + " is broken"},
			want: "database connection failed: the server rejected the connection (SQLSTATE XX000)",
		},
		{
			name: "a MySQL server error is its number, never its message",
			err:  &mysql.MySQLError{Number: 1045, Message: "Access denied for user '" + leakWord + "'@'host'"},
			want: "database connection failed: the server rejected the connection (MySQL error 1045, access denied)",
		},
		{
			name: "a MySQL error with no name for its number",
			err:  &mysql.MySQLError{Number: 1146, Message: "Table '" + leakWord + "' doesn't exist"},
			want: "database connection failed: the server rejected the connection (MySQL error 1146)",
		},
		{
			name: "a DNS error that names the host",
			err:  leak("failed to connect to `host=%s user=u database=d`: %w", leakWord, &net.DNSError{Err: "no such host", Name: leakWord + ".invalid", IsNotFound: true}),
			want: "database connection failed: host name did not resolve",
		},
		{
			name: "a refused connection",
			err:  leak("dial tcp %s:5432: %w", leakWord, &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED}),
			want: "database connection failed: connection refused",
		},
		{
			name: "a deadline",
			err:  leak("failed to connect to `user=%s`: %w", leakWord, context.DeadlineExceeded),
			want: "database connection failed: timed out",
		},
		{
			name: "a cancellation",
			err:  leak("dial %s: %w", leakWord, context.Canceled),
			want: "database connection failed: canceled",
		},
		{
			name: "a reset",
			err:  leak("read %s: %w", leakWord, syscall.ECONNRESET),
			want: "database connection failed: connection reset",
		},
		{
			name: "an unreachable network",
			err:  leak("dial %s: %w", leakWord, syscall.ENETUNREACH),
			want: "database connection failed: network unreachable",
		},
		{
			name: "a certificate the client does not trust",
			err:  leak("failed to connect to %s: %w", leakWord, &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}),
			want: "database connection failed: TLS certificate verification failed",
		},
		{
			name: "a certificate for another host name",
			err:  leak("failed to connect to %s: %w", leakWord, x509.HostnameError{Host: leakWord, Certificate: &x509.Certificate{}}),
			want: "database connection failed: TLS certificate verification failed",
		},
		{
			name: "a TLS alert",
			err:  leak("failed to connect to %s: %w", leakWord, tls.AlertError(40)),
			want: "database connection failed: TLS handshake failed",
		},
		{
			name: "an error of no known type",
			err:  errors.New("something about " + leakWord),
			want: "database connection failed: connection failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyConnectError(tc.err)

			assert.Equal(t, tc.want, got.Error())
			assert.NotContains(t, got.Error(), leakWord)
			assert.NotContains(t, fmt.Sprintf("%+v %#v %s", got, got, got), leakWord, "printing the failure in any verb")
			assert.Nil(t, errors.Unwrap(got), "the driver's error stays unreachable")
		})
	}
}

func TestConnectFailure_KeepsTheContextAndTimeoutTests(t *testing.T) {
	deadline := classifyConnectError(fmt.Errorf("dial: %w", context.DeadlineExceeded))
	canceled := classifyConnectError(fmt.Errorf("dial: %w", context.Canceled))
	refused := classifyConnectError(syscall.ECONNREFUSED)

	assert.ErrorIs(t, deadline, context.DeadlineExceeded)
	assert.NotErrorIs(t, deadline, context.Canceled)
	assert.ErrorIs(t, canceled, context.Canceled)
	assert.NotErrorIs(t, refused, context.DeadlineExceeded)

	// the shape callers test for a timeout (as pgconn.Timeout does)
	type timeouter interface{ Timeout() bool }
	var timeout timeouter
	require.ErrorAs(t, deadline, &timeout)
	assert.True(t, timeout.Timeout())
	require.ErrorAs(t, refused, &timeout)
	assert.False(t, timeout.Timeout())
}

// failingConnector stands for a driver whose connect error quotes the DSN.
type failingConnector struct{ err error }

func (c failingConnector) Connect(context.Context) (driver.Conn, error) { return nil, c.err }
func (c failingConnector) Driver() driver.Driver                        { return nil }

func TestSealedConnector_FailsWithoutDriverText(t *testing.T) {
	inner := failingConnector{err: fmt.Errorf("dial tcp %s:5432: %w", leakWord, syscall.ECONNREFUSED)}
	db := sql.OpenDB(sealConnector(inner))
	t.Cleanup(func() { _ = db.Close() })

	// Every attempt of the pool goes through the connector: the first, a pool
	// growth and a reconnect all read the same sealed error.
	for attempt := 1; attempt <= 3; attempt++ {
		err := db.Ping()

		require.Error(t, err, "attempt %d", attempt)
		assert.NotContains(t, err.Error(), leakWord, "attempt %d", attempt)
		assert.NotContains(t, fmt.Sprintf("%+v", err), leakWord, "attempt %d", attempt)
		assert.Contains(t, err.Error(), "connection refused")
	}
}

func TestSealedConnector_KeepsTheRetrySentinel(t *testing.T) {
	sealed := sealConnector(failingConnector{err: fmt.Errorf("wrapped %s: %w", leakWord, driver.ErrBadConn)})

	_, err := sealed.Connect(context.Background())

	assert.Equal(t, driver.ErrBadConn, err, "database/sql reads this sentinel to retry")
}
