package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadDatabaseConfigFile(t *testing.T, content string) *Config {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	return cfg
}

// The connection target must follow database.dsn when it is set, then
// database.path, then the built-in default (#276).
func TestGetDatabaseConfig_ConnectionTarget(t *testing.T) {
	tests := []struct {
		name    string
		content string
		env     map[string]string
		want    string
	}{
		{
			name:    "no database section uses the default file",
			content: "server:\n  port: 8080\n",
			want:    "data/shelly.db",
		},
		{
			name:    "path from the config file",
			content: "database:\n  path: data/shelly-dev.db\n",
			want:    "data/shelly-dev.db",
		},
		{
			name:    "dsn from the config file beats path",
			content: "database:\n  path: data/ignored.db\n  dsn: data/explicit.db\n",
			want:    "data/explicit.db",
		},
		{
			name:    "path from the environment",
			content: "server:\n  port: 8080\n",
			env:     map[string]string{"SHELLY_DATABASE_PATH": "/data/env.db"},
			want:    "/data/env.db",
		},
		{
			name:    "dsn from the environment without a dsn key in the file",
			content: "database:\n  path: data/file.db\n",
			env:     map[string]string{"SHELLY_DATABASE_DSN": "/data/env-dsn.db"},
			want:    "/data/env-dsn.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			cfg := loadDatabaseConfigFile(t, tt.content)
			if got := cfg.GetDatabaseConfig().DSN; got != tt.want {
				t.Errorf("GetDatabaseConfig().DSN = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDatabaseTarget(t *testing.T) {
	sqlite := loadDatabaseConfigFile(t, "database:\n  path: data/shelly-dev.db\n")
	if got := sqlite.DatabaseTarget(); got != "data/shelly-dev.db" {
		t.Errorf("SQLite DatabaseTarget() = %q, want the file path", got)
	}

	const secret = "s3cret"
	pg := loadDatabaseConfigFile(t,
		"database:\n  provider: postgresql\n  dsn: postgres://app:"+secret+"@db:5432/shelly\n")
	got := pg.DatabaseTarget()
	if strings.Contains(got, secret) {
		t.Errorf("PostgreSQL DatabaseTarget() = %q, leaks the DSN password", got)
	}
	if !strings.Contains(got, "postgresql") {
		t.Errorf("PostgreSQL DatabaseTarget() = %q, want it to name the provider", got)
	}
}
