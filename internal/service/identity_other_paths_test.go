package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
)

// The record stores the MAC in the colon spelling a record may hold; a device reports 12 hex digits.
const (
	identityStoredMAC   = "68:C6:3A:12:34:56"
	identityReportedMAC = "68C63A123456"
	identityOtherMAC    = "DCAA2E000000"
)

// identityDevice is a Gen1 device at a local address. Like a real one it serves GET /shelly without
// a login; every other path needs the login it accepts (none when login is empty). strictInfo makes
// it also refuse a wrong login sent to /shelly, which a documented device does not do: the older
// fixtures reject at that path, and the clear sites are only reached there.
type identityDevice struct {
	mu         sync.Mutex
	mac        string
	login      string // "user:password"
	strictInfo bool
	writes     int // POST requests served
	reads      int // GET requests served on a path other than /shelly
}

func (d *identityDevice) handler(w http.ResponseWriter, r *http.Request) {
	user, pass, hasLogin := r.BasicAuth()
	accepted := d.login == "" || (hasLogin && user+":"+pass == d.login)
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/shelly" {
		if d.strictInfo && hasLogin && !accepted {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "SHPLG-S", "mac": d.mac, "auth": d.login != "", "fw": "1.14.0"})
		return
	}
	if !accepted {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	d.mu.Lock()
	if r.Method == http.MethodPost {
		d.writes++
	} else {
		d.reads++
	}
	d.mu.Unlock()
	if r.URL.Path == "/status" {
		_, _ = w.Write([]byte(`{"relays":[{"ison":false}]}`))
		return
	}
	_, _ = w.Write([]byte(`{"ison":true}`))
}

func (d *identityDevice) counts() (writes, reads int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.writes, d.reads
}

// identityFixture is a service with one Gen1 device record at the fake's address. storedMAC may be
// empty: a record that names no device.
func identityFixture(t *testing.T, dev *identityDevice, storedMAC, settings string) (*ShellyService, *database.Device) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(dev.handler))
	t.Cleanup(server.Close)

	db := createTestDB(t)
	service := NewService(db, createTestConfig())
	device := createTestDevice(t, db, strings.TrimPrefix(server.URL, "http://"))
	device.MAC = storedMAC
	device.Settings = settings
	if err := db.UpdateDevice(device); err != nil {
		t.Fatal(err)
	}
	return service, device
}

const identityNoLogin = `{"gen":1,"auth_enabled":false}`

// A toggle, on, off or reboot goes to whatever answers at the address, so each first reads who
// answers and refuses another device: nothing is sent to it, not even the status read of a toggle.
func TestControlDevice_RefusesADeviceItCannotNameAsTheStoredOne(t *testing.T) {
	cases := []struct {
		name      string
		storedMAC string
		reported  string
	}{
		{"another MAC", identityStoredMAC, identityOtherMAC},
		{"no stored MAC", "", identityReportedMAC},
		{"the device reports no MAC", identityStoredMAC, ""},
	}
	for _, tc := range cases {
		for _, action := range []string{"on", "off", "toggle", "reboot"} {
			t.Run(tc.name+"/"+action, func(t *testing.T) {
				dev := &identityDevice{mac: tc.reported}
				service, device := identityFixture(t, dev, tc.storedMAC, identityNoLogin)

				err := service.ControlDevice(device.ID, action, map[string]interface{}{"force": true})

				if !errors.Is(err, configuration.ErrDeviceIdentity) {
					t.Fatalf("got %v, want an error wrapping ErrDeviceIdentity", err)
				}
				if writes, reads := dev.counts(); writes != 0 || reads != 0 {
					t.Errorf("the device was sent %d writes and %d reads, want none", writes, reads)
				}
			})
		}
	}
}

// The stored device still gets its commands, in any spelling of its MAC.
func TestControlDevice_ControlsTheStoredDevice(t *testing.T) {
	for _, action := range []string{"on", "off", "toggle", "reboot"} {
		t.Run(action, func(t *testing.T) {
			dev := &identityDevice{mac: identityReportedMAC}
			service, device := identityFixture(t, dev, identityStoredMAC, identityNoLogin)

			if err := service.ControlDevice(device.ID, action, map[string]interface{}{"force": true}); err != nil {
				t.Fatalf("ControlDevice failed: %v", err)
			}
			if writes, _ := dev.counts(); writes != 1 {
				t.Errorf("the device was sent %d writes, want 1", writes)
			}
		})
	}
}

