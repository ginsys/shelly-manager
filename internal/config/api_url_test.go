package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Only the provisioner agent reads api.url, and it needs the manager's real address. A default that points
// at 0.0.0.0 reaches nothing and hides the missing setting, so the default is empty and the agent says so.
func TestAPIURL_DefaultIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.API.URL != "" {
		t.Errorf("api.url default = %q, want empty", cfg.API.URL)
	}
}

func TestAPIURL_FileAndEnvStillSetIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "url.yaml")
	if err := os.WriteFile(path, []byte("api:\n  url: \"http://manager:80\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.API.URL != "http://manager:80" {
		t.Errorf("api.url from the file = %q, want http://manager:80", cfg.API.URL)
	}

	t.Setenv("SHELLY_API_URL", "http://prod-shelly-manager:80")
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.API.URL != "http://prod-shelly-manager:80" {
		t.Errorf("api.url from SHELLY_API_URL = %q, want http://prod-shelly-manager:80", cfg.API.URL)
	}
}

// A sample config must not ship the address the default no longer holds. Both samples carry an api
// section, and the manager's copy is the one a person starts from.
func TestAPIURL_SampleProvisionerConfigLeavesItEmpty(t *testing.T) {
	for _, sample := range []string{"shelly-provisioner.yaml", "shelly-manager.yaml"} {
		t.Run(sample, func(t *testing.T) {
			cfg, err := Load("../../configs/" + sample)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.API.URL != "" {
				t.Errorf("configs/%s sets api.url = %q, want it empty", sample, cfg.API.URL)
			}
		})
	}
}
