package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/notification"
)

// The contract the notification pages rely on (ginsys/shelly-manager#250): one channel can be read, a
// rule can be deleted, a PUT replaces the channel and a create keeps enabled:false.

func plainWebhook(name string, enabled bool) map[string]any {
	return map[string]any{
		"name": name, "type": "webhook", "enabled": enabled, "description": "first",
		"config": map[string]any{"url": "http://127.0.0.1:1/hook-" + name, "secret": leakSecret},
	}
}

func (e *notificationSecretsEnv) channelRow(t *testing.T, id uint) notification.NotificationChannel {
	t.Helper()
	var ch notification.NotificationChannel
	require.NoError(t, e.db.First(&ch, id).Error)
	return ch
}

func (e *notificationSecretsEnv) createRule(t *testing.T, body map[string]any) uint {
	t.Helper()
	code, resp := e.do(t, "POST", "/notifications/rules", body)
	require.Equal(t, http.StatusCreated, code, resp)
	var wrap struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(resp), &wrap))
	return wrap.Data.ID
}

func (e *notificationSecretsEnv) ruleRow(t *testing.T, id uint) notification.NotificationRule {
	t.Helper()
	var rule notification.NotificationRule
	require.NoError(t, e.db.First(&rule, id).Error)
	return rule
}

// A PUT that carries enabled:false and an empty description writes them. Before, the update was a struct
// update that skips zero fields, so a disabled channel stayed enabled.
func TestNotificationContract_UpdateWritesFalseAndEmpty(t *testing.T) {
	env := setupNotificationSecrets(t)
	id, _ := env.createChannel(t, plainWebhook("hook", true))
	require.True(t, env.channelRow(t, id).Enabled)

	code, resp := env.do(t, "PUT", fmt.Sprintf("/notifications/channels/%d", id), map[string]any{
		"name": "hook", "type": "webhook", "enabled": false, "description": "",
		"config": map[string]any{"url": "http://127.0.0.1:1/hook-hook", "secret": "********"},
	})
	require.Equal(t, http.StatusOK, code, resp)

	row := env.channelRow(t, id)
	assert.False(t, row.Enabled, "enabled:false is written")
	assert.Equal(t, "", row.Description, "an empty description is written")
	assert.Equal(t, leakSecret, env.storedConfig(t, id)["secret"], "the placeholder restored the stored credential")
}

// PUT replaces the five fields: a body that omits enabled disables the channel and one that omits the
// description blanks it. This is the named behaviour of the replace (docs/api/API_NOTIFICATION.md).
func TestNotificationContract_UpdateReplacesTheFiveFields(t *testing.T) {
	env := setupNotificationSecrets(t)
	id, _ := env.createChannel(t, plainWebhook("hook", true))

	code, resp := env.do(t, "PUT", fmt.Sprintf("/notifications/channels/%d", id), map[string]any{
		"name": "renamed", "type": "webhook",
		"config": map[string]any{"url": "http://127.0.0.1:1/hook-hook"},
	})
	require.Equal(t, http.StatusOK, code, resp)

	row := env.channelRow(t, id)
	assert.Equal(t, "renamed", row.Name)
	assert.False(t, row.Enabled, "a body without enabled disables the channel")
	assert.Equal(t, "", row.Description, "a body without a description blanks it")
}

// A toggle as the page sends it: read the channel, send it back complete with enabled flipped. The stored
// credential survives and the channel flips both ways.
func TestNotificationContract_ToggleRoundTripKeepsCredentials(t *testing.T) {
	env := setupNotificationSecrets(t)
	id, _ := env.createChannel(t, deadWebhook(closedPort(t)))
	before := env.storedConfig(t, id)

	for _, enabled := range []bool{false, true} {
		code, body := env.do(t, "GET", fmt.Sprintf("/notifications/channels/%d", id), nil)
		require.Equal(t, http.StatusOK, code, body)
		var wrap struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &wrap))
		wrap.Data["enabled"] = enabled

		code, body = env.do(t, "PUT", fmt.Sprintf("/notifications/channels/%d", id), wrap.Data)
		require.Equal(t, http.StatusOK, code, body)
		assert.Equal(t, enabled, env.channelRow(t, id).Enabled)
		assert.Equal(t, before, env.storedConfig(t, id), "the stored config is unchanged by a toggle")
	}
}

// A create with enabled:false keeps it: the column has a default of true that gorm applies to a false.
func TestNotificationContract_CreateKeepsEnabledFalse(t *testing.T) {
	env := setupNotificationSecrets(t)

	chID, _ := env.createChannel(t, plainWebhook("off", false))
	assert.False(t, env.channelRow(t, chID).Enabled, "a channel created disabled is disabled")

	onID, _ := env.createChannel(t, plainWebhook("on", true))
	assert.True(t, env.channelRow(t, onID).Enabled)

	ruleID := env.createRule(t, map[string]any{"name": "off-rule", "enabled": false, "channel_id": chID, "alert_level": "all"})
	assert.False(t, env.ruleRow(t, ruleID).Enabled, "a rule created disabled is disabled")
	onRule := env.createRule(t, map[string]any{"name": "on-rule", "enabled": true, "channel_id": chID, "alert_level": "all"})
	assert.True(t, env.ruleRow(t, onRule).Enabled)
}

