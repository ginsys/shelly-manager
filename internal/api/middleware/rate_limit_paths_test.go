package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A path limit counts only the requests to its own paths; the global limit counts everything
// (ginsys/shelly-manager#658).

const (
	bulkPath    = "/api/v1/config/bulk-import"
	devicesPath = "/api/v1/devices"
)

func controlPath(id int) string { return fmt.Sprintf("/api/v1/devices/%d/control", id) }

func newTestLimiter(cfg *SecurityConfig) *RateLimiter { return NewRateLimiter(cfg, nil) }

// allowN sends n requests to path and returns how many were allowed.
func allowN(rl *RateLimiter, ip, path string, n int) int {
	allowed := 0
	for i := 0; i < n; i++ {
		if rl.Allow(ip, path) {
			allowed++
		}
	}
	return allowed
}

func TestRateLimiterBulkCountsOnlyBulkRequests(t *testing.T) {
	rl := newTestLimiter(DefaultSecurityConfig())
	ip := "10.58.0.1"

	// Ordinary traffic well under the global limit must not use up the bulk budget.
	require.Equal(t, 25, allowN(rl, ip, devicesPath, 25))
	assert.True(t, rl.Allow(ip, bulkPath), "the first bulk call after 25 other requests is allowed")
}

func TestRateLimiterBulkLimitRefusesTheNextCallOnly(t *testing.T) {
	rl := newTestLimiter(DefaultSecurityConfig())
	ip := "10.58.0.2"

	assert.Equal(t, 20, allowN(rl, ip, bulkPath, 20), "the bulk limit is 20 calls")
	assert.False(t, rl.Allow(ip, bulkPath), "the 21st bulk call is refused")

	// The refusal is for the bulk rule: it sets no block for the other routes.
	assert.True(t, rl.Allow(ip, devicesPath), "an ordinary route still works after a bulk refusal")
	assert.True(t, rl.Allow(ip, controlPath(7)), "another rule still works after a bulk refusal")
	assert.False(t, rl.Allow(ip, bulkPath), "bulk stays refused until its own window resets")
}

func TestRateLimiterControlLimitCountsPerDevice(t *testing.T) {
	rl := newTestLimiter(DefaultSecurityConfig())
	ip := "10.58.0.3"

	assert.Equal(t, 100, allowN(rl, ip, controlPath(7), 100), "100 control calls to device 7 pass")
	assert.False(t, rl.Allow(ip, controlPath(7)), "the 101st control call to device 7 is refused")
	assert.True(t, rl.Allow(ip, controlPath(8)), "device 8 has its own counter")
	assert.True(t, rl.Allow(ip, devicesPath), "a control refusal sets no block for the other routes")
}

func TestRateLimiterProvisioningAndBulkHaveSeparateCounters(t *testing.T) {
	rl := newTestLimiter(DefaultSecurityConfig())
	ip := "10.58.0.4"

	assert.Equal(t, 50, allowN(rl, ip, "/api/v1/provisioning/tasks", 50))
	assert.False(t, rl.Allow(ip, "/api/v1/provisioning/status"), "the 51st provisioning call is refused")
	assert.True(t, rl.Allow(ip, bulkPath), "bulk is not affected by provisioning calls")
}

func TestRateLimiterGlobalLimitAndBlockAreUnchanged(t *testing.T) {
	cfg := DefaultSecurityConfig()
	cfg.RateLimit = 10
	cfg.RateLimitByPath = map[string]int{"/api/v1/config/bulk": 3}
	rl := newTestLimiter(cfg)
	ip := "10.58.0.5"

	require.Equal(t, 10, allowN(rl, ip, devicesPath, 10))
	assert.False(t, rl.Allow(ip, devicesPath), "the 11th request is over the global limit")
	// The global limit blocks the client on every route, bulk included.
	assert.False(t, rl.Allow(ip, "/api/v1/other"), "the global limit blocks all routes")
	assert.False(t, rl.Allow(ip, bulkPath), "the global limit blocks the bulk route too")
}

