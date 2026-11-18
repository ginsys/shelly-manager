package database

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/config"
)

// A deployment that sets only the provider and the DSN must get the provider's
// pool on a live server, through the same Load -> GetDatabaseConfig -> provider
// path the server takes (#570). The values are read from the opened pool, not
// from the config struct. PostgreSQL and MySQL are env-gated like the other
// provider tests and skip without a server; CI supplies them.
func TestPoolDefaultProviders(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) providerFixture
		open  int
	}{
		{name: "postgresql", setup: setupPostgreSQLFixture, open: 25},
		{name: "mysql", setup: setupMySQLFixture, open: 25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := tt.setup(t) // skips this provider when unavailable
			cfg := loadProviderOnlyConfig(t, fixture)

			manager, err := NewManagerFromConfigWithLogger(cfg, testLogger(t))
			require.NoError(t, err)
			t.Cleanup(func() { _ = manager.Close() })

			sqlDB, err := manager.GetDB().DB()
			require.NoError(t, err)
			require.Equal(t, tt.open, sqlDB.Stats().MaxOpenConnections,
				"a config with only provider and dsn must get the %s pool", tt.name)
		})
	}

	t.Run("sqlite keeps a pool of one", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "shelly.db")
		cfg := loadConfigYAML(t, fmt.Sprintf("database:\n  provider: sqlite\n  dsn: %q\n", path))

		manager, err := NewManagerFromConfigWithLogger(cfg, testLogger(t))
		require.NoError(t, err)
		t.Cleanup(func() { _ = manager.Close() })

		sqlDB, err := manager.GetDB().DB()
		require.NoError(t, err)
		require.Equal(t, 1, sqlDB.Stats().MaxOpenConnections)
	})
}

// loadProviderOnlyConfig writes a config file holding the fixture's provider,
// DSN and driver options, and no pool keys, and loads it the way the server does.
func loadProviderOnlyConfig(t *testing.T, fixture providerFixture) *config.Config {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "database:\n  provider: %q\n  dsn: %q\n", fixture.config.Provider, fixture.config.DSN)
	if len(fixture.config.Options) > 0 {
		b.WriteString("  options:\n")
		keys := make([]string, 0, len(fixture.config.Options))
		for k := range fixture.config.Options {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "    %s: %q\n", k, fixture.config.Options[k])
		}
	}
	return loadConfigYAML(t, b.String())
}

func loadConfigYAML(t *testing.T, content string) *config.Config {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	cfg, err := config.Load(path)
	require.NoError(t, err)
	return cfg
}