// GET one channel: the same masked object the list holds, a 404 for an unknown id, a 400 for a bad one.
func TestNotificationContract_GetChannel(t *testing.T) {
	env := setupNotificationSecrets(t)
	port := closedPort(t)
	hookID, _ := env.createChannel(t, deadWebhook(port))
	slackID, _ := env.createChannel(t, map[string]any{
		"name": "ops-slack", "type": "slack", "enabled": true,
		"config": map[string]any{"webhook_url": leakSlack, "channel": "#ops"},
	})

	_, listBody := env.do(t, "GET", "/notifications/channels", nil)
	var list struct {
		Data struct {
			Channels []map[string]any `json:"channels"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(listBody), &list))

	for _, id := range []uint{hookID, slackID} {
		code, body := env.do(t, "GET", fmt.Sprintf("/notifications/channels/%d", id), nil)
		require.Equal(t, http.StatusOK, code, body)
		assertNoLeaks(t, fmt.Sprintf("GET /notifications/channels/%d", id), body)
		assert.Contains(t, body, "********")

		var one struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &one))
		var inList map[string]any
		for _, c := range list.Data.Channels {
			if uint(c["id"].(float64)) == id {
				inList = c
			}
		}
		require.NotNil(t, inList, "channel %d is in the list", id)
		assert.Equal(t, inList, one.Data, "one channel is the object the list holds")
	}

	code, body := env.do(t, "GET", "/notifications/channels/9999", nil)
	assert.Equal(t, http.StatusNotFound, code, body)
	assertNoLeaks(t, "the 404 body", body)
	code, _ = env.do(t, "GET", "/notifications/channels/abc", nil)
	assert.Equal(t, http.StatusBadRequest, code)
}

// DELETE a rule removes the rule and its delivery history in one go and keeps the channel. With
// constraints on (test mode unset), a bare delete of a rule that history points at fails: the control.
func TestNotificationContract_DeleteRule(t *testing.T) {
	t.Setenv("SHELLY_SECURITY_VALIDATION_TEST_MODE", "")
	env := setupNotificationSecrets(t)
	chID, _ := env.createChannel(t, plainWebhook("hook", true))
	ruleID := env.createRule(t, map[string]any{"name": "r1", "enabled": true, "channel_id": chID, "alert_level": "all"})
	otherID := env.createRule(t, map[string]any{"name": "r2", "enabled": true, "channel_id": chID, "alert_level": "all"})

	for _, rid := range []uint{ruleID, otherID} {
		require.NoError(t, env.db.Create(&notification.NotificationHistory{
			RuleID: rid, ChannelID: chID, TriggerType: "test", Subject: "s", Message: "m", Status: "sent",
		}).Error)
	}

	// The control: the constraint is on, so the history row blocks a bare delete.
	err := env.db.Delete(&notification.NotificationRule{}, ruleID).Error
	require.Error(t, err, "a bare delete of a rule with history must fail on the foreign key")
	var stillThere int64
	require.NoError(t, env.db.Model(&notification.NotificationRule{}).Where("id = ?", ruleID).Count(&stillThere).Error)
	require.EqualValues(t, 1, stillThere)

	code, body := env.do(t, "DELETE", fmt.Sprintf("/notifications/rules/%d", ruleID), nil)
	require.Equal(t, http.StatusOK, code, body)
	assertNoLeaks(t, "DELETE rule", body)

	var rules, history, otherHistory, channels int64
	require.NoError(t, env.db.Model(&notification.NotificationRule{}).Where("id = ?", ruleID).Count(&rules).Error)
	require.NoError(t, env.db.Model(&notification.NotificationHistory{}).Where("rule_id = ?", ruleID).Count(&history).Error)
	require.NoError(t, env.db.Model(&notification.NotificationHistory{}).Where("rule_id = ?", otherID).Count(&otherHistory).Error)
	require.NoError(t, env.db.Model(&notification.NotificationChannel{}).Where("id = ?", chID).Count(&channels).Error)
	assert.EqualValues(t, 0, rules, "the rule is gone")
	assert.EqualValues(t, 0, history, "its history is gone")
	assert.EqualValues(t, 1, otherHistory, "another rule's history stays")
	assert.EqualValues(t, 1, channels, "the channel stays")

	code, _ = env.do(t, "DELETE", fmt.Sprintf("/notifications/rules/%d", ruleID), nil)
	assert.Equal(t, http.StatusNotFound, code, "a second delete is a 404")
	code, _ = env.do(t, "DELETE", "/notifications/rules/9999", nil)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = env.do(t, "DELETE", "/notifications/rules/abc", nil)
	assert.Equal(t, http.StatusBadRequest, code)
}
