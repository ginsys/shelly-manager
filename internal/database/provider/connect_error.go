package provider

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

// connectFailure is the only error a provider's connector returns. The drivers
// build their connect and parse errors from the operator's own configuration
// (the user, the database, the host, a quoted value), in shapes nobody can
// list, and a DSN typed in the wrong place puts a password into every one of
// them. So no driver text is carried: the failure is classified from its type
// and reported as a fixed phrase, with the server's error code where there is
// one. There is deliberately no Unwrap, so the driver's error is unreachable.
type connectFailure struct {
	kind   string
	detail string
	cause  error // kept only to answer Is and Timeout, never printed
}

func (f *connectFailure) Error() string {
	if f.detail == "" {
		return "database connection failed: " + f.kind
	}
	return "database connection failed: " + f.kind + " (" + f.detail + ")"
}

// Timeout lets a caller that tests for a timeout (net.Error) keep working.
func (f *connectFailure) Timeout() bool { return f.kind == kindTimeout }

// Is lets a caller that tests for a context error keep working.
func (f *connectFailure) Is(target error) bool {
	return (target == context.DeadlineExceeded || target == context.Canceled) && errors.Is(f.cause, target)
}

const (
	kindRefused     = "connection refused"
	kindDNS         = "host name did not resolve"
	kindTimeout     = "timed out"
	kindCanceled    = "canceled"
	kindReset       = "connection reset"
	kindUnreachable = "network unreachable"
	kindTLSVerify   = "TLS certificate verification failed"
	kindTLSHandshke = "TLS handshake failed"
	kindServer      = "the server rejected the connection"
	kindOther       = "connection failed"
)

// postgresStates names the SQLSTATE codes an operator meets most when a
// connection is refused (PostgreSQL manual, appendix A). Any other code is
// printed as the code alone.
var postgresStates = map[string]string{
	"28P01": "invalid password",
	"28000": "invalid authorization",
	"3D000": "database does not exist",
	"57P03": "server not accepting connections",
	"53300": "too many connections",
}

// mysqlErrors names the server error numbers an operator meets most when a
// connection is refused. Any other number is printed as the number alone.
var mysqlErrors = map[uint16]string{
	1045: "access denied",
	1049: "unknown database",
	1040: "too many connections",
}

// classifyConnectError reads the type of err and never its text: the text is
// what carries the configuration. A *pgconn.PgError or *mysql.MySQLError gives
// its code only, because the server echoes the database and the user it was
// sent in its message.
func classifyConnectError(err error) *connectFailure {
	failure := &connectFailure{kind: kindOther, cause: err}

	var pgErr *pgconn.PgError
	var myErr *mysql.MySQLError
	var dnsErr *net.DNSError
	var netErr net.Error
	var verifyErr *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var invalidCert x509.CertificateInvalidError
	var alertErr tls.AlertError
	var recordErr tls.RecordHeaderError

	switch {
	case errors.As(err, &pgErr):
		failure.kind = kindServer
		if name, ok := postgresStates[pgErr.Code]; ok {
			failure.detail = fmt.Sprintf("SQLSTATE %s, %s", pgErr.Code, name)
		} else {
			failure.detail = "SQLSTATE " + pgErr.Code
		}
	case errors.As(err, &myErr):
		failure.kind = kindServer
		if name, ok := mysqlErrors[myErr.Number]; ok {
			failure.detail = fmt.Sprintf("MySQL error %d, %s", myErr.Number, name)
		} else {
			failure.detail = fmt.Sprintf("MySQL error %d", myErr.Number)
		}
	case errors.Is(err, context.Canceled):
		failure.kind = kindCanceled
	case errors.Is(err, context.DeadlineExceeded), pgconn.Timeout(err),
		errors.As(err, &netErr) && netErr.Timeout(), errors.Is(err, syscall.ETIMEDOUT):
		failure.kind = kindTimeout
	case errors.As(err, &dnsErr):
		failure.kind = kindDNS
	case errors.Is(err, syscall.ECONNREFUSED):
		failure.kind = kindRefused
	case errors.Is(err, syscall.ECONNRESET), errors.Is(err, syscall.EPIPE):
		failure.kind = kindReset
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		failure.kind = kindUnreachable
	case errors.As(err, &verifyErr), errors.As(err, &unknownAuthority),
		errors.As(err, &hostnameErr), errors.As(err, &invalidCert):
		failure.kind = kindTLSVerify
	case errors.As(err, &alertErr), errors.As(err, &recordErr):
		failure.kind = kindTLSHandshke
	}
	return failure
}

// sealedConnector wraps a driver.Connector so that every connection attempt of
// the pool (the first, each growth of the pool, each reconnect after a dropped
// connection) fails with a connectFailure and never with the driver's error.
type sealedConnector struct {
	inner driver.Connector
}

// sealConnector returns inner with its connect errors sealed.
func sealConnector(inner driver.Connector) driver.Connector {
	return sealedConnector{inner: inner}
}

func (c sealedConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		// driver.ErrBadConn is a sentinel with fixed text that database/sql
		// reads to retry; it carries no configuration.
		if errors.Is(err, driver.ErrBadConn) {
			return nil, driver.ErrBadConn
		}
		return nil, classifyConnectError(err)
	}
	return conn, nil
}

func (c sealedConnector) Driver() driver.Driver { return c.inner.Driver() }

// errDSNNotParsed is what a DSN a driver's own parser refused reports. The
// parsers quote part of what they were given, so nothing of the error is
// passed on.
var errDSNNotParsed = errors.New("the DSN could not be parsed by the database driver")
