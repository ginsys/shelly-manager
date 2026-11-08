package configuration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// The configuration builder must reach a device with credentials stored in the legacy shape.
func TestCreateClientForDevice_UsesLegacyDeviceCredentials(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		mu.Lock()
		seen = append(seen, user+":"+pass)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"type":"SHPLG-S","mac":"DCAA2E000000","auth":true,"fw":"1.14.0"}`)
	}))
	defer server.Close()

	service, db := setupTestService(t)
	device := &Device{
		ID:       1,
		Name:     "plug",
		Type:     "SHPLG-S",
		IP:       strings.TrimPrefix(server.URL, "http://"),
		MAC:      "DC:AA:2E:00:00:00",
		Settings: `{"gen":1,"auth":{"username":"legacy","password":"legacypw"}}`,
	}
	require.NoError(t, db.Create(device).Error)

	client, err := service.createClientForDevice(device.ID)
	require.NoError(t, err)
	_, _ = client.GetInfo(context.Background())

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, seen, "the client sent no request to the device")
	for _, got := range seen {
		require.Equal(t, "legacy:legacypw", got)
	}
}

func TestLegacyDeviceAuth(t *testing.T) {
	cases := []struct {
		name, settings, user, pass string
		ok                         bool
	}{
		{"legacy pair", `{"gen":1,"auth":{"username":"admin","password":"pw"}}`, "admin", "pw", true},
		{"legacy pair beside empty flat keys", `{"auth_user":"","auth_pass":"","auth":{"username":"admin","password":"pw"}}`, "admin", "pw", true},
		{"flat keys win", `{"auth_user":"root","auth_pass":"x","auth":{"username":"admin","password":"pw"}}`, "", "", false},
		{"discovery boolean", `{"auth":false}`, "", "", false},
		{"incomplete legacy pair", `{"auth":{"username":"admin"}}`, "", "", false},
		{"no auth", `{"gen":2}`, "", "", false},
		{"empty", ``, "", "", false},
		{"invalid json", `{`, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user, pass, ok := LegacyDeviceAuth(tc.settings)
			if user != tc.user || pass != tc.pass || ok != tc.ok {
				t.Errorf("got (%q, %q, %v), want (%q, %q, %v)", user, pass, ok, tc.user, tc.pass, tc.ok)
			}
		})
	}
}
