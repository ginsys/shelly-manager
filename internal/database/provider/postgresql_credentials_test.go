package provider

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// trickyPassword holds every character that breaks a DSN pasted together by
// hand: userinfo delimiters, a percent sign, and the two sequences
// validateDSNInput refuses.
const trickyPassword = "p@ss:w/rd%41;--x=y s3cret"

// passwordForms lists every spelling of a password that must never reach an
// error text, a log line or a printed config.
func passwordForms(password string) []string {
	return []string{
		password,
		url.QueryEscape(password),
		url.PathEscape(password),
		strings.TrimPrefix(url.UserPassword("", password).String(), ":"),
	}
}

func assertNoPassword(t *testing.T, text, password string) {
	t.Helper()
	for _, form := range passwordForms(password) {
		assert.NotContains(t, text, form, "text must not hold the password")
	}
}

func newCredentialProvider(cfg DatabaseConfig) *PostgreSQLProvider {
	p := NewPostgreSQLProvider(logging.GetDefault())
	p.config = cfg
	return p
}

// fileLogger returns a logger that writes everything to a file and a function
// that reads the file back.
func fileLogger(t *testing.T) (*logging.Logger, func() string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "provider.log")
	logger, err := logging.New(logging.Config{Level: "debug", Format: "json", Output: path})
	require.NoError(t, err)
	return logger, func() string {
		b, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(b)
	}
}

func TestPostgreSQLCredentials_BuildDSN(t *testing.T) {
	t.Run("username and password go into the URL and decode back", func(t *testing.T) {
		p := newCredentialProvider(DatabaseConfig{Username: "shelly app", Password: trickyPassword})

		dsn, err := p.buildDSN("postgresql://db.example:5432/shelly", nil)
		require.NoError(t, err)

		u, err := url.Parse(dsn)
		require.NoError(t, err)
		require.NotNil(t, u.User)
		assert.Equal(t, "shelly app", u.User.Username())
		got, ok := u.User.Password()
		assert.True(t, ok)
		assert.Equal(t, trickyPassword, got)
		assert.Equal(t, "db.example:5432", u.Host)
		assert.Equal(t, "/shelly", u.Path)
		assert.Equal(t, "require", u.Query().Get("sslmode"), "the existing defaults still apply")
	})

	t.Run("a username alone is allowed", func(t *testing.T) {
		p := newCredentialProvider(DatabaseConfig{Username: "shelly"})

		dsn, err := p.buildDSN("postgresql://db.example/shelly", nil)
		require.NoError(t, err)
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		assert.Equal(t, "shelly", u.User.Username())
		_, hasPassword := u.User.Password()
		assert.False(t, hasPassword)
	})

	t.Run("both fields empty leaves the DSN exactly as before", func(t *testing.T) {
		p := newCredentialProvider(DatabaseConfig{})
		const base = "postgresql://user:pw@db.example:5432/shelly?sslmode=disable"

		dsn, err := p.buildDSN(base, nil)
		require.NoError(t, err)
		assert.Equal(t, base+"&connect_timeout=10", dsn)
	})

	t.Run("the keys a DSN may keep next to the two fields are accepted", func(t *testing.T) {
		p := newCredentialProvider(DatabaseConfig{Username: "shelly", Password: trickyPassword})

		dsn, err := p.buildDSN("postgresql://db.example/shelly?sslmode=disable&application_name=x&connect_timeout=5", nil)

		require.NoError(t, err)
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		assert.Equal(t, "disable", u.Query().Get("sslmode"))
		assert.Equal(t, "x", u.Query().Get("application_name"))
	})

	// database.options.sslmode and database.options.connect_timeout are appended to the
	// DSN after the query check ran, so a value holding "&" must stay one value.
	for name, tc := range map[string]struct {
		options map[string]string
		key     string
		want    string
		bad     string
	}{
		"sslmode":         {map[string]string{"sslmode": "require&password=x"}, "sslmode", "require&password=x", "password"},
		"connect_timeout": {map[string]string{"connect_timeout": "5&user=x"}, "connect_timeout", "5&user=x", "user"},
	} {
		t.Run("an option "+name+" holding a query separator cannot add a credential key", func(t *testing.T) {
			p := newCredentialProvider(DatabaseConfig{Username: "shelly", Password: trickyPassword})

			dsn, err := p.buildDSN("postgresql://db.example/shelly", tc.options)

			require.NoError(t, err)
			u, err := url.Parse(dsn)
			require.NoError(t, err)
			assert.False(t, u.Query().Has(tc.bad), "the value must not become a %s key", tc.bad)
			assert.Equal(t, tc.want, u.Query().Get(tc.key), "the value stays whole")
			got, _ := u.User.Password()
			assert.Equal(t, trickyPassword, got)
		})
	}

	// pgx reads user, password, passfile and service from the query and lets them
	// win over the userinfo, and its key parsing differs from net/url's (padding,
	// encoding, a '#'). So only a list of literal keys passes; every other shape
	// below is refused without printing any of it.
	for name, dsn := range map[string]string{
		"userinfo":                   "postgresql://inline_user:inline_pw@db.example/shelly",
		"userinfo user only":         "postgresql://inline_user@db.example/shelly",
		"user parameter":             "postgresql://db.example/shelly?user=inline_user",
		"password parameter":         "postgresql://db.example/shelly?sslmode=disable&password=inline_pw",
		"encoded password parameter": "postgresql://db.example/shelly?pass%77ord=inline_pw",
		"encoded user parameter":     "postgresql://db.example/shelly?us%65r=inline_user",
		"padded password parameter":  "postgresql://db.example/shelly? password =inline_pw",
		"password after a hash":      "postgresql://db.example/shelly?application_name=x#&password=inline_pw",
		"sslpassword parameter":      "postgresql://db.example/shelly?sslpassword=inline_pw",
		"passfile parameter":         "postgresql://db.example/shelly?passfile=/inline_pw",
		"service parameter":          "postgresql://db.example/shelly?service=inline_pw",
		"a key with no value":        "postgresql://db.example/shelly?inline_pw",
	} {
		t.Run("a DSN with "+name+" and the two fields is refused without printing either", func(t *testing.T) {
			p := newCredentialProvider(DatabaseConfig{Username: "shelly", Password: trickyPassword})

			_, err := p.buildDSN(dsn, nil)

			require.Error(t, err)
			for _, secret := range []string{"inline_user", "inline_pw", "shelly:"} {
				assert.NotContains(t, err.Error(), secret)
			}
			assertNoPassword(t, err.Error(), trickyPassword)
			assert.Contains(t, err.Error(), "database.username")
		})
	}

	// "postgresql://127.0.0.1:1/inline_pw@127.0.0.1/shelly" parses with a numeric
	// port and "inline_pw@127.0.0.1/shelly" as the database name, which the driver
	// prints in its connect error. A database name is one segment without '@'.
	for name, cfg := range map[string]DatabaseConfig{
		"both keys unset": {},
		"both keys set":   {Username: "shelly", Password: trickyPassword},
	} {
		for dsnName, dsn := range map[string]string{
			"numeric prefix":  "postgresql://127.0.0.1:1/inline_pw@127.0.0.1/shelly",
			"two segments":    "postgresql://db.example/shelly/inline_pw",
			"blank in a name": "postgresql://db.example/shelly inline_pw",
		} {
			t.Run("a DSN with a "+dsnName+" is refused, "+name, func(t *testing.T) {
				p := newCredentialProvider(cfg)

				_, err := p.buildDSN(dsn, nil)

				require.Error(t, err)
				assert.NotContains(t, err.Error(), "inline_pw")
				assert.Contains(t, err.Error(), "database name")
			})
		}
	}

	t.Run("a password without a username is refused", func(t *testing.T) {
		p := newCredentialProvider(DatabaseConfig{Password: trickyPassword})

		_, err := p.buildDSN("postgresql://db.example/shelly", nil)
		require.Error(t, err)
		assertNoPassword(t, err.Error(), trickyPassword)
		assert.Contains(t, err.Error(), "database.username")
	})
}

