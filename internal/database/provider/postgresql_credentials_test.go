package provider

import (
	"fmt"
	"net/url"
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
		url.UserPassword("u", password).String(),
	}
}

func assertNoPassword(t *testing.T, text, password string) {
	t.Helper()
	for _, form := range passwordForms(password) {
		// The userinfo form carries the user name; only its password part matters.
		if i := strings.Index(form, ":"); i >= 0 && strings.HasPrefix(form, "u:") {
			form = form[i+1:]
		}
		assert.NotContains(t, text, form, "text must not hold the password")
	}
}

func newCredentialProvider(cfg DatabaseConfig) *PostgreSQLProvider {
	p := NewPostgreSQLProvider(logging.GetDefault())
	p.config = cfg
	return p
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

	t.Run("a DSN that carries credentials and the two fields is refused without printing either", func(t *testing.T) {
		p := newCredentialProvider(DatabaseConfig{Username: "shelly", Password: trickyPassword})

		_, err := p.buildDSN("postgresql://inline_user:inline_pw@db.example/shelly", nil)
		require.Error(t, err)
		for _, secret := range []string{"inline_user", "inline_pw", "shelly:", trickyPassword} {
			assert.NotContains(t, err.Error(), secret)
		}
		assert.Contains(t, err.Error(), "database.username")
	})

	t.Run("a password without a username is refused", func(t *testing.T) {
		p := newCredentialProvider(DatabaseConfig{Password: trickyPassword})

		_, err := p.buildDSN("postgresql://db.example/shelly", nil)
		require.Error(t, err)
		assertNoPassword(t, err.Error(), trickyPassword)
		assert.Contains(t, err.Error(), "database.username")
	})
}

func TestPostgreSQLCredentials_ErrorsHoldNoPassword(t *testing.T) {
	t.Run("a refused connection", func(t *testing.T) {
		// Port 1 on the loopback address is closed: the connect fails at once and
		// no server is needed.
		p := NewPostgreSQLProvider(logging.GetDefault())
		err := p.Connect(DatabaseConfig{
			Provider: "postgresql",
			DSN:      "postgresql://127.0.0.1:1/shelly",
			Username: "shelly",
			Password: trickyPassword,
			Options:  map[string]string{"sslmode": "disable", "connect_timeout": "1"},
		})
		require.Error(t, err)
		assertNoPassword(t, err.Error(), trickyPassword)
	})

	t.Run("a DSN that cannot be built", func(t *testing.T) {
		p := NewPostgreSQLProvider(logging.GetDefault())
		err := p.Connect(DatabaseConfig{
			Provider: "postgresql",
			DSN:      "postgresql://inline_user:" + url.QueryEscape("in line") + "@db.example:badport/shelly",
			Username: "shelly",
			Password: trickyPassword,
		})
		require.Error(t, err)
		assertNoPassword(t, err.Error(), trickyPassword)
		assert.NotContains(t, err.Error(), "in+line")
		assert.NotContains(t, err.Error(), "in%20line")
	})
}

func TestPostgreSQLSanitizeError_Password(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		cfg  DatabaseConfig
		gone []string
	}{
		{
			name: "the known password in an unkeyed position, raw and escaped",
			msg:  fmt.Sprintf("dial failed near %s then %s", trickyPassword, url.QueryEscape(trickyPassword)),
			cfg:  DatabaseConfig{Username: "shelly", Password: trickyPassword},
			gone: passwordForms(trickyPassword),
		},
		{
			name: "a keyed value starting with s",
			msg:  "cannot parse `host=h password=secret user=u`",
			gone: []string{"secret"},
		},
		{
			name: "a keyed value holding an equals sign",
			msg:  "cannot parse `host=h password=a=b user=u`",
			gone: []string{"a=b"},
		},
		{
			name: "URL userinfo",
			msg:  "failed to connect to postgresql://shelly:hunter2@db.example/shelly",
			gone: []string{"hunter2"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newCredentialProvider(tc.cfg)

			got := p.sanitizeError(fmt.Errorf("%s", tc.msg))
			require.Error(t, got)
			for _, secret := range tc.gone {
				assert.NotContains(t, got.Error(), secret)
			}
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
	cfg := DatabaseConfig{
		Provider: "postgresql",
		DSN:      "postgresql://inline_user:inline_pw@db.example/shelly",
		Username: "shelly",
		Password: trickyPassword,
	}

	for name, got := range map[string]string{
		"%v value":    fmt.Sprintf("%v", cfg),
		"%+v value":   fmt.Sprintf("%+v", cfg),
		"%v pointer":  fmt.Sprintf("%v", &cfg),
		"%+v pointer": fmt.Sprintf("%+v", &cfg),
		"String()":    cfg.String(),
	} {
		t.Run(name, func(t *testing.T) {
			assertNoPassword(t, got, trickyPassword)
			assert.NotContains(t, got, "inline_pw")
			assert.Contains(t, got, "db.example", "the host stays readable")
		})
	}
}
