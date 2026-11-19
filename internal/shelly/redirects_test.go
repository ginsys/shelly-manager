package shelly

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// redirectingServer answers every request with a 307 to a second server, which counts what it
// receives. It returns the first server's host:port and the counter.
func redirectingServer(t *testing.T) (host string, hits *atomic.Int32) {
	t.Helper()

	hits = &atomic.Int32{}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"id":"shellyplus1-aabbcc","gen":2,"type":"SHSW-1","mac":"AABBCC"}`))
	}))
	t.Cleanup(target.Close)

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redirector.Close)

	return strings.TrimPrefix(redirector.URL, "http://"), hits
}

// A device that answers with a redirect is not followed: the request goes to the address it was
// given and nowhere else.
func TestFactory_DetectGeneration_DoesNotFollowRedirects(t *testing.T) {
	host, hits := redirectingServer(t)

	generation, err := NewFactory().DetectGeneration(context.Background(), host)
	if err == nil {
		t.Errorf("Expected an error for a redirect response, got generation %d", generation)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("Expected the redirect target to receive no request, got %d", n)
	}
}

func TestDetectGeneration_DoesNotFollowRedirects(t *testing.T) {
	host, hits := redirectingServer(t)

	generation, err := DetectGeneration(context.Background(), host)
	if err == nil {
		t.Errorf("Expected an error for a redirect response, got generation %d", generation)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("Expected the redirect target to receive no request, got %d", n)
	}
}