func TestPostgreSQLCredentials_ErrorsAndLogsHoldNoPassword(t *testing.T) {
	t.Run("a refused connection, error and log", func(t *testing.T) {
		// Port 1 on the loopback address is closed: the connect fails at once and
		// no server is needed. The driver's own error omits the password, so this
		// is the regression guard for the whole path, and the log is read too.
		logger, readLog := fileLogger(t)
		p := NewPostgreSQLProvider(logger)

		err := p.Connect(DatabaseConfig{
			Provider: "postgresql",
			DSN:      "postgresql://127.0.0.1:1/shelly",
			Username: "shelly",
			Password: trickyPassword,
			LogLevel: "info",
			Options:  map[string]string{"sslmode": "disable", "connect_timeout": "1"},
		})

		require.Error(t, err)
		assertNoPassword(t, err.Error(), trickyPassword)
		logged := readLog()
		assert.NotEmpty(t, logged, "the log must hold something, or this assertion proves nothing")
		assertNoPassword(t, logged, trickyPassword)
	})

	// Here the error comes from this package's own DSN check, which must never
	// quote the DSN it refused.
	for name, dsn := range map[string]string{
		"userinfo":                "postgresql://inline_user:inline_pw@db.example:badport/shelly",
		"password parameter":      "postgresql://db.example:badport/shelly?password=inline_pw",
		"encoded key":             "postgresql://db.example:badport/shelly?pass%77ord=inline_pw",
		"padded key":              "postgresql://db.example:badport/shelly? password =inline_pw",
		"a slash in the password": "postgresql://app:inline_pw/tail@db.example/shelly",
		"numeric prefix":          "postgresql://127.0.0.1:1/inline_pw@127.0.0.1/shelly",
		"a newline in userinfo":   "postgresql://app:inline_pw\ntail@db.example/shelly",
		"password after a hash":   "postgresql://db.example:badport/shelly?application_name=x#&password=inline_pw",
	} {
		t.Run("a DSN that cannot be built, with "+name, func(t *testing.T) {
			p := NewPostgreSQLProvider(logging.GetDefault())

			err := p.Connect(DatabaseConfig{Provider: "postgresql", DSN: dsn})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to build DSN")
			assert.NotContains(t, err.Error(), "inline_pw")
		})
	}
}

