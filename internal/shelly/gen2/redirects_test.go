package gen2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A device that answers with a redirect is not followed: the request goes to the address it was
// given and nowhere else.
func TestClient_DoesNotFollowRedirects(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"id":"shellyplus1-aabbcc","mac":"AABBCC","model":"SNSW-001X16EU","gen":2,"ver":"1.0.3"}`))
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	// No retry delay: a redirect is a response, and the retry loop would only repeat it.
	client := NewClient(strings.TrimPrefix(redirector.URL, "http://"), WithRetry(1, time.Millisecond))

	if info, err := client.GetInfo(context.Background()); err == nil {
		t.Errorf("Expected an error for a redirect response, got info: %+v", info)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("Expected the redirect target to receive no request, got %d", n)
	}
}
