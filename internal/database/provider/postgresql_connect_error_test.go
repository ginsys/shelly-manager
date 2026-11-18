package provider

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// leakWord stands for the password an operator typed in the wrong place. It is
// never a value any provider was given as a password, so only a provider that
// carries driver text can print it.
const leakWord = "LEAKWORD7"

// postgresReproDSNs are the malformed DSNs of ginsys/shelly-manager#587: net/url
// and pgx read each one differently, and the driver prints what it parsed.
var postgresReproDSNs = map[string]string{
	// net/url reads /LEAKWORD7 as the path, pgx reads the fragment as part of the
	// database name and prints it.
	"a fragment in the database name": "postgresql://127.0.0.1:1/LEAKWORD7#@127.0.0.1/shelly?sslmode=disable&connect_timeout=1",
	// net/url splits the userinfo at the last "@", pgx at the first, so the
	// password suffix becomes the host in pgx's DNS error.
	"an unescaped @ in the password": "postgresql://app:prefix@LEAKWORD7@127.0.0.1:1/shelly?sslmode=disable&connect_timeout=1",
	// the numeric prefix is read as a port (refused by buildDSN, kept for the pattern)
	"a numeric prefix": "postgresql://127.0.0.1:1/LEAKWORD7@127.0.0.1/shelly",
	// the same two shapes with a name that never resolves, so the error is a DNS one
	"an unescaped @ with a host that does not resolve": "postgresql://app:prefix@LEAKWORD7@db.invalid:5432/shelly?sslmode=disable&connect_timeout=1",
	"the password as the database name":                "postgresql://app@127.0.0.1:1/LEAKWORD7?sslmode=disable&connect_timeout=1",
	"the password as the user":                         "postgresql://LEAKWORD7@127.0.0.1:1/shelly?sslmode=disable&connect_timeout=1",
}

func TestPostgreSQLConnectError_HoldsNoDSNText(t *testing.T) {
	for name, dsn := range postgresReproDSNs {
		t.Run(name, func(t *testing.T) {
			logger, readLog := fileLogger(t)
			p := NewPostgreSQLProvider(logger)

			err := p.Connect(DatabaseConfig{Provider: "postgresql", DSN: dsn, LogLevel: "info"})

			require.Error(t, err, "nothing listens on these addresses")
			assert.NotContains(t, err.Error(), leakWord, "the returned error")
			assert.NotContains(t, fmt.Sprintf("%+v", err), leakWord, "the error with %+v")
			assert.NotContains(t, readLog(), leakWord, "the captured log")
		})
	}
}

func TestPostgreSQLConnectError_PasswordFieldHoldsNoText(t *testing.T) {
	logger, readLog := fileLogger(t)
	p := NewPostgreSQLProvider(logger)

	err := p.Connect(DatabaseConfig{
		Provider: "postgresql",
		DSN:      "postgresql://127.0.0.1:1/shelly",
		Username: "shelly",
		Password: leakWord,
		LogLevel: "info",
		Options:  map[string]string{"sslmode": "disable", "connect_timeout": "1"},
	})

	require.Error(t, err)
	assert.NotContains(t, err.Error(), leakWord)
	logged := readLog()
	assert.NotEmpty(t, logged, "the log must hold something, or this assertion proves nothing")
	assert.NotContains(t, logged, leakWord)
}
