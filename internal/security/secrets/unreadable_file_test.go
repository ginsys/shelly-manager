package secrets

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ginsys/shelly-manager/internal/config"
)

// secretVariables are the six variables ApplyToConfig reads, each with the config field it fills.
var secretVariables = []struct {
	name string
	get  func(*config.Config) *string
}{
	{"SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD", func(c *config.Config) *string { return &c.Notifications.Email.SMTPPassword }},
	{"SHELLY_OPNSENSE_API_KEY", func(c *config.Config) *string { return &c.OPNSense.APIKey }},
	{"SHELLY_OPNSENSE_API_SECRET", func(c *config.Config) *string { return &c.OPNSense.APISecret }},
	{"SHELLY_SECURITY_ADMIN_API_KEY", func(c *config.Config) *string { return &c.Security.AdminAPIKey }},
	{"SHELLY_SECURITY_AGENT_API_KEY", func(c *config.Config) *string { return &c.Security.AgentAPIKey }},
	{"SHELLY_API_KEY", func(c *config.Config) *string { return &c.API.Key }},
}

// A _FILE variable that is set to a path the process cannot read is an error naming the variable and
// the path, for every secret and for both ways a read fails (ginsys/shelly-manager#682): a typo in a
// mount path must not leave the key empty, which opens the guarded routes.
func TestApplyToConfig_UnreadableKeyFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	paths := map[string]string{
		"missing path": filepath.Join(dir, "no-such-secret"),
		"a directory":  dir,
	}
	for _, v := range secretVariables {
		for name, path := range paths {
			t.Run(v.name+"/"+name, func(t *testing.T) {
				unsetEnv(t, v.name, v.name+"_FILE")
				t.Setenv(v.name+"_FILE", path)

				cfg := &config.Config{}
				*v.get(cfg) = "from-config"
				err := ApplyToConfig(cfg)

				if err == nil {
					t.Fatalf("expected an error for %s_FILE=%s, got nil", v.name, path)
				}
				if !strings.Contains(err.Error(), v.name+"_FILE") || !strings.Contains(err.Error(), path) {
					t.Fatalf("expected the error to name the variable and the path, got %q", err)
				}
			})
		}
	}
}

func TestGetEnvOrFile_UnreadableFileWrapsTheCause(t *testing.T) {
	const key = "DEMO_UNREADABLE_SECRET"
	unsetEnv(t, key, key+"_FILE")
	t.Setenv(key+"_FILE", filepath.Join(t.TempDir(), "no-such-secret"))

	got, ok, err := GetEnvOrFile(key)

	if got != "" || ok {
		t.Fatalf("expected no value, got %q, %v", got, ok)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected an error wrapping fs.ErrNotExist, got %v", err)
	}
}

// A plain variable with a value wins before the file is looked at, so an unreadable file beside it is
// not an error (it is never read), and an unset or empty _FILE variable is absent, not an error.
func TestApplyToConfig_UnreadableFileIsOnlyAnErrorWhenItIsNeeded(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-secret")

	t.Run("plain value beside an unreadable file", func(t *testing.T) {
		clearKeyEnv(t)
		t.Setenv("SHELLY_SECURITY_ADMIN_API_KEY", "from-env")
		t.Setenv("SHELLY_SECURITY_ADMIN_API_KEY_FILE", missing)

		cfg := &config.Config{}
		mustApply(t, cfg)

		if cfg.Security.AdminAPIKey != "from-env" {
			t.Fatalf("expected the plain variable, got %q", cfg.Security.AdminAPIKey)
		}
	})
	t.Run("empty _FILE variable", func(t *testing.T) {
		clearKeyEnv(t)
		t.Setenv("SHELLY_SECURITY_ADMIN_API_KEY_FILE", "")

		cfg := &config.Config{}
		cfg.Security.AdminAPIKey = "from-config"
		mustApply(t, cfg)

		if cfg.Security.AdminAPIKey != "from-config" {
			t.Fatalf("expected the configured key to stay, got %q", cfg.Security.AdminAPIKey)
		}
	})
	t.Run("no _FILE variable", func(t *testing.T) {
		clearKeyEnv(t)

		cfg := &config.Config{}
		cfg.Security.AdminAPIKey = "from-config"
		mustApply(t, cfg)

		if cfg.Security.AdminAPIKey != "from-config" {
			t.Fatalf("expected the configured key to stay, got %q", cfg.Security.AdminAPIKey)
		}
	})
}
