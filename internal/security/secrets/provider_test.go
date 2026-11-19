package secrets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ginsys/shelly-manager/internal/config"
)

func TestGetEnvOrFile_FileTakesEffect(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "secret.txt")
	content := "supersecret\n"
	if err := os.WriteFile(fp, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	t.Setenv("DEMO_SECRET_FILE", fp)

	got, ok := GetEnvOrFile("DEMO_SECRET")
	if !ok {
		t.Fatalf("expected secret to be found via *_FILE")
	}
	if got != "supersecret" { // trailing newline trimmed
		t.Fatalf("unexpected secret value: %q", got)
	}
}

func TestApplyToConfig_OverridesFromEnvFile(t *testing.T) {
	// A key variable exported on the runner must not change the result.
	clearKeyEnv(t)
	// Prepare env-file for admin api key
	dir := t.TempDir()
	fp := filepath.Join(dir, "adminkey")
	if err := os.WriteFile(fp, []byte("adminkey123\n"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	t.Setenv("SHELLY_SECURITY_ADMIN_API_KEY_FILE", fp)

	cfg := &config.Config{}
	cfg.Security.AdminAPIKey = ""

	ApplyToConfig(cfg)

	if cfg.Security.AdminAPIKey != "adminkey123" {
		t.Fatalf("expected AdminAPIKey from file, got %q", cfg.Security.AdminAPIKey)
	}
}

func writeKeyFile(t *testing.T, content string) string {
	t.Helper()
	fp := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(fp, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return fp
}

// clearKeyEnv makes the four key variables absent for the test and restores them afterwards: t.Setenv
// records the original value, and t.Setenv(k, "") alone would leave the variable set but empty.
func clearKeyEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"SHELLY_SECURITY_ADMIN_API_KEY", "SHELLY_SECURITY_ADMIN_API_KEY_FILE",
		"SHELLY_SECURITY_AGENT_API_KEY", "SHELLY_SECURITY_AGENT_API_KEY_FILE",
	} {
		t.Setenv(k, "")
		if err := os.Unsetenv(k); err != nil {
			t.Fatalf("unset %s: %v", k, err)
		}
	}
}

// The agent key reads its _FILE form like the admin key does (ginsys/shelly-manager#623): the file is
// read, its trailing newlines are trimmed, and the admin key is left alone.
func TestApplyToConfig_AgentKeyFromFile(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("SHELLY_SECURITY_AGENT_API_KEY_FILE", writeKeyFile(t, "agentkey456\n"))

	cfg := &config.Config{}
	ApplyToConfig(cfg)

	if cfg.Security.AgentAPIKey != "agentkey456" {
		t.Fatalf("expected AgentAPIKey from file, got %q", cfg.Security.AgentAPIKey)
	}
	if cfg.Security.AdminAPIKey != "" {
		t.Fatalf("the agent key file must not set the admin key, got %q", cfg.Security.AdminAPIKey)
	}
}

func TestApplyToConfig_AgentKeyPlainVariableWinsOverFile(t *testing.T) {
	t.Setenv("SHELLY_SECURITY_AGENT_API_KEY", "from-env")
	t.Setenv("SHELLY_SECURITY_AGENT_API_KEY_FILE", writeKeyFile(t, "from-file\n"))

	cfg := &config.Config{}
	ApplyToConfig(cfg)

	if cfg.Security.AgentAPIKey != "from-env" {
		t.Fatalf("expected the plain variable to win, got %q", cfg.Security.AgentAPIKey)
	}
}

func TestApplyToConfig_AgentKeyKeptWithoutVariable(t *testing.T) {
	clearKeyEnv(t)
	cfg := &config.Config{}
	cfg.Security.AgentAPIKey = "from-config"
	ApplyToConfig(cfg)

	if cfg.Security.AgentAPIKey != "from-config" {
		t.Fatalf("expected the configured key to stay, got %q", cfg.Security.AgentAPIKey)
	}
}

// An empty value counts as absent, for the variable and for the file content alike (the rule viper
// applies to its own env lookup). The shipped compose file sets both plain variables, empty by default,
// so a key file given beside them must still be read (ginsys/shelly-manager#672).
func TestApplyToConfig_EmptyPlainVariableFallsThroughToFile(t *testing.T) {
	t.Setenv("SHELLY_SECURITY_ADMIN_API_KEY", "")
	t.Setenv("SHELLY_SECURITY_ADMIN_API_KEY_FILE", writeKeyFile(t, "adminkey123\n"))
	t.Setenv("SHELLY_SECURITY_AGENT_API_KEY", "")
	t.Setenv("SHELLY_SECURITY_AGENT_API_KEY_FILE", writeKeyFile(t, "agentkey456\n"))

	cfg := &config.Config{}
	ApplyToConfig(cfg)

	if cfg.Security.AdminAPIKey != "adminkey123" || cfg.Security.AgentAPIKey != "agentkey456" {
		t.Fatalf("expected both keys from their files, got admin %q and agent %q",
			cfg.Security.AdminAPIKey, cfg.Security.AgentAPIKey)
	}
}

// An empty plain variable with no file leaves the configured key alone. An empty admin key means "no
// guard", so blanking a key set in the YAML file would open every guarded route.
func TestApplyToConfig_EmptyPlainVariableKeepsConfiguredWithoutFile(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("SHELLY_SECURITY_ADMIN_API_KEY", "")
	t.Setenv("SHELLY_SECURITY_AGENT_API_KEY", "")

	cfg := &config.Config{}
	cfg.Security.AdminAPIKey = "admin-from-config"
	cfg.Security.AgentAPIKey = "agent-from-config"
	ApplyToConfig(cfg)

	if cfg.Security.AdminAPIKey != "admin-from-config" || cfg.Security.AgentAPIKey != "agent-from-config" {
		t.Fatalf("expected the configured keys to stay, got admin %q and agent %q",
			cfg.Security.AdminAPIKey, cfg.Security.AgentAPIKey)
	}
}

// A key file that was created but never filled (empty, or only newlines) is absent too, so it keeps the
// configured key instead of blanking it.
func TestApplyToConfig_EmptyKeyFileKeepsConfigured(t *testing.T) {
	for name, content := range map[string]string{"empty": "", "newlines only": "\n\n"} {
		t.Run(name, func(t *testing.T) {
			clearKeyEnv(t)
			t.Setenv("SHELLY_SECURITY_ADMIN_API_KEY_FILE", writeKeyFile(t, content))
			t.Setenv("SHELLY_SECURITY_AGENT_API_KEY_FILE", writeKeyFile(t, content))

			cfg := &config.Config{}
			cfg.Security.AdminAPIKey = "admin-from-config"
			cfg.Security.AgentAPIKey = "agent-from-config"
			ApplyToConfig(cfg)

			if cfg.Security.AdminAPIKey != "admin-from-config" || cfg.Security.AgentAPIKey != "agent-from-config" {
				t.Fatalf("expected the configured keys to stay, got admin %q and agent %q",
					cfg.Security.AdminAPIKey, cfg.Security.AgentAPIKey)
			}
		})
	}
}

func TestGetEnvOrFile_EmptyPlainFallsThroughAndEmptyIsAbsent(t *testing.T) {
	const key = "DEMO_EMPTY_SECRET"
	unset := func(t *testing.T) {
		t.Helper()
		for _, k := range []string{key, key + "_FILE"} {
			t.Setenv(k, "")
			if err := os.Unsetenv(k); err != nil {
				t.Fatalf("unset %s: %v", k, err)
			}
		}
	}

	t.Run("empty plain falls through to the file", func(t *testing.T) {
		unset(t)
		t.Setenv(key, "")
		t.Setenv(key+"_FILE", writeKeyFile(t, "from-file\n"))
		if got, ok := GetEnvOrFile(key); !ok || got != "from-file" {
			t.Fatalf("expected the file value, got %q, %v", got, ok)
		}
	})
	t.Run("empty plain alone is absent", func(t *testing.T) {
		unset(t)
		t.Setenv(key, "")
		if got, ok := GetEnvOrFile(key); ok || got != "" {
			t.Fatalf("expected absent, got %q, %v", got, ok)
		}
	})
	for name, content := range map[string]string{"empty file": "", "newlines only": "\n\n"} {
		t.Run(name+" is absent", func(t *testing.T) {
			unset(t)
			t.Setenv(key+"_FILE", writeKeyFile(t, content))
			if got, ok := GetEnvOrFile(key); ok || got != "" {
				t.Fatalf("expected absent, got %q, %v", got, ok)
			}
		})
	}
}
