package database

import (
	"net/url"
	"strings"
	"testing"

	"github.com/ginsys/shelly-manager/internal/config"
)

// The server calls NewManagerFromConfigWithLogger at start and prints the error
// it returns. A connect failure with a database password set must leave no
// spelling of the password in that text (#571). Port 1 on the loopback address
// is closed, so no database is needed.
func TestNewManagerFromConfig_ConnectFailureHoldsNoPassword(t *testing.T) {
	const password = "p@ss:w/rd%41;--x=y s3cret"

	var cfg config.Config
	cfg.Database.Provider = "postgresql"
	cfg.Database.DSN = "postgresql://127.0.0.1:1/shelly"
	cfg.Database.Username = "shelly"
	cfg.Database.Password = password
	cfg.Database.Options = map[string]string{"sslmode": "disable", "connect_timeout": "1"}

	_, err := NewManagerFromConfig(&cfg)
	if err == nil {
		t.Fatal("expected the connection to a closed port to fail")
	}

	for _, form := range []string{password, url.QueryEscape(password), url.PathEscape(password)} {
		if strings.Contains(err.Error(), form) {
			t.Errorf("error text holds the password (%q): %s", form, err)
		}
	}
}
