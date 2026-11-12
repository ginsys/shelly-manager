package service

import (
	"testing"

	"github.com/ginsys/shelly-manager/internal/database"
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
func loginGoesAToBToA(t *testing.T, service *ShellyService, id uint) func() {
	return func() {
		if err := service.UpdateDeviceAuth(id, "bee", "beepw"); err != nil {
			t.Errorf("UpdateDeviceAuth to B failed: %v", err)
		}
		if err := service.UpdateDeviceAuth(id, "old", "oldpw"); err != nil {
			t.Errorf("UpdateDeviceAuth back to A failed: %v", err)
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

// A cached client carries login B. The caller's record holds login A, and a cache hit does not
// refresh it. The device rejects B. During the request the saved login goes A to B to A. The
// clear must compare the stored login with B, the login that the device rejected, and keep A.
// A compare against the caller's record reads A against A and clears a login that nobody
// rejected (ginsys/shelly-manager#487).
func TestControlDevice_401TakesTheRejectedLoginFromTheCachedClient(t *testing.T) {
	gate := &gatedDevice{accept: map[string]bool{"old:oldpw": true}, byPair: true, gatePOST: true}
	service, device := retryFixture(t, gate)
	cacheClientWithOtherLogin(t, service, device)
	gate.setReject(loginGoesAToBToA(t, service, device.ID))

	if err := service.ControlDevice(device.ID, "on", nil); err != nil {
		t.Fatalf("ControlDevice failed: %v", err)
	}
	requireLoginAKept(t, service, device.ID, gate)
}
