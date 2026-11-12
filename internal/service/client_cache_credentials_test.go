package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/database"
)

// loginRecorder is a Gen1 device that answers every request and notes the login of each one.
type loginRecorder struct {
	mu    sync.Mutex
	users []string
}

func (l *loginRecorder) handler(w http.ResponseWriter, r *http.Request) {
	user, _, _ := r.BasicAuth()
	l.mu.Lock()
	l.users = append(l.users, user)
	l.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprint(w, `{"type":"SHPLG-S","mac":"DCAA2E000000","auth":true,"fw":"1.14.0"}`)
}

func (l *loginRecorder) last() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.users) == 0 {
		return "(no request)"
	}
	return l.users[len(l.users)-1]
}

func credentialCacheFixture(t *testing.T) (*ShellyService, *database.Device, *loginRecorder) {
	t.Helper()
	rec := &loginRecorder{}
	server := httptest.NewServer(http.HandlerFunc(rec.handler))
	t.Cleanup(server.Close)

	db := createTestDB(t)
	service := NewService(db, createTestConfig())
	device := createTestDevice(t, db, server.URL[len("http://"):])
	device.Settings = `{"gen":1,"auth_enabled":true,"auth_user":"old","auth_pass":"oldpw"}`
	if err := db.UpdateDevice(device); err != nil {
		t.Fatal(err)
	}
	return service, device, rec
}

// useLogin sends one request through the client and returns the login the device saw.
func useLogin(t *testing.T, service *ShellyService, device *database.Device, rec *loginRecorder) string {
	t.Helper()
	client, err := service.getClientWithRetry(device, false)
	if err != nil {
		t.Fatalf("getClientWithRetry failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.GetInfo(ctx); err != nil {
		t.Fatalf("GetInfo failed: %v", err)
	}
	return rec.last()
}

// A read loads the device, then a credential update lands, then the read builds its client. The
// client must carry the new login, because the cache keeps it for later reads.
func TestGetClient_ReadRacingACredentialUpdateNeverCachesTheOldLogin(t *testing.T) {
	service, device, rec := credentialCacheFixture(t)

	fired := 0
	service.beforeClientBuild = func() {
		fired++
		if err := service.UpdateDeviceAuth(device.ID, "new", "newpw"); err != nil {
			t.Errorf("UpdateDeviceAuth failed: %v", err)
		}
	}

	// device still holds the old settings: it was loaded before the update.
	if got := useLogin(t, service, device, rec); got != "new" {
		t.Errorf("the first read sent login %q, want new", got)
	}
	if fired != 1 {
		t.Fatalf("the update ran %d times inside the window, want 1", fired)
	}
	// A later read reuses the cached client. It must still carry the new login.
	service.beforeClientBuild = nil
	if got := useLogin(t, service, device, rec); got != "new" {
		t.Errorf("a later read sent login %q, want new", got)
	}
}

// The control: without the update the same read sends the stored login, so the test above
// fails because of the interleave and not because of how the device is set up.
func TestGetClient_WithoutAnUpdateUsesTheStoredLogin(t *testing.T) {
	service, device, rec := credentialCacheFixture(t)
	if got := useLogin(t, service, device, rec); got != "old" {
		t.Errorf("the read sent login %q, want old", got)
	}
}

// The order in which a read publishes its client first and the update comes after: the update
// clears that client, and the next read builds a new one with the new login.
func TestGetClient_UpdateAfterThePublishRemovesTheOldClient(t *testing.T) {
	service, device, rec := credentialCacheFixture(t)
	if got := useLogin(t, service, device, rec); got != "old" {
		t.Fatalf("the first read sent login %q, want old", got)
	}
	if err := service.UpdateDeviceAuth(device.ID, "new", "newpw"); err != nil {
		t.Fatal(err)
	}
	if got := useLogin(t, service, device, rec); got != "new" {
		t.Errorf("the read after the update sent login %q, want new", got)
	}
}

// The update lands after the read reloaded the device and built its client, and before the
// client is cached. The read holds the cache lock, so the update waits at its cache clear, and the
// clear removes the client that the read cached from the old login.
func TestGetClient_UpdateBetweenReloadAndPublishRemovesTheOldClient(t *testing.T) {
	service, device, rec := credentialCacheFixture(t)

	updated := make(chan error, 1)
	service.beforeClientPublish = func() {
		go func() { updated <- service.UpdateDeviceAuth(device.ID, "new", "newpw") }()
		// Wait until the update has written the database. Its cache clear then waits for the
		// lock that this read holds.
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if stored, err := service.DB.GetDevice(device.ID); err == nil && strings.Contains(stored.Settings, `"new"`) {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Error("the update did not write the database in time")
	}

	if got := useLogin(t, service, device, rec); got != "old" {
		t.Fatalf("the read sent login %q, want old: it reloaded before the update", got)
	}
	service.beforeClientPublish = nil
	if err := <-updated; err != nil {
		t.Fatalf("UpdateDeviceAuth failed: %v", err)
	}

	service.clientMu.RLock()
	cached := len(service.clients)
	service.clientMu.RUnlock()
	if cached != 0 {
		t.Errorf("%d client(s) stay cached after the update, want 0", cached)
	}
	if got := useLogin(t, service, device, rec); got != "new" {
		t.Errorf("the next read sent login %q, want new", got)
	}
}

// failingReload is a database whose device read fails on demand.
type failingReload struct {
	database.DatabaseInterface
	fail bool
}

func (f *failingReload) GetDevice(id uint) (*database.Device, error) {
	if f.fail {
		return nil, errors.New("database unavailable")
	}
	return f.DatabaseInterface.GetDevice(id)
}

// If the reload fails, the read cannot know which login is current, so it returns the error and
// caches nothing.
func TestGetClient_FailedReloadCachesNothing(t *testing.T) {
	service, device, _ := credentialCacheFixture(t)
	db := &failingReload{DatabaseInterface: service.DB, fail: true}
	service.DB = db

	if _, err := service.getClientWithRetry(device, false); err == nil {
		t.Fatal("getClientWithRetry succeeded although the reload failed")
	}
	service.clientMu.RLock()
	cached := len(service.clients)
	service.clientMu.RUnlock()
	if cached != 0 {
		t.Errorf("%d client(s) cached after a failed reload, want 0", cached)
	}

	db.fail = false
	if _, err := service.getClientWithRetry(device, false); err != nil {
		t.Errorf("the read after the database came back failed: %v", err)
	}
}
