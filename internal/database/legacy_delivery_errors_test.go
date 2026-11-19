package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/notification"
)

// History rows written before ginsys/shelly-manager#652 hold a failed send's error text, which quotes the
// request URL and so the webhook token. notification.RemoveLegacyDeliveryErrors replaces each such text
// by its exact prefix and a note. It is a write query, so every provider runs the same cases.
func checkLegacyDeliveryErrors(t *testing.T, manager *Manager) {
	t.Helper()
	db := manager.GetDB()
	const token = "LEAKTOKEN652"

	// Start from a database that holds no old row left by an earlier run.
	_, err := notification.RemoveLegacyDeliveryErrors(db)
	require.NoError(t, err)

	suffix := fmt.Sprint(time.Now().UnixNano())
	channel := &notification.NotificationChannel{Name: "legacy-" + suffix, Type: "webhook", Enabled: true}
	require.NoError(t, db.Create(channel).Error)
	rule := &notification.NotificationRule{Name: "legacy-" + suffix, Enabled: true, ChannelID: channel.ID, AlertLevel: "all"}
	require.NoError(t, db.Create(rule).Error)
	t.Cleanup(func() {
		db.Where("rule_id = ?", rule.ID).Delete(&notification.NotificationHistory{})
		db.Delete(rule)
		db.Delete(channel)
	})

	seed := func(status, text string) uint {
		h := &notification.NotificationHistory{
			RuleID: rule.ID, ChannelID: channel.ID, TriggerType: "test", Status: status, Error: text, CreatedAt: time.Now(),
		}
		require.NoError(t, db.Create(h).Error)
		return h.ID
	}
	old := map[string]uint{
		"failed to send webhook:":            seed("failed", `failed to send webhook: Post "http://h.example/`+token+`?t=q": dial tcp: connection refused`),
		"failed to send slack notification:": seed("failed", `failed to send slack notification: Post "https://hooks.slack.com/services/`+token+`": EOF`),
		"failed to create request:":          seed("failed", `failed to create request: parse "http://h:bad/`+token+`": invalid port`),
		"failed to create slack request:":    seed("failed", `failed to create slack request: parse "https://x/`+token+`": bad`),
	}
	newText := seed("failed", "webhook request failed: Post timed out")
	otherText := seed("failed", "webhook returned status 500")
	sentRow := seed("sent", "")
	contained := seed("failed", `email failed, then failed to send webhook: x`) // the prefix must start the text

	text := func(id uint) string {
		var h notification.NotificationHistory
		require.NoError(t, db.First(&h, id).Error)
		return h.Error
	}

	changed, err := notification.RemoveLegacyDeliveryErrors(db)
	require.NoError(t, err)
	assert.EqualValues(t, 4, changed, "the four old rows, and nothing else")
	for prefix, id := range old {
		assert.Equal(t, prefix+" details removed", text(id))
		assert.NotContains(t, text(id), token)
	}
	assert.Equal(t, "webhook request failed: Post timed out", text(newText))
	assert.Equal(t, "webhook returned status 500", text(otherText))
	assert.Equal(t, "", text(sentRow))
	assert.Equal(t, `email failed, then failed to send webhook: x`, text(contained))

	// A second start changes no row.
	changed, err = notification.RemoveLegacyDeliveryErrors(db)
	require.NoError(t, err)
	assert.EqualValues(t, 0, changed, "idempotent")
	assert.Equal(t, "failed to send webhook: details removed", text(old["failed to send webhook:"]))
}

func TestRemoveLegacyDeliveryErrors_SQLite(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	checkLegacyDeliveryErrors(t, manager)
}

// The same cases on a PostgreSQL and a MySQL server. Each subtest skips when its server is not
// configured; the Database Provider Migration job of the CI supplies both.
func TestRemoveLegacyDeliveryErrorsProviders(t *testing.T) {
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
			checkLegacyDeliveryErrors(t, startProviderManager(t, fixture))
		})
	}
}
