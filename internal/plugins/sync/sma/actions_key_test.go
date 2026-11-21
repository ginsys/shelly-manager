package sma

import (
	"encoding/json"
	"strings"
	"testing"
)

// The typed key of a Gen1 action's URL list is secret_urls. A URL can hold a password or a token, so
// the SMA export must redact it: a later edit of this key rule cannot drop it silently.
func TestIsSensitiveField_CoversTheActionURLKey(t *testing.T) {
	if !(&SMAPlugin{}).isSensitiveField("secret_urls") {
		t.Fatal("the SMA export would write action URLs in clear")
	}
}

// The export's own tree walk replaces the URL list of every action, whatever the list holds.
func TestRedactSensitive_HidesActionURLLists(t *testing.T) {
	const doc = `{"actions":[` +
		`{"name":"btn_on_url","index":0,"secret_urls":["http://admin:hunter2@h/on?token=s3cr3t","http://a/2"]},` +
		`{"name":"out_on_url","index":0,"secret_urls":[]}]}`
	if !strings.Contains(doc, "hunter2") {
		t.Fatal("the fixture must hold the secret")
	}
	var tree map[string]interface{}
	if err := json.Unmarshal([]byte(doc), &tree); err != nil {
		t.Fatal(err)
	}

	redactSensitive(tree, (&SMAPlugin{}).isSensitiveField)

	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"hunter2", "s3cr3t", "http://a/2"} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("the export tree still holds %q: %s", secret, raw)
		}
	}
	if !strings.Contains(string(raw), "btn_on_url") {
		t.Errorf("the action name must stay: %s", raw)
	}
}
