package gen1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGen1Client_RelaySettingsRequestShape(t *testing.T) {
	var gotMethod, gotPath, gotType, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotType = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				t.Errorf("ParseForm: %v", err)
			}
			gotBody = r.PostForm.Encode()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"Heater","default_state":"last","auto_on":5,"auto_off":3600}`))
	}))
	defer server.Close()

	client := NewClient(strings.TrimPrefix(server.URL, "http://"))
	ctx := context.Background()

	err := client.SetRelaySettings(ctx, 1, map[string]interface{}{
		"default_state": "last",
		"auto_off":      3600,
	})
	if err != nil {
		t.Fatalf("SetRelaySettings: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/settings/relay/1" {
		t.Errorf("write went to %s %s, want POST /settings/relay/1", gotMethod, gotPath)
	}
	if !strings.HasPrefix(gotType, "application/x-www-form-urlencoded") {
		t.Errorf("write content type %q, want a flat form", gotType)
	}
	if gotBody != "auto_off=3600&default_state=last" {
		t.Errorf("write body %q, want flat form parameters only", gotBody)
	}

	live, err := client.GetRelaySettings(ctx, 1)
	if err != nil {
		t.Fatalf("GetRelaySettings: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/settings/relay/1" {
		t.Errorf("read went to %s %s, want GET /settings/relay/1", gotMethod, gotPath)
	}
	if live["auto_off"] != float64(3600) || live["default_state"] != "last" {
		t.Errorf("read-back %v does not carry the device answer", live)
	}
}
