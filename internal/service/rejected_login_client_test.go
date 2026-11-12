package service

import (
	"testing"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
)

// cacheClientWithOtherLogin caches a client that carries login B (bee/beepw) while the stored
// login stays A (old/oldpw). A writer that takes no clientMu does that: it puts login A back
// without dropping the cache. The device accepts only A.
func cacheClientWithOtherLogin(t *testing.T, service *ShellyService, device *database.Device) {
	t.Helper()
	saved := device.Settings
	if err := service.UpdateDeviceAuth(device.ID, "bee", "beepw"); err != nil {
		t.Fatalf("UpdateDeviceAuth failed: %v", err)
	}
	reloaded, err := service.DB.GetDevice(device.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.getClientWithRetry(reloaded, true); err != nil {
		t.Fatalf("getClientWithRetry failed: %v", err)
	}
	restore, err := service.DB.GetDevice(device.ID)
	if err != nil {
		t.Fatal(err)
	}
	restore.Settings = saved
	if err := service.DB.UpdateDevice(restore); err != nil {
		t.Fatal(err)
	}
}

// loginGoesAToBToA moves the saved login from A to B and back to A, as a request is rejected.
// It writes the record directly, as a writer that takes no clientMu does. UpdateDeviceAuth would
// drop the cached client, and then the test would not read the login from the cache entry.
func loginGoesAToBToA(t *testing.T, service *ShellyService, id uint, saved string) func() {
	return func() {
		for _, settings := range []string{
			`{"gen":1,"auth_enabled":true,"auth_user":"bee","auth_pass":"beepw","note":"keep"}`,
			saved,
		} {
			stored, err := service.DB.GetDevice(id)
			if err != nil {
				t.Errorf("GetDevice failed: %v", err)
				return
			}
			stored.Settings = settings
			if err := service.DB.UpdateDevice(stored); err != nil {
				t.Errorf("UpdateDevice failed: %v", err)
			}
		}
	}
}

func requireLoginAKept(t *testing.T, service *ShellyService, id uint, gate *gatedDevice) {
	t.Helper()
	user, settings := storedLogin(t, service, id)
	if user != "old" || settings["auth_pass"] != "oldpw" {
		t.Errorf("the stored login is %q/%v after the call, want old/oldpw", user, settings["auth_pass"])
	}
	if got := gate.last(); got != "old:oldpw" {
		t.Errorf("the retry sent login %q, want old:oldpw", got)
	}
}

// The caller's record holds login A. The cached client was built without a saved login. A writer
// that takes no clientMu stores login B and leaves that client cached, and the device now
// accepts B only. The client's rejection must still start the retry: it clears nothing, keeps
// B and rebuilds from the stored record. A gate that reads only the cached client's login
// would return the auth error.
func TestGetClientWithAuthRetry_401RetriesWhenTheCachedClientHadNoSavedLogin(t *testing.T) {
	gate := &gatedDevice{accept: map[string]bool{":": true}, byPair: true, gateGET: true}
	service, device := retryFixture(t, gate)

	// Cache a client without a login. The first request that the device serves flips the device
	// to B only and stores B, as a lock-free writer does.
	service.clientMu.Lock()
	service.storeClientLocked(device, gen1.NewClient(device.IP), "", "")
	service.clientMu.Unlock()
	gate.mu.Lock()
	gate.onServe = func() {
		gate.mu.Lock()
		gate.accept = map[string]bool{"bee:beepw": true}
		gate.mu.Unlock()
		stored, err := service.DB.GetDevice(device.ID)
		if err != nil {
			t.Errorf("GetDevice failed: %v", err)
			return
		}
		stored.Settings = `{"gen":1,"auth_enabled":true,"auth_user":"bee","auth_pass":"beepw","note":"keep"}`
		if err := service.DB.UpdateDevice(stored); err != nil {
			t.Errorf("UpdateDevice failed: %v", err)
		}
	}
	gate.mu.Unlock()

	if _, err := service.getClientWithAuthRetry(device); err != nil {
		t.Fatalf("getClientWithAuthRetry failed: %v", err)
	}
	user, settings := storedLogin(t, service, device.ID)
	if user != "bee" || settings["auth_pass"] != "beepw" {
		t.Errorf("the stored login is %q/%v, want bee/beepw", user, settings["auth_pass"])
	}
	if got := gate.last(); got != "bee:beepw" {
		t.Errorf("the retry sent login %q, want bee:beepw", got)
	}
}

// A cached client carries login B. The caller's record holds login A, and a cache hit does not
// refresh it. The device rejects B. During the request the saved login goes A to B to A. The
// clear must compare the stored login with B, the login that the device rejected, and keep A.
// A compare against the caller's record reads A against A and clears a login that nobody
// rejected (ginsys/shelly-manager#487).
func TestControlDevice_401TakesTheRejectedLoginFromTheCachedClient(t *testing.T) {
	gate := &gatedDevice{accept: map[string]bool{"old:oldpw": true}, byPair: true, gatePOST: true}
	service, device := retryFixture(t, gate)
	saved := device.Settings
	cacheClientWithOtherLogin(t, service, device)
	gate.setReject(loginGoesAToBToA(t, service, device.ID, saved))

	if err := service.ControlDevice(device.ID, "on", nil); err != nil {
		t.Fatalf("ControlDevice failed: %v", err)
	}
	requireLoginAKept(t, service, device.ID, gate)
}
