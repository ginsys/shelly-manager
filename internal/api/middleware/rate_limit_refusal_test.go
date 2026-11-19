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

// A refusal by the global limit reports when the client is let in again (ginsys/shelly-manager#677): the
// first whole second after the later of the end of its window and the end of its 15-minute block.

// blockedGlobalClient makes a client go over a global limit of 2, then moves its window start back by
// windowAge, as if the window had begun that long before the block. It returns the limiter, the client
// and the moment the block began.
func blockedGlobalClient(t *testing.T, ip string, windowAge time.Duration) (*RateLimiter, *clientInfo, time.Time) {
	t.Helper()
	cfg := DefaultSecurityConfig()
	cfg.RateLimit = 2
	cfg.RateLimitWindow = time.Hour
	cfg.RateLimitByPath = map[string]int{}
	rl := newTestLimiter(cfg)
	require.Nil(t, rl.check(ip, "/api/v1/ping"))
	require.Nil(t, rl.check(ip, "/api/v1/ping"))
	require.NotNil(t, rl.check(ip, "/api/v1/ping"), "the 3rd request goes over the limit and blocks the client")

	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	client := rl.clients[ip]
	require.True(t, client.blocked)
	client.window = client.window.Add(-windowAge)
	return rl, client, client.blockTime
}

// assertResetAfter checks that the refusal's reset is the first whole second after end and that
// retry_after reaches it in whole seconds.
func assertResetAfter(t *testing.T, refusal *rateRefusal, end, now time.Time) {
	t.Helper()
	require.NotNil(t, refusal)
	assert.Equal(t, 2, refusal.limit, "the global limit")
	assert.Equal(t, 0, refusal.reset.Nanosecond(), "the reset is a whole second")
	assert.True(t, refusal.reset.After(end), "after the last instant the client is still refused")
	assert.LessOrEqual(t, refusal.reset.Sub(end), time.Second, "the first whole second after it")
	assert.GreaterOrEqual(t, refusal.retryAfter, refusal.reset.Sub(now), "retry_after reaches the reset second")
	assert.Less(t, refusal.retryAfter-refusal.reset.Sub(now), time.Second)
	assert.Equal(t, time.Duration(0), refusal.retryAfter%time.Second, "retry_after is whole seconds")
}

// The window began 30 minutes ago and the block lasts to 15 minutes from now: the window ends later.
func TestRateLimiterGlobalRefusalReportsTheEndOfTheWindow(t *testing.T) {
	ip := "10.77.0.1"
	rl, client, _ := blockedGlobalClient(t, ip, 30*time.Minute)

	refusal := rl.check(ip, "/api/v1/ping") // refused by the block
	now := time.Now()
	rl.mutex.Lock()
	end := client.window.Add(time.Hour)
	rl.mutex.Unlock()
	assertResetAfter(t, refusal, end, now)
	assert.Less(t, refusal.retryAfter, 31*time.Minute, "about 30 minutes, not a full window")
}

// The window began 55 minutes ago and the block lasts to 15 minutes from now: the block ends later, and
// after it the client is refused again only until the window resets, which has already happened.
func TestRateLimiterGlobalRefusalReportsTheEndOfTheBlockWhenItOutlastsTheWindow(t *testing.T) {
	ip := "10.77.0.2"
	rl, _, blockTime := blockedGlobalClient(t, ip, 55*time.Minute)

	refusal := rl.check(ip, "/api/v1/ping")
	now := time.Now()
	assertResetAfter(t, refusal, blockTime.Add(15*time.Minute), now)
	assert.Less(t, refusal.retryAfter, 16*time.Minute, "about 15 minutes, not the 5 left of the window")
}

// A client that returns at the reset second is let in, whether the window or the block ends later.
func TestRateLimiterGlobalClientReturningAtTheAdvertisedResetIsAllowed(t *testing.T) {
	for _, windowAge := range []time.Duration{30 * time.Minute, 55 * time.Minute} {
		ip := "10.77.0.3"
		rl, client, _ := blockedGlobalClient(t, ip, windowAge)
		refusal := rl.check(ip, "/api/v1/ping")
		require.NotNil(t, refusal, "window age %v: still blocked", windowAge)

		// Move the client's times back by the wait, as if it had come back at the reset second.
		wait := time.Until(refusal.reset)
		rl.mutex.Lock()
		client.window = client.window.Add(-wait)
		client.blockTime = client.blockTime.Add(-wait)
		rl.mutex.Unlock()
		assert.Nil(t, rl.check(ip, "/api/v1/ping"), "window age %v: let in at the reset second", windowAge)
	}
}
