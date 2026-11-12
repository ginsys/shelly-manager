package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// A login applied to a device is stored through UpdateDeviceAuth, and the client built afterwards
// sends the new credentials. The device here answers 401 to anything else.
func TestLoginHooks_StoreThenReconnectUsesTheNewCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "s3cret-pw-9" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"device":{"type":"SHPLG-S"}}`))
	}))
	defer server.Close()

	db := createTestDB(t)
	service := NewService(db, createTestConfig())
	device := createTestDevice(t, db, server.URL[len("http://"):])
	hooks := loginHooks{service}
	change := configuration.LoginChange{Enabled: true, Username: "admin", Password: "s3cret-pw-9"}

	if err := hooks.Store(context.Background(), device.ID, change); err != nil {
		t.Fatalf("Store: %v", err)
	}
	client, err := hooks.Client(context.Background(), device.ID)
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	if _, err := client.GetConfig(context.Background()); err != nil {
		t.Fatalf("the rebuilt client must reach the device with the new login: %v", err)
	}
}

func TestLoginHooks_DisabledLoginStoresNoCredentials(t *testing.T) {
	db := createTestDB(t)
	service := NewService(db, createTestConfig())
	device := createTestDevice(t, db, "192.0.2.12:80")
	if err := service.UpdateDeviceAuth(device.ID, "admin", "pw"); err != nil {
		t.Fatal(err)
	}

	if err := (loginHooks{service}).Store(context.Background(), device.ID, configuration.LoginChange{Enabled: false}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	updated, err := db.GetDevice(device.ID)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(updated.Settings), &settings); err != nil {
		t.Fatal(err)
	}
	if settings["auth_enabled"] != false || settings["auth_user"] != nil || settings["auth_pass"] != nil {
		t.Errorf("a disabled login must leave no stored credentials, got %v", settings)
	}
}
