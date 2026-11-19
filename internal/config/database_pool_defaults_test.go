package config

import (
	"testing"
	"time"
)

// A deployment that sets only provider and dsn must get the provider's pool,
// not the SQLite value of 1 (#570). An explicit non-zero value is never replaced.
func TestGetDatabaseConfig_ProviderPoolDefaults(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		env      map[string]string
		wantOpen int
		wantIdle int
	}{
		{
			name:     "sqlite keeps a pool of one",
			content:  "database:\n  provider: sqlite\n",
			wantOpen: 1, wantIdle: 1,
		},
		{
			name:     "no database section is sqlite",
			content:  "server:\n  port: 8080\n",
			wantOpen: 1, wantIdle: 1,
		},
		{
			name:     "sqlite3 alias keeps a pool of one",
			content:  "database:\n  provider: sqlite3\n",
			wantOpen: 1, wantIdle: 1,
		},
		{
			name:     "postgresql with only provider and dsn",
			content:  "database:\n  provider: postgresql\n  dsn: host=db user=app dbname=shelly\n",
			wantOpen: 25, wantIdle: 5,
		},
		{
			name:     "postgres alias gets the postgresql pool",
			content:  "database:\n  provider: postgres\n  dsn: host=db user=app dbname=shelly\n",
			wantOpen: 25, wantIdle: 5,
		},
		{
			name:     "mysql with only provider and dsn",
			content:  "database:\n  provider: mysql\n  dsn: app:pw@tcp(db:3306)/shelly\n",
			wantOpen: 25, wantIdle: 5,
		},
		{
			name:     "provider from the environment only",
			content:  "server:\n  port: 8080\n",
			env:      map[string]string{"SHELLY_DATABASE_PROVIDER": "postgresql", "SHELLY_DATABASE_DSN": "host=db"},
			wantOpen: 25, wantIdle: 5,
		},
		{
			name:     "an explicit 1 on postgresql stays 1",
			content:  "database:\n  provider: postgresql\n  dsn: host=db\n  max_open_conns: 1\n  max_idle_conns: 1\n",
			wantOpen: 1, wantIdle: 1,
		},
		{
			name:     "only max_open_conns set: idle never exceeds it",
			content:  "database:\n  provider: postgresql\n  dsn: host=db\n  max_open_conns: 3\n",
			wantOpen: 3, wantIdle: 3,
		},
		{
			name:     "only max_open_conns set above the idle default",
			content:  "database:\n  provider: postgresql\n  dsn: host=db\n  max_open_conns: 8\n",
			wantOpen: 8, wantIdle: 5,
		},
		{
			name:     "explicit values are kept",
			content:  "database:\n  provider: mysql\n  dsn: app:pw@tcp(db)/shelly\n  max_open_conns: 10\n  max_idle_conns: 2\n",
			wantOpen: 10, wantIdle: 2,
		},
		{
			name:     "max_open_conns from the environment reaches the struct",
			content:  "database:\n  provider: postgresql\n  dsn: host=db\n",
			env:      map[string]string{"SHELLY_DATABASE_MAX_OPEN_CONNS": "40"},
			wantOpen: 40, wantIdle: 5,
		},
		{
			name:     "an unknown provider keeps its own fallback",
			content:  "database:\n  provider: other\n  dsn: x\n",
			wantOpen: 10, wantIdle: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			cfg := loadDatabaseConfigFile(t, tt.content)
			got := cfg.GetDatabaseConfig()
			if got.MaxOpenConns != tt.wantOpen || got.MaxIdleConns != tt.wantIdle {
				t.Errorf("pool = %d open / %d idle, want %d / %d",
					got.MaxOpenConns, got.MaxIdleConns, tt.wantOpen, tt.wantIdle)
			}
		})
	}
}

// The image bakes configs/shelly-manager.yaml. If that file sets the pool, a
// PostgreSQL deployment that only switches provider and dsn keeps a pool of one
// from the file whatever the built-in default says (#570).
func TestExampleConfig_PoolFollowsTheProvider(t *testing.T) {
	const example = "../../configs/shelly-manager.yaml"

	tests := []struct {
		provider string
		open     int
		idle     int
	}{
		{"sqlite", 1, 1},
		{"postgresql", 25, 5},
		{"mysql", 25, 5},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			t.Setenv("SHELLY_DATABASE_PROVIDER", tt.provider)
			cfg, err := Load(example)
			if err != nil {
				t.Fatalf("Load(%s): %v", example, err)
			}
			got := cfg.GetDatabaseConfig()
			if got.MaxOpenConns != tt.open || got.MaxIdleConns != tt.idle {
				t.Errorf("pool = %d open / %d idle, want %d / %d",
					got.MaxOpenConns, got.MaxIdleConns, tt.open, tt.idle)
			}
		})
	}
}

// The lifetime and idle time defaults are the helper's too (ginsys/shelly-manager#581): a configuration
// that sets neither gets the same pair on every provider, a value the operator set is kept, and the
// values viper registers are those constants, not a second copy.
func TestGetDatabaseConfig_LifetimeAndIdleTimeDefaults(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		wantLifetime time.Duration
		wantIdleTime time.Duration
	}{
		{"sqlite", "database:\n  provider: sqlite\n", 5 * time.Minute, 10 * time.Minute},
		{"postgresql", "database:\n  provider: postgresql\n  dsn: host=db\n", 5 * time.Minute, 10 * time.Minute},
		{"mysql", "database:\n  provider: mysql\n  dsn: app:pw@tcp(db)/shelly\n", 5 * time.Minute, 10 * time.Minute},
		{"set values are kept", "database:\n  provider: mysql\n  dsn: app:pw@tcp(db)/shelly\n  conn_max_lifetime: 1800\n  conn_max_idle_time: 60\n",
			30 * time.Minute, time.Minute},
		{"an explicit 0 gets the default", "database:\n  provider: mysql\n  dsn: app:pw@tcp(db)/shelly\n  conn_max_lifetime: 0\n  conn_max_idle_time: 0\n",
			5 * time.Minute, 10 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loadDatabaseConfigFile(t, tt.content).GetDatabaseConfig()
			if got.ConnMaxLifetime != tt.wantLifetime || got.ConnMaxIdleTime != tt.wantIdleTime {
				t.Errorf("lifetime / idle time = %v / %v, want %v / %v",
					got.ConnMaxLifetime, got.ConnMaxIdleTime, tt.wantLifetime, tt.wantIdleTime)
			}
		})
	}

	t.Run("viper registers the constants", func(t *testing.T) {
		cfg := loadDatabaseConfigFile(t, "database:\n  provider: sqlite\n")
		if got, want := time.Duration(cfg.Database.ConnMaxLifetime)*time.Second, defaultConnMaxLifetime; got != want {
			t.Errorf("registered lifetime = %v, want %v", got, want)
		}
		if got, want := time.Duration(cfg.Database.ConnMaxIdleTime)*time.Second, defaultConnMaxIdleTime; got != want {
			t.Errorf("registered idle time = %v, want %v", got, want)
		}
	})
}
