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