func TestRateLimiterRefusedPathRequestStillCountsGlobally(t *testing.T) {
	cfg := DefaultSecurityConfig()
	cfg.RateLimit = 5
	cfg.RateLimitByPath = map[string]int{"/api/v1/config/bulk": 2}
	rl := newTestLimiter(cfg)
	ip := "10.58.0.6"

	assert.True(t, rl.Allow(ip, bulkPath))                 // global 1
	assert.True(t, rl.Allow(ip, bulkPath))                 // global 2
	assert.False(t, rl.Allow(ip, bulkPath))                // refused by the rule, global 3
	assert.True(t, rl.Allow(ip, devicesPath))              // global 4
	assert.True(t, rl.Allow(ip, devicesPath))              // global 5
	assert.False(t, rl.Allow(ip, devicesPath), "global 6") // over the global limit
}

func TestRateLimiterPathCountersAreWindowedAndPerClient(t *testing.T) {
	cfg := DefaultSecurityConfig()
	cfg.RateLimitWindow = 300 * time.Millisecond
	cfg.RateLimitByPath = map[string]int{"/api/v1/config/bulk": 2}
	rl := newTestLimiter(cfg)

	require.Equal(t, 2, allowN(rl, "10.58.0.7", bulkPath, 2))
	assert.False(t, rl.Allow("10.58.0.7", bulkPath))
	assert.True(t, rl.Allow("10.58.0.8", bulkPath), "another client has its own bulk counter")

	time.Sleep(400 * time.Millisecond)
	assert.True(t, rl.Allow("10.58.0.7", bulkPath), "the bulk counter resets with its window")
}

// The control route parses the id as a decimal number, so 7, 07 and 007 are one device: they
// share one counter, or a client could dodge the limit by changing the spelling.
func TestRateLimiterControlLimitCountsOneDeviceUnderEverySpelling(t *testing.T) {
	rl := newTestLimiter(DefaultSecurityConfig())
	ip := "10.58.0.7"

	assert.Equal(t, 100, allowN(rl, ip, controlPath(7), 100), "the control limit is 100 calls")
	assert.False(t, rl.Allow(ip, "/api/v1/devices/07/control"), "a leading zero is the same device")
	assert.False(t, rl.Allow(ip, "/api/v1/devices/0007/control"), "so are more leading zeros")
	assert.True(t, rl.Allow(ip, controlPath(70)), "70 is another device")
	assert.True(t, rl.Allow(ip, "/api/v1/devices/0/control"), "0 is its own device")
	assert.True(t, rl.Allow(ip, "/api/v1/devices/00/control"), "00 is device 0")
}

func TestRateLimiterLongestMatchingRuleWins(t *testing.T) {
	// Map order is random, so run the case many times.
	for i := 0; i < 50; i++ {
		cfg := DefaultSecurityConfig()
		cfg.RateLimitByPath = map[string]int{
			"/api/v1/config":      100,
			"/api/v1/config/bulk": 2,
		}
		rl := newTestLimiter(cfg)
		require.Equal(t, 2, allowN(rl, "10.58.0.9", bulkPath, 2), "run %d", i)
		require.False(t, rl.Allow("10.58.0.9", bulkPath), "run %d: the 3rd bulk call uses the bulk rule", i)
	}
}

func TestRateLimitMiddlewareAnswersAPathRefusalLikeAGlobalOne(t *testing.T) {
	cfg := DefaultSecurityConfig()
	cfg.RateLimit = 100
	cfg.RateLimitByPath = map[string]int{"/api/v1/config/bulk": 1}
	handler := RateLimitMiddleware(cfg, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	serve := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, nil)
		req.RemoteAddr = "10.58.0.10:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}

	require.Equal(t, http.StatusOK, serve(bulkPath).Code)
	rr := serve(bulkPath)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "100", rr.Header().Get("X-RateLimit-Limit"), "the headers keep the global limit")
	assert.Equal(t, "0", rr.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Reset"))
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, false, body["success"])
	assert.Equal(t, "RATE_LIMIT_EXCEEDED", body["error"].(map[string]any)["code"])

	assert.Equal(t, http.StatusOK, serve(devicesPath).Code, "other routes are not blocked")
}

