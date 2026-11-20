package notification

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/redact"
)

const (
	urlRule    = "must be an absolute http or https URL with a host"
	methodRule = "webhook method must be one of POST, PUT, PATCH"
)

func webhookJSON(url, method string) string {
	cfg := map[string]any{"url": url}
	if method != "" {
		cfg["method"] = method
	}
	b, _ := json.Marshal(cfg)
	return string(b)
}

func slackJSON(url string) string {
	b, _ := json.Marshal(map[string]any{"webhook_url": url, "channel": "#ops"})
	return string(b)
}

func newChannel(name, typ, config string) *NotificationChannel {
	return &NotificationChannel{Name: name, Type: typ, Enabled: true, Config: json.RawMessage(config)}
}

// seed writes a channel straight into the database, past the service: it stands for a channel stored
// before the rules existed.
func seed(t *testing.T, db *gorm.DB, name, typ, config string) uint {
	t.Helper()
	ch := newChannel(name, typ, config)
	require.NoError(t, db.Create(ch).Error)
	return ch.ID
}

func storedChannel(t *testing.T, db *gorm.DB, id uint) (NotificationChannel, map[string]any) {
	t.Helper()
	var ch NotificationChannel
	require.NoError(t, db.First(&ch, id).Error)
	var cfg map[string]any
	require.NoError(t, json.Unmarshal(ch.Config, &cfg))
	return ch, cfg
}

var badMethods = map[string]string{
	"not a method":   "NOT A METHOD",
	"lower case":     "post",
	"get":            "GET",
	"delete":         "DELETE",
	"trailing space": "POST ",
}

var badURLs = map[string]string{
	"file scheme":         "file:///etc/passwd",
	"no scheme":           "hook.example/x",
	"scheme-relative":     "//h.example/x",
	"ftp":                 "ftp://h.example/x",
	"javascript":          "javascript:alert(1)",
	"no host":             "http:///x",
	"scheme only":         "https://",
	"opaque":              "http:x",
	"broken IPv6 host":    "http://[::1",
	"leading space":       " https://h.example/x",
	"port without a host": "http://:8080/x",
}

