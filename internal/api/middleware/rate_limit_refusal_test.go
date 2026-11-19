package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The limiter reports which limit refused a request and when that limit's window ends for the client
// (ginsys/shelly-manager#665). The middleware writes it into the response.

// The window end of a rule is read from the rule's own counter, not taken as now plus the window.
func TestRateLimiterPathRefusalReportsTheEndOfTheRulesWindow(t *testing.T) {
	cfg := DefaultSecurityConfig()
	cfg.RateLimitWindow = time.Hour
	cfg.RateLimitByPath = map[string]int{"/api/v1/config/bulk": 3}
	rl := newTestLimiter(cfg)
	ip := "10.65.0.2"
	require.Nil(t, rl.check(ip, bulkPath), "the first bulk call is allowed")

	// Move the rule's window 30 minutes back and fill it, as if the calls were half a window old.
	windowStart := time.Now().Add(-30*time.Minute - 400*time.Millisecond)
	rl.mutex.Lock()
	counter := rl.clients[ip].paths["/api/v1/config/bulk"]
	require.NotNil(t, counter)
	counter.window = windowStart
	counter.requests = 3
	rl.mutex.Unlock()

	refusal := rl.check(ip, bulkPath)
	now := time.Now()
	require.NotNil(t, refusal, "the 4th bulk call is refused")
	assert.Equal(t, 3, refusal.limit)
	end := windowStart.Add(time.Hour)
	assert.Equal(t, 0, refusal.reset.Nanosecond(), "the reset is a whole second")
	assert.False(t, refusal.reset.Before(end), "rounded up, so the client is not refused again at the reset")
	assert.Less(t, refusal.reset.Sub(end), time.Second)
	assert.GreaterOrEqual(t, refusal.retryAfter, refusal.reset.Sub(now), "retry_after reaches the reset second")
	assert.Less(t, refusal.retryAfter-refusal.reset.Sub(now), time.Second)
	assert.Equal(t, time.Duration(0), refusal.retryAfter%time.Second, "retry_after is whole seconds")
	assert.Less(t, refusal.retryAfter, 31*time.Minute, "about 30 minutes, not a full window")
}

func TestRateLimiterPathRefusalOfADeviceRuleReportsThatRulesLimit(t *testing.T) {
	rl := newTestLimiter(DefaultSecurityConfig())
	ip := "10.65.0.3"
	for i := 0; i < 100; i++ {
		require.Nil(t, rl.check(ip, controlPath(7)))
	}
	refusal := rl.check(ip, controlPath(7))
	require.NotNil(t, refusal)
	assert.Equal(t, 100, refusal.limit, "the per-device control rule, not the global limit")
}
