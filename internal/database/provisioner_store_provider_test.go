package database

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The same cases on a PostgreSQL and a MySQL server. On MySQL they prove the binary collation (three ids
// that differ by case or by a trailing space are three agents) and the LONGTEXT columns (an 8 MiB config
// round-trips). Each subtest skips when its server is not configured; the Database Provider Migration
// job of the CI supplies both, and its -run list names this test.
func TestProvisionerStoreProviders(t *testing.T) {
	fixtures := []struct {
		name  string
		setup func(t *testing.T) providerFixture
	}{
		{name: "postgresql", setup: setupPostgreSQLFixture},
		{name: "mysql", setup: setupMySQLFixture},
	}
	for _, spec := range fixtures {
		t.Run(spec.name, func(t *testing.T) {
			fixture := spec.setup(t)
			checkProvisionerStore(t, startProviderManager(t, fixture))
			checkProvisionerEnrolment(t, startProviderManager(t, fixture))
			checkProvisionerIDRule(t, startProviderManager(t, fixture))
			checkProvisionerPurgeByState(t, startProviderManager(t, fixture))
			checkProvisionerAdmin(t, startProviderManager(t, fixture))
			checkProvisionerKeyed(t, startProviderManager(t, fixture))
			checkProvisionerStrict(t, startProviderManager(t, fixture))

			// A restart runs AutoMigrate over a populated database: it must leave the big config in place.
			t.Run("a restart keeps what the tables hold", func(t *testing.T) {
				reopened := startProviderManager(t, fixture)
				task, err := NewProvisionerStore(reopened.GetDB()).GetTask(provisionerCtx(t), "big")
				require.NoError(t, err)
				assert.Equal(t, strings.Repeat("x", 8<<20), task.Config["blob"])
			})

			if spec.name == "mysql" {
				t.Run("a DSN with clientFoundRows", func(t *testing.T) {
					found := fixture
					found.config.DSN += "&clientFoundRows=true"
					checkReRegisterReplacesToken(t, startProviderManager(t, found))
				})
			}
		})
	}
}

// The same re-register case on SQLite, where it holds without any option.
func TestProvisionerStore_ReRegisterReplacesToken(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	checkReRegisterReplacesToken(t, manager)
}