func TestChannelValidation_CreateRefusals(t *testing.T) {
	svc, _, cleanup := setupSimpleTestService(t)
	defer cleanup()

	for name, m := range badMethods {
		t.Run("webhook method "+name, func(t *testing.T) {
			err := svc.CreateChannel(newChannel("m-"+name, "webhook", webhookJSON("https://h.example/x", m)))
			require.Error(t, err)
			assert.Contains(t, err.Error(), methodRule)
		})
	}
	for name, u := range badURLs {
		t.Run("webhook url "+name, func(t *testing.T) {
			err := svc.CreateChannel(newChannel("w-"+name, "webhook", webhookJSON(u, "")))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "webhook URL "+urlRule)
		})
		t.Run("slack url "+name, func(t *testing.T) {
			err := svc.CreateChannel(newChannel("s-"+name, "slack", slackJSON(u)))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "slack webhook URL "+urlRule)
		})
	}
	t.Run("member names fold as the decoder folds them", func(t *testing.T) {
		err := svc.CreateChannel(newChannel("folded", "webhook", `{"URL":"ftp://h.example","Method":"GET"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), urlRule)
	})
}

func TestChannelValidation_Accepted(t *testing.T) {
	svc, _, cleanup := setupSimpleTestService(t)
	defer cleanup()

	good := map[string]*NotificationChannel{
		"empty method is POST": newChannel("a1", "webhook", webhookJSON("https://h.example/x", "")),
		"POST":                 newChannel("a2", "webhook", webhookJSON("https://h.example/x", "POST")),
		"PUT":                  newChannel("a3", "webhook", webhookJSON("http://h.example/x", "PUT")),
		"PATCH":                newChannel("a4", "webhook", webhookJSON("https://h.example/x", "PATCH")),
		"upper case scheme":    newChannel("a5", "webhook", webhookJSON("HTTP://H.example:8080/x?t=1", "")),
		"userinfo":             newChannel("a6", "webhook", webhookJSON("https://u:p@h.example/x", "")),
		"IPv6 host":            newChannel("a7", "webhook", webhookJSON("http://[::1]:8080/x", "")),
		"slack":                newChannel("a8", "slack", slackJSON("https://hooks.slack.com/services/T/B/X")),
	}
	for name, ch := range good {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, svc.CreateChannel(ch))
		})
	}
}

func TestChannelValidation_UpdateRefusals(t *testing.T) {
	svc, db, cleanup := setupSimpleTestService(t)
	defer cleanup()

	wid := seed(t, db, "hook", "webhook", webhookJSON("https://h.example/x", "POST"))
	sid := seed(t, db, "slack", "slack", slackJSON("https://hooks.slack.com/services/T/B/X"))
	_, wBefore := storedChannel(t, db, wid)
	_, sBefore := storedChannel(t, db, sid)

	for name, m := range badMethods {
		t.Run("webhook method "+name, func(t *testing.T) {
			err := svc.UpdateChannel(wid, newChannel("hook", "webhook", webhookJSON("https://h.example/x", m)))
			require.Error(t, err)
			assert.Contains(t, err.Error(), methodRule)
		})
	}
	for name, u := range badURLs {
		t.Run("webhook url "+name, func(t *testing.T) {
			err := svc.UpdateChannel(wid, newChannel("hook", "webhook", webhookJSON(u, "")))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "webhook URL "+urlRule)
		})
		t.Run("slack url "+name, func(t *testing.T) {
			err := svc.UpdateChannel(sid, newChannel("slack", "slack", slackJSON(u)))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "slack webhook URL "+urlRule)
		})
	}

	_, wAfter := storedChannel(t, db, wid)
	_, sAfter := storedChannel(t, db, sid)
	assert.Equal(t, wBefore, wAfter, "a refused update leaves the stored config alone")
	assert.Equal(t, sBefore, sAfter)
}

// The client sends the masked url back as the placeholder, and the stored value is what is checked.
// Validating the placeholder itself would refuse every edit that keeps the URL.
func TestChannelValidation_KeptURLIsCheckedAsStored(t *testing.T) {
	svc, db, cleanup := setupSimpleTestService(t)
	defer cleanup()
	ph := redact.Placeholder

	t.Run("a kept webhook url passes and stays", func(t *testing.T) {
		id := seed(t, db, "kept", "webhook", webhookJSON("https://h.example/kept?t=tok", "POST"))
		in := `{"url":"` + ph + `","method":"PUT"}`
		require.NoError(t, svc.UpdateChannel(id, newChannel("kept", "webhook", in)))
		_, cfg := storedChannel(t, db, id)
		assert.Equal(t, "https://h.example/kept?t=tok", cfg["url"])
		assert.Equal(t, "PUT", cfg["method"])
	})
	t.Run("a kept slack url passes and stays", func(t *testing.T) {
		id := seed(t, db, "kept-slack", "slack", slackJSON("https://hooks.slack.com/services/T/B/KEPT"))
		in := `{"webhook_url":"` + ph + `","channel":"#new"}`
		require.NoError(t, svc.UpdateChannel(id, newChannel("kept-slack", "slack", in)))
		_, cfg := storedChannel(t, db, id)
		assert.Equal(t, "https://hooks.slack.com/services/T/B/KEPT", cfg["webhook_url"])
		assert.Equal(t, "#new", cfg["channel"])
	})
	t.Run("a bad method is still refused beside a kept url", func(t *testing.T) {
		id := seed(t, db, "kept-badmethod", "webhook", webhookJSON("https://h.example/kept2", "POST"))
		err := svc.UpdateChannel(id, newChannel("kept-badmethod", "webhook", `{"url":"`+ph+`","method":"GET"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), methodRule)
	})
}

