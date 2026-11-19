package provider

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"strings"
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
//
// The same rule holds after Connect has returned: an error of an open
// connection (a statement the server rejected, a connection that dropped, a
// transaction that could not begin) quotes the address, the socket path, the
// SQL or a stored value just as a connect error does. Those go through
// sealOpenError (sealed_dialector.go), which reports an operation failure
// classified by this same function.
type connectFailure struct {
	kind      string
	detail    string
	operation bool  // an error of an open connection, not of a connect
	cause     error // kept only to answer Is and Timeout, never printed
}

func (f *connectFailure) Error() string {
	prefix := "database connection failed: "
	if f.operation {
		prefix = "database operation failed: "
	}
	if f.detail == "" {
		return prefix + f.kind
	}
	return prefix + f.kind + " (" + f.detail + ")"
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

	// kindStatement and kindUnclassified are the two kinds only an open
	// connection has: the database answered and refused the statement, or the
	// error is of a type nobody listed.
	kindStatement    = "the database rejected the statement"
	kindUnclassified = "unclassified error"

	// kindDSNParameter is a SQLite open the driver refused for one query key of
	// the DSN; the key is the detail, never its value.
	kindDSNParameter = "invalid DSN parameter"
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
	// The ones a statement meets (the same appendix).
	"42P01": "undefined table",
	"42703": "undefined column",
	"40001": "serialization failure",
	"40P01": "deadlock detected",
	"57014": "query canceled",
}

// mysqlErrors names the server error numbers an operator meets most when a
// connection is refused. Any other number is printed as the number alone.
var mysqlErrors = map[uint16]string{
	1045: "access denied",
	1049: "unknown database",
	1040: "too many connections",
	// The ones a statement meets.
	1146: "table does not exist",
	1054: "unknown column",
	1205: "lock wait timeout",
	1213: "deadlock",
}

// classifyConnectError reads the type of err and never its text: the text is
// what carries the configuration. A *pgconn.PgError or *mysql.MySQLError gives
// its code only, because the server echoes the database and the user it was
// sent in its message. The one reader of the text is sealDSNParameterError, which
// uses it to pick a key of the DSN and prints nothing of it.
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
	var coded interface{ Code() int }

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
	case errors.As(err, &coded):
		// The SQLite driver's error: its result code is a number, its text
		// quotes the SQL's identifiers.
		failure.kind = kindServer
		failure.detail = fmt.Sprintf("SQLite error %d", coded.Code())
	}
	return failure
}

// classifyOperationError classifies an error of an open connection by the same
// rule, and words the result for a statement: the server's answer is a refused
// statement, and a type nobody listed is unclassified.
func classifyOperationError(err error) *connectFailure {
	failure := classifyConnectError(err)
	failure.operation = true
	switch failure.kind {
	case kindServer:
		failure.kind = kindStatement
	case kindOther:
		failure.kind = kindUnclassified
	}
	return failure
}

// sealError returns err unchanged when it is already a connectFailure, and its
// classification otherwise. It is for the errors of an operation on an open
// connection that the provider itself reports, such as the ping and the version
// probe of Connect: those do not pass through the connector, and a transport
// error there quotes the socket path or the address.
func sealError(err error) error {
	if err == nil {
		return nil
	}
	var failure *connectFailure
	if errors.As(err, &failure) {
		return err
	}
	return classifyConnectError(err)
}

// sealDSNParameterError seals the error a SQLite open returns. The driver builds
// that error from the DSN query it refuses (`unknown _txlock "<value>"`,
// `conflicting vfs query parameters: [<values>]`), and glebarez/sqlite opens the
// connection inside its dialector, so there is no connector to wrap as for the
// other providers. The failure is classified by sealError. Where it is the
// unclassified kind, the driver text is read once, to find which key of the DSN it
// names, and the key is printed from the DSN: no character of the driver text is.
// Text that names no key of the DSN (a bad escape, a database error) stays generic.
func sealDSNParameterError(dsn string, err error) error {
	sealed := sealError(err)
	var failure *connectFailure
	if !errors.As(sealed, &failure) || failure.kind != kindOther {
		return sealed
	}
	if key := dsnKeyNamedBy(dsn, err.Error()); key != "" {
		return &connectFailure{kind: kindDSNParameter, detail: key, cause: err}
	}
	return sealed
}

// dsnKeyNamedBy returns the key of the DSN query that text names as a whole token,
// the one named first, or "". Only keys of letters, digits and underscores count.
func dsnKeyNamedBy(dsn, text string) string {
	_, query, found := strings.Cut(dsn, "?")
	if !found {
		return ""
	}
	best, bestAt := "", -1
	for _, pair := range strings.Split(query, "&") {
		key, _, _ := strings.Cut(pair, "=")
		if key == "" || strings.Trim(key, wordChars) != "" {
			continue
		}
		if at := wholeTokenIndex(text, key); at >= 0 && (bestAt < 0 || at < bestAt) {
			best, bestAt = key, at
		}
	}
	return best
}

const wordChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"

// wholeTokenIndex returns the index of the first occurrence of token in text that is
// not part of a longer word, or -1.
func wholeTokenIndex(text, token string) int {
	for from := 0; from <= len(text)-len(token); {
		at := strings.Index(text[from:], token)
		if at < 0 {
			return -1
		}
		at += from
		end := at + len(token)
		if (at == 0 || !strings.ContainsRune(wordChars, rune(text[at-1]))) &&
			(end == len(text) || !strings.ContainsRune(wordChars, rune(text[end]))) {
			return at
		}
		from = at + 1
	}
	return -1
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
