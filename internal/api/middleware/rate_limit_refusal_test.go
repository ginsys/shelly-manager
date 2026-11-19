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
	assert.True(t, refusal.reset.After(end), "the first whole second after the window's last instant")
	assert.LessOrEqual(t, refusal.reset.Sub(end), time.Second)
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

// A counter resets only when MORE than a window has passed, so a client that returns at the reset second
// must be let in, also when the window ends exactly on a whole second.
func TestRateLimiterClientReturningAtTheAdvertisedResetIsAllowed(t *testing.T) {
	for _, offset := range []time.Duration{0, 400 * time.Millisecond} {
		cfg := DefaultSecurityConfig()
		cfg.RateLimitWindow = time.Hour
		cfg.RateLimitByPath = map[string]int{"/api/v1/config/bulk": 3}
		rl := newTestLimiter(cfg)
		ip := "10.65.0.5"
		require.Nil(t, rl.check(ip, bulkPath))

		// A window that began on a whole second (offset 0) or a little after one, half a window ago.
		windowStart := time.Now().Add(-30 * time.Minute).Truncate(time.Second).Add(offset)
		rl.mutex.Lock()
		client := rl.clients[ip]
		counter := client.paths["/api/v1/config/bulk"]
		require.NotNil(t, counter)
		counter.window = windowStart
		counter.requests = 3
		rl.mutex.Unlock()

		refusal := rl.check(ip, bulkPath)
		require.NotNil(t, refusal, "offset %v: the 4th bulk call is refused", offset)
		require.True(t, refusal.reset.After(windowStart.Add(time.Hour)), "offset %v", offset)

		rl.mutex.Lock()
		again := rl.allowPath(client, ip, bulkPath, refusal.reset)
		rl.mutex.Unlock()
		assert.Nil(t, again, "offset %v: a client returning at the reset second is not refused again", offset)
	}
}
