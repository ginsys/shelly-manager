package gen1

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// A device that answered once with an HTTP error has answered, even if the retries that follow get
// no answer: the error handed back must still say so.
func TestGen1Client_GetKeepsTheEvidenceOfAnAnsweredAttempt(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requests, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("Hijack: %v", err)
			return
		}
		_ = conn.Close() // no answer
	}))
	defer server.Close()

	client := NewClient(strings.TrimPrefix(server.URL, "http://"), WithRetry(2, time.Millisecond))
	_, err := client.GetConfig(context.Background())

	if err == nil {
		t.Fatal("GetConfig: expected an error")
	}
	// The control: the retries ran. The HTTP transport may add one resend of its own.
	if got := atomic.LoadInt32(&requests); got < 3 {
		t.Fatalf("requests = %d, want at least 3", got)
	}
	var deviceErr *shelly.DeviceError
	if !errors.As(err, &deviceErr) {
		t.Fatalf("error %v (%T) lost the evidence that the device answered with HTTP 500", err, err)
	}
}

func TestGen1Client_PostKeepsTheEvidenceOfAnAnsweredAttempt(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requests, 1) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("Hijack: %v", err)
			return
		}
		_ = conn.Close() // no answer
	}))
	defer server.Close()

	client := NewClient(strings.TrimPrefix(server.URL, "http://"), WithRetry(2, time.Millisecond))
	err := client.SetWiFiConfig(context.Background(), map[string]interface{}{"ssid": "guest"})

	if err == nil {
		t.Fatal("SetWiFiConfig: expected an error")
	}
	// The control: the retries ran. The HTTP transport may add one resend of its own.
	if got := atomic.LoadInt32(&requests); got < 3 {
		t.Fatalf("requests = %d, want at least 3", got)
	}
	var deviceErr *shelly.DeviceError
	if !errors.As(err, &deviceErr) {
		t.Fatalf("error %v (%T) lost the evidence that the device answered with HTTP 400", err, err)
	}
}

// postedForm runs one write against a fake device and returns the method, the path and the form.
func postedForm(t *testing.T, write func(c *Client) error) (string, string, map[string][]string) {
	t.Helper()
	var gotMethod, gotPath string
	var gotForm map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient(strings.TrimPrefix(server.URL, "http://"))
	if err := write(client); err != nil {
		t.Fatalf("write: %v", err)
	}
	return gotMethod, gotPath, gotForm
}

func TestGen1Client_StationWriteGoesToSettingsSta(t *testing.T) {
	method, path, form := postedForm(t, func(c *Client) error {
		return c.SetWiFiConfig(context.Background(), map[string]interface{}{
			"enabled":     true,
			"ssid":        "office",
			"ipv4_method": "static",
			"netmask":     "255.255.255.0",
			"gateway":     "192.168.1.1",
		})
	})
	if method != http.MethodPost || path != "/settings/sta" {
		t.Errorf("write went to %s %s, want POST /settings/sta", method, path)
	}
	for key, want := range map[string]string{
		"enabled": "true", "ssid": "office", "ipv4_method": "static",
		"netmask": "255.255.255.0", "gateway": "192.168.1.1",
	} {
		if got := form[key]; len(got) != 1 || got[0] != want {
			t.Errorf("form %s = %v, want %q", key, got, want)
		}
	}
	if _, sent := form["key"]; sent {
		t.Errorf("a key that was not set was sent: %v", form["key"])
	}
}

// schedule_rules is a list: it goes out as one comma-separated parameter, whether it arrives as a
// []string or as decoded JSON.
func TestGen1Client_RelayWriteSendsScheduleRulesCommaSeparated(t *testing.T) {
	for name, rules := range map[string]interface{}{
		"strings":   []string{"0700-0123456-on", "2200-0123456-off"},
		"json list": []interface{}{"0700-0123456-on", "2200-0123456-off"},
	} {
		t.Run(name, func(t *testing.T) {
			method, path, form := postedForm(t, func(c *Client) error {
				return c.SetRelaySettings(context.Background(), 1, map[string]interface{}{"schedule_rules": rules, "schedule": true})
			})
			if method != http.MethodPost || path != "/settings/relay/1" {
				t.Errorf("write went to %s %s, want POST /settings/relay/1", method, path)
			}
			if got := form["schedule_rules"]; len(got) != 1 || got[0] != "0700-0123456-on,2200-0123456-off" {
				t.Errorf("form schedule_rules = %v, want one comma-separated value", got)
			}
			if got := form["schedule"]; len(got) != 1 || got[0] != "true" {
				t.Errorf("form schedule = %v, want true", got)
			}
		})
	}
}

func TestGen1Client_AccessPointWriteGoesToSettingsAp(t *testing.T) {
	method, path, form := postedForm(t, func(c *Client) error {
		return c.SetAPConfig(context.Background(), map[string]interface{}{
			"enabled": false,
			"key":     "ap-secret",
		})
	})
	if method != http.MethodPost || path != "/settings/ap" {
		t.Errorf("write went to %s %s, want POST /settings/ap", method, path)
	}
	if got := form["enabled"]; len(got) != 1 || got[0] != "false" {
		t.Errorf("form enabled = %v, want false", got)
	}
	if got := form["key"]; len(got) != 1 || got[0] != "ap-secret" {
		t.Errorf("form key = %v, want ap-secret", got)
	}
}