func TestRateLimiterMatchRule(t *testing.T) {
	rl := newTestLimiter(DefaultSecurityConfig())
	tests := []struct {
		path       string
		wantLimit  int
		wantKey    string // the counter key: the matched prefix with the id filled in; "" for no rule
		wantNoRule bool
	}{
		{path: "/api/v1/devices/7/control", wantLimit: 100, wantKey: "/api/v1/devices/7/control"},
		{path: "/api/v1/devices/abc-12/control/extra", wantLimit: 100, wantKey: "/api/v1/devices/abc-12/control"},
		{path: "/api/v1/devices/007/control", wantLimit: 100, wantKey: "/api/v1/devices/7/control"},
		{path: "/api/v1/devices/00/control", wantLimit: 100, wantKey: "/api/v1/devices/0/control"},
		{path: "/api/v1/devices/0a7/control", wantLimit: 100, wantKey: "/api/v1/devices/0a7/control"},
		{path: "/api/v1/devices//control", wantNoRule: true},
		{path: "/api/v1/devices/7/status", wantNoRule: true},
		{path: "/api/v1/devices/7", wantNoRule: true},
		{path: "/api/v1/devices", wantNoRule: true},
		{path: "/api/v1/provisioning/tasks/3/cancel", wantLimit: 50, wantKey: "/api/v1/provisioning"},
		{path: "/api/v1/provisioning/bulk", wantLimit: 50, wantKey: "/api/v1/provisioning"},
		{path: "/api/v1/provisioner/agents/register", wantNoRule: true},
		{path: "/api/v1/config/bulk-import", wantLimit: 20, wantKey: "/api/v1/config/bulk"},
		{path: "/api/v1/config/bulk-drift-detect-enhanced", wantLimit: 20, wantKey: "/api/v1/config/bulk"},
		{path: "/api/v1/config/bulk", wantLimit: 20, wantKey: "/api/v1/config/bulk"},
		{path: "/api/v1/config", wantNoRule: true},
		{path: "/x/api/v1/config/bulk", wantNoRule: true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			limit, key, ok := rl.matchRule(tt.path)
			assert.Equal(t, !tt.wantNoRule, ok)
			if ok {
				assert.Equal(t, tt.wantLimit, limit)
				assert.Equal(t, tt.wantKey, key)
			}
		})
	}
}

// A client that stays active keeps its entry, so its expired path counters are pruned by themselves:
// a client that asks for a fresh device id every time must not grow the map for ever.
func TestRateLimiterPrunesExpiredPathCounters(t *testing.T) {
	cfg := DefaultSecurityConfig()
	rl := newTestLimiter(cfg)
	ip := "10.58.0.11"

	for id := 1; id <= 5; id++ {
		require.True(t, rl.Allow(ip, controlPath(id)))
	}
	require.Len(t, rl.clients[ip].paths, 5)

	// Within the window nothing is pruned.
	rl.prune(time.Now().Add(cfg.RateLimitWindow / 2))
	require.Len(t, rl.clients[ip].paths, 5)

	// Past the window the counters are gone, and the client entry stays (its own window is not
	// two hours old).
	rl.prune(time.Now().Add(cfg.RateLimitWindow + time.Minute))
	require.Contains(t, rl.clients, ip)
	assert.Empty(t, rl.clients[ip].paths)

	// A fresh counter made after the prune survives a prune at the same instant.
	rl.mutex.Lock()
	rl.clients[ip].paths = map[string]*pathCount{
		"/api/v1/devices/9/control": {requests: 1, window: time.Now().Add(cfg.RateLimitWindow)},
	}
	rl.mutex.Unlock()
	rl.prune(time.Now().Add(cfg.RateLimitWindow + time.Minute))
	assert.Len(t, rl.clients[ip].paths, 1)
}