func TestMySQLProvider_RefusesSeparateCredentials(t *testing.T) {
	for name, cfg := range map[string]DatabaseConfig{
		"username": {Username: "shelly"},
		"password": {Password: trickyPassword},
		"both":     {Username: "shelly", Password: trickyPassword},
	} {
		t.Run(name, func(t *testing.T) {
			cfg.Provider = "mysql"
			cfg.DSN = "user:pw@tcp(127.0.0.1:1)/shelly"

			err := NewMySQLProvider(logging.GetDefault()).Connect(cfg)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "not supported by the MySQL provider")
			assertNoPassword(t, err.Error(), trickyPassword)
		})
	}
}

func TestDatabaseConfig_StringMasksSecrets(t *testing.T) {
	for name, dsn := range map[string]string{
		"userinfo":                   "postgresql://inline_user:inline_pw@db.example/shelly",
		"password parameter":         "postgresql://db.example/shelly?user=app&password=inline_pw",
		"encoded password parameter": "postgresql://db.example/shelly?pass%77ord=inline_pw&sslmode=disable",
		"ssl password parameter":     "postgresql://db.example/shelly?sslpassword=inline_pw",
		"userinfo with a slash":      "postgresql://app:inline_pw/tail@db.example/shelly",
		"padded password parameter":  "postgresql://db.example/shelly? password =inline_pw",
		"padded sslpassword":         "postgresql://db.example/shelly? sslpassword =inline_pw",
		"password after a hash":      "postgresql://db.example/shelly?application_name=x#&password=inline_pw",
		"query password holding @":   "postgresql://db.example/shelly?password=prefix@inline_pw",
		"newline in the userinfo":    "postgresql://app:inline_pw\ntail@db.example/shelly",
		"numeric prefix":             "postgresql://127.0.0.1:1/inline_pw@127.0.0.1/shelly",
		"mysql form":                 "app:inline_pw@tcp(db.example:3306)/shelly",
	} {
		for _, provider := range []string{"postgresql", "mysql", ""} {
			t.Run(name+" as "+provider, func(t *testing.T) {
				cfg := DatabaseConfig{Provider: provider, DSN: dsn, Username: "shelly", Password: trickyPassword}

				for how, got := range map[string]string{
					"%v value":    fmt.Sprintf("%v", cfg),
					"%+v value":   fmt.Sprintf("%+v", cfg),
					"%v pointer":  fmt.Sprintf("%v", &cfg),
					"%+v pointer": fmt.Sprintf("%+v", &cfg),
					"String()":    cfg.String(),
				} {
					assertNoPassword(t, got, trickyPassword)
					assert.NotContains(t, got, "inline_pw", how)
					assert.NotContains(t, got, "db.example", "no part of a network DSN is printed: "+how)
				}
			})
		}
	}

	t.Run("a SQLite DSN is a path and prints as it is", func(t *testing.T) {
		cfg := DatabaseConfig{Provider: "sqlite", DSN: "data/shelly.db"}

		assert.Contains(t, cfg.String(), "data/shelly.db")
		assert.Equal(t, "data/shelly.db", cfg.Target())
	})

	t.Run("Target names the provider for a network DSN and the path for SQLite", func(t *testing.T) {
		for name, tc := range map[string]struct{ cfg, want string }{
			"postgresql":          {"postgresql", "postgresql (DSN redacted)"},
			"mysql with blanks":   {" MySQL ", "mysql (DSN redacted)"},
			"sqlite3":             {"sqlite3", "data/shelly.db"},
			"an unknown provider": {"other", "other (DSN redacted)"},
			"no provider":         {"", " (DSN redacted)"},
		} {
			t.Run(name, func(t *testing.T) {
				cfg := DatabaseConfig{Provider: tc.cfg, DSN: "data/shelly.db?_pragma_key=" + leakWord}

				assert.Equal(t, tc.want, cfg.Target())
				assert.NotContains(t, cfg.Target(), leakWord)
			})
		}
	})
}