// A channel stored before the rules existed. A save that leaves its config as it is passes, so the channel
// can still be switched off or renamed; a save that changes the config is judged in full.
func TestChannelValidation_StoredBadChannel(t *testing.T) {
	svc, db, cleanup := setupSimpleTestService(t)
	defer cleanup()
	ph := redact.Placeholder

	t.Run("disabled with the config sent back as read", func(t *testing.T) {
		id := seed(t, db, "bad-url", "webhook", webhookJSON("ftp://h.example/x?t=tok", "POST"))
		_, before := storedChannel(t, db, id)
		up := newChannel("bad-url", "webhook", `{"url":"`+ph+`","method":"POST"}`)
		up.Enabled = false
		require.NoError(t, svc.UpdateChannel(id, up))
		ch, after := storedChannel(t, db, id)
		assert.False(t, ch.Enabled)
		assert.Equal(t, before, after, "the config is as it was")
	})
	t.Run("renamed with the config unchanged", func(t *testing.T) {
		id := seed(t, db, "bad-method", "webhook", webhookJSON("https://h.example/x", "GET"))
		up := newChannel("renamed", "webhook", webhookJSON("https://h.example/x", "GET"))
		up.Description = "still failing"
		require.NoError(t, svc.UpdateChannel(id, up))
		ch, _ := storedChannel(t, db, id)
		assert.Equal(t, "renamed", ch.Name)
	})
	t.Run("a slack channel with a bad url can be disabled", func(t *testing.T) {
		id := seed(t, db, "bad-slack", "slack", slackJSON("not a url"))
		up := newChannel("bad-slack", "slack", `{"webhook_url":"`+ph+`","channel":"#ops"}`)
		up.Enabled = false
		require.NoError(t, svc.UpdateChannel(id, up))
	})
	t.Run("a change to the config that leaves the url bad is refused", func(t *testing.T) {
		id := seed(t, db, "bad-url-2", "webhook", webhookJSON("ftp://h.example/x", "POST"))
		err := svc.UpdateChannel(id, newChannel("bad-url-2", "webhook", `{"url":"`+ph+`","method":"PUT"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "webhook URL "+urlRule)
		_, cfg := storedChannel(t, db, id)
		assert.Equal(t, "POST", cfg["method"], "the row is unchanged")
	})
	t.Run("a change that leaves the method bad is refused", func(t *testing.T) {
		id := seed(t, db, "bad-method-2", "webhook", webhookJSON("https://h.example/x", "GET"))
		err := svc.UpdateChannel(id, newChannel("bad-method-2", "webhook", `{"url":"`+ph+`","method":"GET","timeout":5}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), methodRule)
	})
	t.Run("a fixed method passes with the kept url", func(t *testing.T) {
		id := seed(t, db, "bad-method-3", "webhook", webhookJSON("https://h.example/x", "GET"))
		require.NoError(t, svc.UpdateChannel(id, newChannel("bad-method-3", "webhook", `{"url":"`+ph+`","method":"POST"}`)))
	})
	t.Run("a change of type is judged in full", func(t *testing.T) {
		id := seed(t, db, "bad-type", "webhook", webhookJSON("ftp://h.example/x", ""))
		err := svc.UpdateChannel(id, newChannel("bad-type", "slack", slackJSON("ftp://h.example/x")))
		require.Error(t, err)
		assert.Contains(t, err.Error(), urlRule)
	})
}

// A webhook URL is the token. The refusal goes to the response and the log, so it never carries the value.
func TestChannelValidation_RefusalCarriesNoValue(t *testing.T) {
	svc, _, cleanup := setupSimpleTestService(t)
	defer cleanup()

	secretURL := "ftp://user:LEAKPWD@h.example/LEAKPATH?t=LEAKQ"
	for _, ch := range []*NotificationChannel{
		newChannel("leak-webhook", "webhook", webhookJSON(secretURL, "")),
		newChannel("leak-slack", "slack", slackJSON(secretURL)),
	} {
		err := svc.CreateChannel(ch)
		require.Error(t, err)
		for _, leak := range []string{"LEAKPWD", "LEAKPATH", "LEAKQ", "h.example", "ftp"} {
			assert.False(t, strings.Contains(err.Error(), leak), "the text carries %q: %s", leak, err)
		}
	}
}
