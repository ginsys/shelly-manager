package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/database"
)

// gatedDevice is a Gen1 device that accepts only some logins. A request from any other login
// gets 401. The first 401 runs onReject, which lets a test land a credential update at the
// moment a request is rejected.
type gatedDevice struct {
	mu       sync.Mutex
	accept   map[string]bool
	gateGET  bool
	gatePOST bool
	onReject func()
	once     sync.Once
	users    []string
}

func (g *gatedDevice) handler(w http.ResponseWriter, r *http.Request) {
	user, _, _ := r.BasicAuth()
	gated := (r.Method == http.MethodGet && g.gateGET) || (r.Method == http.MethodPost && g.gatePOST)
	g.mu.Lock()
	allowed := g.accept[user]
	g.users = append(g.users, user)
	hook := g.onReject
	g.mu.Unlock()

	if gated && !allowed {
		if hook != nil {
			g.once.Do(hook)
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodPost {
		_, _ = fmt.Fprint(w, `{"ison":true}`)
		return
	}
	_, _ = fmt.Fprint(w, `{"type":"SHPLG-S","mac":"DCAA2E000000","auth":true,"fw":"1.14.0"}`)
}

func (g *gatedDevice) setReject(f func()) {
	g.mu.Lock()
	g.onReject = f
	g.mu.Unlock()
}

func (g *gatedDevice) last() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.users) == 0 {
		return "(no request)"
	}
	return g.users[len(g.users)-1]
}

// retryFixture is a service with one Gen1 device that stores the login old/oldpw.
func retryFixture(t *testing.T, gate *gatedDevice) (*ShellyService, *database.Device) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(gate.handler))
	t.Cleanup(server.Close)

	db := createTestDB(t)
	service := NewService(db, createTestConfig())
	device := createTestDevice(t, db, server.URL[len("http://"):])
	device.Settings = `{"gen":1,"auth_enabled":true,"auth_user":"old","auth_pass":"oldpw","note":"keep"}`
	if err := db.UpdateDevice(device); err != nil {
		t.Fatal(err)
	}
	return service, device
}

// storedLogin reads the login that the database holds for the device.
func storedLogin(t *testing.T, service *ShellyService, id uint) (user string, settings map[string]any) {
	t.Helper()
	stored, err := service.DB.GetDevice(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(stored.Settings), &settings); err != nil {
		t.Fatalf("stored settings are not JSON: %v", err)
	}
	user, _ = settings["auth_user"].(string)
	return user, settings
}

// The device rejects the old login. While it does, a credential update stores a new login. The
// 401 path must see that the stored login is no longer the rejected one and write nothing.
func TestGetClient_401DoesNotEraseALoginSavedDuringTheRead(t *testing.T) {
	gate := &gatedDevice{accept: map[string]bool{"new": true}, gateGET: true}
	service, device := retryFixture(t, gate)
	gate.setReject(func() {
		if err := service.UpdateDeviceAuth(device.ID, "new", "newpw"); err != nil {
			t.Errorf("UpdateDeviceAuth failed: %v", err)
		}
	})

	if _, err := service.getClientWithRetry(device, true); err != nil {
		t.Fatalf("getClientWithRetry failed: %v", err)
	}

	if user, _ := storedLogin(t, service, device.ID); user != "new" {
		t.Errorf("the stored login is %q after the 401 path ran, want new", user)
	}
	if got := gate.last(); got != "new" {
		t.Errorf("the retry sent login %q, want new", got)
	}
}

// The control: nothing updates the login, so the 401 path clears the rejected one and keeps the
// other settings.
func TestGetClient_401ClearsALoginThatIsStillTheRejectedOne(t *testing.T) {
	gate := &gatedDevice{accept: map[string]bool{"": true}, gateGET: true}
	service, device := retryFixture(t, gate)

	if _, err := service.getClientWithRetry(device, true); err != nil {
		t.Fatalf("getClientWithRetry failed: %v", err)
	}

	user, settings := storedLogin(t, service, device.ID)
	if user != "" {
		t.Errorf("the stored login is %q, want it cleared", user)
	}
	if settings["note"] != "keep" {
		t.Errorf("the clear dropped other settings: %v", settings)
	}
	if got := gate.last(); got != "" {
		t.Errorf("the retry sent login %q, want none", got)
	}
}

// The same interleave on the control path: the action is rejected, a credential update lands,
// and the retry must not erase the new login.
func TestControlDevice_401DoesNotEraseALoginSavedDuringTheAction(t *testing.T) {
	gate := &gatedDevice{accept: map[string]bool{"new": true}, gatePOST: true}
	service, device := retryFixture(t, gate)
	gate.setReject(func() {
		if err := service.UpdateDeviceAuth(device.ID, "new", "newpw"); err != nil {
			t.Errorf("UpdateDeviceAuth failed: %v", err)
		}
	})

	if err := service.ControlDevice(device.ID, "on", nil); err != nil {
		t.Fatalf("ControlDevice failed: %v", err)
	}

	if user, _ := storedLogin(t, service, device.ID); user != "new" {
		t.Errorf("the stored login is %q after the retry, want new", user)
	}
	if got := gate.last(); got != "new" {
		t.Errorf("the retried action sent login %q, want new", got)
	}
}

// The control for the control path: the rejected login is still the stored one, so it is cleared.
func TestControlDevice_401ClearsALoginThatIsStillTheRejectedOne(t *testing.T) {
	gate := &gatedDevice{accept: map[string]bool{"": true}, gatePOST: true}
	service, device := retryFixture(t, gate)

	if err := service.ControlDevice(device.ID, "on", nil); err != nil {
		t.Fatalf("ControlDevice failed: %v", err)
	}

	user, settings := storedLogin(t, service, device.ID)
	if user != "" {
		t.Errorf("the stored login is %q, want it cleared", user)
	}
	if settings["note"] != "keep" {
		t.Errorf("the clear dropped other settings: %v", settings)
	}
}

// Both 401 paths and UpdateDeviceAuth run together. The device accepts every login that an
// update stores, so nothing may ever clear one. Run under -race. The deadline turns a deadlock
// into a failure instead of a hung Job.
func TestAuthRetryPaths_ConcurrentUpdatesNeitherDeadlockNorLoseALogin(t *testing.T) {
	gate := &gatedDevice{accept: map[string]bool{"a": true, "b": true}, gateGET: true, gatePOST: true}
	service, device := retryFixture(t, gate)

	done := make(chan struct{})
	var wg sync.WaitGroup
	lastUpdate := ""
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			user := "a"
			if i%2 == 1 {
				user = "b"
			}
			if err := service.UpdateDeviceAuth(device.ID, user, user+"pw"); err != nil {
				t.Errorf("UpdateDeviceAuth failed: %v", err)
				return
			}
			lastUpdate = user
		}
	}()
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			stale := *device // each reader keeps the snapshot it loaded first
			for i := 0; i < 15; i++ {
				snapshot := stale
				if r%2 == 0 {
					_, _ = service.getClientWithRetry(&snapshot, true)
				} else {
					_ = service.ControlDevice(device.ID, "on", nil)
				}
			}
		}(r)
	}
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("readers and updates did not finish in 60 s: deadlock")
	}

	if user, _ := storedLogin(t, service, device.ID); user != lastUpdate {
		t.Errorf("the stored login is %q, want the last update %q", user, lastUpdate)
	}
}
