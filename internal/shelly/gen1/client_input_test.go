package gen1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGen1Client_InputSettingsRequestShape(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"name":null,"btn_type":"toggle","btn_reverse":1}`))
	}))
	defer server.Close()

	client := NewClient(strings.TrimPrefix(server.URL, "http://"))
	ctx := context.Background()

	err := client.SetInputSettings(ctx, 2, map[string]interface{}{
		"btn_type":    "toggle",
		"btn_reverse": 1,
	})
	if err != nil {
		t.Fatalf("SetInputSettings: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/settings/input/2" {
		t.Errorf("write went to %s %s, want POST /settings/input/2", gotMethod, gotPath)
	}
	if !strings.HasPrefix(gotType, "application/x-www-form-urlencoded") {
		t.Errorf("write content type %q, want a flat form", gotType)
	}
	if gotBody != "btn_reverse=1&btn_type=toggle" {
		t.Errorf("write body %q, want flat form parameters only", gotBody)
	}

	live, err := client.GetInputSettings(ctx, 2)
	if err != nil {
		t.Fatalf("GetInputSettings: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/settings/input/2" {
		t.Errorf("read went to %s %s, want GET /settings/input/2", gotMethod, gotPath)
	}
	if live["btn_type"] != "toggle" || live["btn_reverse"] != float64(1) {
		t.Errorf("read-back %v does not carry the device answer", live)
	}
}