// A device that rejects the saved login is the case the clear sites meet. When it is not the stored
// device, its rejection says nothing about the stored login: the login stays and no retry runs.
// The same rejection by the stored device still clears it, as before.
func TestGetClient_AnotherDevicesRejectionClearsNoLogin(t *testing.T) {
	const saved = `{"gen":1,"auth_enabled":true,"auth_user":"old","auth_pass":"oldpw","note":"keep"}`
	sites := map[string]func(s *ShellyService, d *database.Device) (any, error){
		"getClientWithRetry": func(s *ShellyService, d *database.Device) (any, error) {
			return s.getClientWithRetry(d, true)
		},
		"getClientWithAuthRetry": func(s *ShellyService, d *database.Device) (any, error) {
			return s.getClientWithAuthRetry(d)
		},
	}
	for name, call := range sites {
		t.Run(name+"/another device", func(t *testing.T) {
			dev := &identityDevice{mac: identityOtherMAC, login: "other:otherpw", strictInfo: true}
			service, device := identityFixture(t, dev, identityStoredMAC, saved)

			_, err := call(service, device)

			if !errors.Is(err, configuration.ErrDeviceIdentity) {
				t.Fatalf("got %v, want an error wrapping ErrDeviceIdentity", err)
			}
			if user, settings := storedLogin(t, service, device.ID); user != "old" || settings["auth_pass"] != "oldpw" || settings["note"] != "keep" {
				t.Errorf("the stored settings are %v after another device's rejection, want the login kept", settings)
			}
		})
		t.Run(name+"/the stored device", func(t *testing.T) {
			dev := &identityDevice{mac: identityReportedMAC, login: "other:otherpw", strictInfo: true}
			service, device := identityFixture(t, dev, identityStoredMAC, saved)

			_, _ = call(service, device)

			if user, _ := storedLogin(t, service, device.ID); user != "" {
				t.Errorf("the stored login is %q after the stored device rejected it, want it cleared", user)
			}
		})
	}
}

// The saved-login path of the client builder: the global login works on whatever answers, and used
// to be stored in the record of the device the address belonged to. Another device's acceptance
// proves nothing about the stored device, so nothing is stored.
func TestGetClient_AnotherDeviceAcceptingTheConfigLoginIsNotSaved(t *testing.T) {
	const noSavedLogin = `{"gen":1,"auth_enabled":true}`
	for _, tc := range []struct {
		name      string
		reported  string
		wantSaved bool
	}{
		{"another device", identityOtherMAC, false},
		{"the stored device", identityReportedMAC, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dev := &identityDevice{mac: tc.reported, login: "cfg:cfgpw"}
			service, device := identityFixture(t, dev, identityStoredMAC, noSavedLogin)
			service.Config.Provisioning.AuthEnabled = true
			service.Config.Provisioning.AuthUser = "cfg"
			service.Config.Provisioning.AuthPassword = "cfgpw"

			if _, err := service.getClientWithRetry(device, true); err != nil {
				t.Fatalf("getClientWithRetry failed: %v", err)
			}

			user, _ := storedLogin(t, service, device.ID)
			if saved := user == "cfg"; saved != tc.wantSaved {
				t.Errorf("login saved = %t, want %t (stored user %q)", saved, tc.wantSaved, user)
			}
		})
	}
}

// A record with no MAC names no device, so a login is neither cleared nor saved on its word.
func TestGetClient_ARecordWithNoMACChangesNoLogin(t *testing.T) {
	t.Run("clear", func(t *testing.T) {
		const saved = `{"gen":1,"auth_enabled":true,"auth_user":"old","auth_pass":"oldpw"}`
		dev := &identityDevice{mac: identityReportedMAC, login: "other:otherpw", strictInfo: true}
		service, device := identityFixture(t, dev, "", saved)

		_, err := service.getClientWithRetry(device, true)

		if !errors.Is(err, configuration.ErrDeviceIdentity) {
			t.Fatalf("got %v, want an error wrapping ErrDeviceIdentity", err)
		}
		if user, _ := storedLogin(t, service, device.ID); user != "old" {
			t.Errorf("the stored login is %q, want old", user)
		}
	})
	t.Run("save", func(t *testing.T) {
		dev := &identityDevice{mac: identityReportedMAC, login: "cfg:cfgpw"}
		service, device := identityFixture(t, dev, "", `{"gen":1,"auth_enabled":true}`)
		service.Config.Provisioning.AuthEnabled = true
		service.Config.Provisioning.AuthUser = "cfg"
		service.Config.Provisioning.AuthPassword = "cfgpw"

		if _, err := service.getClientWithRetry(device, true); err != nil {
			t.Fatalf("getClientWithRetry failed: %v", err)
		}

		if user, _ := storedLogin(t, service, device.ID); user != "" {
			t.Errorf("the stored login is %q, want none", user)
		}
	})
}

// A read path stays a read path: status of a device that is not the stored one is still served, as
// it was, because only writes and stored state are guarded.
func TestGetDeviceStatus_StillReadsWhateverAnswers(t *testing.T) {
	dev := &identityDevice{mac: identityOtherMAC}
	service, device := identityFixture(t, dev, identityStoredMAC, identityNoLogin)

	if _, err := service.GetDeviceStatus(device.ID); err != nil {
		t.Fatalf("GetDeviceStatus failed: %v", err)
	}
	if writes, _ := dev.counts(); writes != 0 {
		t.Errorf("a read path sent %d writes", writes)
	}
}
