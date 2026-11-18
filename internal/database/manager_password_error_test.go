package database

import (
	"net/url"
	"strings"
	"testing"

	"github.com/ginsys/shelly-manager/internal/config"
)

const testPassword = "p@ss:w/rd%41;--x=y s3cret"

func assertTextHoldsNoPassword(t *testing.T, text, password string) {
	t.Helper()
	for _, form := range []string{
		password,
		url.QueryEscape(password),
		url.PathEscape(password),
		strings.TrimPrefix(url.UserPassword("", password).String(), ":"),
	} {
		if strings.Contains(text, form) {
			t.Errorf("error text holds the password (%q): %s", form, text)
		}
	}
}

// The server calls NewManagerFromConfig at start and prints the error it
// returns. A connect failure must leave no spelling of a password in that text
// (ginsys/shelly-manager#571). Port 1 on the loopback address is closed, so no
// database is needed.
func TestNewManagerFromConfig_ConnectFailureHoldsNoPassword(t *testing.T) {
	t.Run("a refused connection with a separate password", func(t *testing.T) {
		// The driver's own refused-connection error omits the password, so this is
		// a guard for the whole path; the subtest below is the one that fails when
		// the sanitising in the provider is removed.
		var cfg config.Config
		cfg.Database.Provider = "postgresql"
		cfg.Database.DSN = "postgresql://127.0.0.1:1/shelly"
		cfg.Database.Username = "shelly"
		cfg.Database.Password = testPassword
		cfg.Database.Options = map[string]string{"sslmode": "disable", "connect_timeout": "1"}

		_, err := NewManagerFromConfig(&cfg)
		if err == nil {
			t.Fatal("expected the connection to a closed port to fail")
		}
		assertTextHoldsNoPassword(t, err.Error(), testPassword)
	})

	// net/url quotes the whole DSN in its parse error. That text must not reach
	// the startup error, whichever spelling the password sits in.
	for name, dsn := range map[string]string{
		"userinfo":     "postgresql://inline_user:inline_pw@127.0.0.1:badport/shelly",
		"encoded key":  "postgresql://127.0.0.1:badport/shelly?pass%77ord=inline_pw",
		"padded key":   "postgresql://127.0.0.1:badport/shelly? password =inline_pw",
		"slash":        "postgresql://app:inline_pw/tail@127.0.0.1/shelly",
		"after a hash": "postgresql://127.0.0.1:badport/shelly?application_name=x#&password=inline_pw",
	} {
		t.Run("a DSN that cannot be parsed, password in "+name, func(t *testing.T) {
			var cfg config.Config
			cfg.Database.Provider = "postgresql"
			cfg.Database.DSN = dsn

			_, err := NewManagerFromConfig(&cfg)
			if err == nil {
				t.Fatal("expected a DSN with a bad port to fail")
			}
			if strings.Contains(err.Error(), "inline_pw") {
				t.Errorf("error text holds the inline password: %s", err)
			}
		})
	}
}
