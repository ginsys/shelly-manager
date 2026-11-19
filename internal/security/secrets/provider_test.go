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

// A secret variable that is set but empty counts as present and beats its _FILE form, so the shipped
// compose file (it always sets both plain variables, empty by default) ignores a key file given beside
// them. The docs say so (docs/security/SECURITY_SECRETS.md). ginsys/shelly-manager#672 changes this
// and flips this test.
func TestApplyToConfig_EmptyPlainVariableWinsOverFile(t *testing.T) {
	t.Setenv("SHELLY_SECURITY_ADMIN_API_KEY", "")
	t.Setenv("SHELLY_SECURITY_ADMIN_API_KEY_FILE", writeKeyFile(t, "adminkey123\n"))
	t.Setenv("SHELLY_SECURITY_AGENT_API_KEY", "")
	t.Setenv("SHELLY_SECURITY_AGENT_API_KEY_FILE", writeKeyFile(t, "agentkey456\n"))

	cfg := &config.Config{}
	ApplyToConfig(cfg)

	if cfg.Security.AdminAPIKey != "" || cfg.Security.AgentAPIKey != "" {
		t.Fatalf("expected the empty plain variables to win, got admin %q and agent %q",
			cfg.Security.AdminAPIKey, cfg.Security.AgentAPIKey)
	}
}
