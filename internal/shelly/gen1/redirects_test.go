package gen1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// A device that answers with a redirect is not followed: the request goes to the address it was
// given and nowhere else.
func TestClient_DoesNotFollowRedirects(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"type":"SHPLG-S","mac":"A4CF12345678","auth":false,"fw":"20231219-134356"}`))
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	client := NewClient(strings.TrimPrefix(redirector.URL, "http://"))

	if info, err := client.GetInfo(context.Background()); err == nil {
		t.Errorf("Expected an error for a redirect response, got info: %+v", info)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("Expected the redirect target to receive no request, got %d", n)
	}
}
