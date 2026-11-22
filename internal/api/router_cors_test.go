package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
)

const (
	corsListed   = "http://app.example.com"
	corsUnlisted = "http://evil.example.net"
)

// corsRouter builds the production router with the given CORS allowlist.
func corsRouter(t *testing.T, allowed []string) *mux.Router {
	t.Helper()
	t.Setenv(testModeEnv, "")
	h, _ := captureHandler(t)
	secCfg := middleware.DefaultSecurityConfig()
	secCfg.CORSAllowedOrigins = allowed
	r := SetupRoutesWithSecurity(h, h.logger, secCfg, middleware.DefaultValidationConfig())
	closeMonitor(t, h)
	return r
}

// corsDo sends one request; an empty origin sends no Origin header.
func corsDo(r http.Handler, method, path, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if method == http.MethodOptions {
		req.Header.Set("Access-Control-Request-Method", "GET")
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

// corsHeaders returns the names of every Access-Control-* header in the response.
func corsHeaders(rr *httptest.ResponseRecorder) []string {
	var names []string
	for name := range rr.Header() {
		if strings.HasPrefix(strings.ToLower(name), "access-control-") {
			names = append(names, name)
		}
	}
	return names
}

func TestCORSAllowlist_ListedOriginIsEchoed(t *testing.T) {
	r := corsRouter(t, []string{corsListed})
	for _, method := range []string{http.MethodGet, http.MethodOptions} {
		rr := corsDo(r, method, "/api/v1/version", corsListed)
		require.Equal(t, http.StatusOK, rr.Code, method)
		require.Equal(t, corsListed, rr.Header().Get("Access-Control-Allow-Origin"), method)
		require.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Methods"), method)
		require.Equal(t, "Origin", rr.Header().Get("Vary"), method)
	}
}

// A path with no route answers 404, and a page on a listed origin can read that answer: the Metrics
// page reads the missing security route as "monitoring is off" (ginsys/shelly-manager#861).
func TestCORSAllowlist_MissingRouteIs404WithCORSHeaders(t *testing.T) {
	r := corsRouter(t, []string{corsListed})
	rr := corsDo(r, http.MethodGet, "/api/v1/no-such-route", corsListed)
	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Equal(t, corsListed, rr.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "Origin", rr.Header().Get("Vary"))

	rr = corsDo(r, http.MethodGet, "/api/v1/no-such-route", corsUnlisted)
	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Empty(t, corsHeaders(rr), "an unlisted origin still gets no grant")
}

func TestCORSAllowlist_UnlistedOriginGetsNoCORSHeaders(t *testing.T) {
	r := corsRouter(t, []string{corsListed})
	for _, method := range []string{http.MethodGet, http.MethodOptions} {
		rr := corsDo(r, method, "/api/v1/version", corsUnlisted)
		require.Equal(t, http.StatusOK, rr.Code, method+": the request is still served, only the CORS grant is withheld")
		require.Empty(t, corsHeaders(rr), method)
		require.Equal(t, "Origin", rr.Header().Get("Vary"), method)
	}
}

// A near miss is not a match: equality, not substring or case folding.
func TestCORSAllowlist_NearMissOriginsAreRefused(t *testing.T) {
	r := corsRouter(t, []string{corsListed})
	for _, origin := range []string{
		corsListed + ".evil.test",
		"https://app.example.com",
		"http://APP.example.com",
		corsListed + "/",
		"example.com",
	} {
		rr := corsDo(r, http.MethodGet, "/api/v1/version", origin)
		require.Empty(t, corsHeaders(rr), origin)
	}
}

func TestCORSAllowlist_RequestWithoutOriginIsAsToday(t *testing.T) {
	r := corsRouter(t, []string{corsListed})
	rr := corsDo(r, http.MethodGet, "/api/v1/version", "")
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSAllowlist_EmptyListKeepsWildcard(t *testing.T) {
	r := corsRouter(t, nil)
	for _, method := range []string{http.MethodGet, http.MethodOptions} {
		rr := corsDo(r, method, "/api/v1/version", corsUnlisted)
		require.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"), method)
		require.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Methods"), method)
	}
}

func TestCORSAllowlist_WildcardEntryEchoesAnyOrigin(t *testing.T) {
	r := corsRouter(t, []string{"*"})
	rr := corsDo(r, http.MethodGet, "/api/v1/version", corsUnlisted)
	require.Equal(t, corsUnlisted, rr.Header().Get("Access-Control-Allow-Origin"))
}
