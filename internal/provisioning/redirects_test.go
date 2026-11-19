package provisioning

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// A device that answers with a redirect is not followed, and the answer is an error: a 3xx is
// not a success for the call that asked for a change on the device.
func TestMakeDeviceRequest_RefusesRedirects(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	sp := NewShellyProvisioner(logging.GetDefault(), nil)

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		var params interface{}
		if method == http.MethodPost {
			params = map[string]string{"ssid": "home", "pass": "secret"}
		}
		err := sp.makeDeviceRequest(context.Background(), method, redirector.URL+"/settings", params)
		if err == nil {
			t.Errorf("%s: expected an error for a redirect response, got nil", method)
		} else if !strings.Contains(err.Error(), "307") {
			t.Errorf("%s: expected the error to name status 307, got: %v", method, err)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("Expected the redirect target to receive no request, got %d", n)
	}
}
