package middleware

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// A block starts the IP's counting over, and the block length and the four thresholds are
// configuration (ginsys/shelly-manager#861).

// newConfiguredMonitor builds a monitor from the default configuration, changed by mutate.
func newConfiguredMonitor(t *testing.T, mutate func(*SecurityConfig)) *SecurityMonitor {
	t.Helper()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text", Output: "stdout"})
	require.NoError(t, err)
	cfg := DefaultSecurityConfig()
	if mutate != nil {
		mutate(cfg)
	}
	monitor := NewSecurityMonitor(cfg, logger)
	t.Cleanup(monitor.Close)
	return monitor
}

// bad sends n requests from ip that the handler answered 400: each is one suspicious request.
func bad(monitor *SecurityMonitor, ip string, n int) {
	for i := 0; i < n; i++ {
		monitor.TrackRequest(rulesRequest(ip, "/api/v1/devices"), http.StatusBadRequest, time.Millisecond)
	}
}

func entry(t *testing.T, monitor *SecurityMonitor, ip string) *AttackInfo {
	t.Helper()
	value, ok := monitor.attackMap.Load(ip)
	require.True(t, ok)
	return value.(*AttackInfo)
}

// endBlock moves the end of ip's block into the past, as the passing of its length does.
func endBlock(t *testing.T, monitor *SecurityMonitor, ip string) {
	t.Helper()
	info := entry(t, monitor, ip)
	info.mu.Lock()
	info.BlockedUntil = time.Now().Add(-time.Second)
	info.mu.Unlock()
}

// Check 1.
func TestBlockResetsCountersAndOneMoreSuspiciousDoesNotReblock(t *testing.T) {
	monitor := newConfiguredMonitor(t, nil)
	ip := "10.86.1.1"

	bad(monitor, ip, 10)
	require.True(t, monitor.IsIPBlocked(ip))

	info := entry(t, monitor, ip)
	info.mu.Lock()
	assert.True(t, info.Blocked)
	assert.Equal(t, 0, info.SuspiciousRequests)
	assert.Equal(t, 0, info.RateLimitViolations)
	assert.Empty(t, info.AttackTypes)
	assert.Equal(t, 0, info.recentCount)
	info.mu.Unlock()

	endBlock(t, monitor, ip)
	require.False(t, monitor.IsIPBlocked(ip))

	bad(monitor, ip, 1)
	assert.False(t, monitor.IsIPBlocked(ip), "one more suspicious request after the block does not block again")
	info.mu.Lock()
	assert.Equal(t, 1, info.SuspiciousRequests)
	info.mu.Unlock()
}

// The reset covers a block made by violations and by attack types too, not only by the count.
func TestBlockByViolationsAndByAttackTypesResetsToo(t *testing.T) {
	monitor := newConfiguredMonitor(t, nil)

	ip := "10.86.1.2"
	for i := 0; i < 5; i++ {
		monitor.TrackRateLimitViolation(rulesRequest(ip, "/api/v1/devices"))
	}
	require.True(t, monitor.IsIPBlocked(ip))
	info := entry(t, monitor, ip)
	info.mu.Lock()
	assert.Equal(t, 0, info.RateLimitViolations)
	info.mu.Unlock()

	ip = "10.86.1.3"
	info = monitor.attackInfoFor(ip)
	info.mu.Lock()
	info.AttackTypes["type_a"] = 1
	info.AttackTypes["type_b"] = 1
	info.mu.Unlock()
	bad(monitor, ip, 1) // adds a third attack type
	require.True(t, monitor.IsIPBlocked(ip))
	info.mu.Lock()
	assert.Empty(t, info.AttackTypes)
	info.mu.Unlock()
}

// Check 5.
func TestBlockUsesConfiguredDuration(t *testing.T) {
	monitor := newConfiguredMonitor(t, func(c *SecurityConfig) { c.BlockDuration = 48 * time.Hour })
	ip := "10.86.2.1"

	bad(monitor, ip, 10)
	require.True(t, monitor.IsIPBlocked(ip))

	info := entry(t, monitor, ip)
	info.mu.Lock()
	until := info.BlockedUntil
	info.mu.Unlock()
	assert.WithinDuration(t, time.Now().Add(48*time.Hour), until, time.Minute)
}

func TestLongBlockSurvivesCleanupAndAnEndedBlockDoesNot(t *testing.T) {
	monitor := newConfiguredMonitor(t, nil)
	ip := "10.86.2.2"

	info := monitor.attackInfoFor(ip)
	info.mu.Lock()
	info.Blocked = true
	info.BlockedUntil = time.Now().Add(48 * time.Hour)
	info.LastSeen = time.Now().Add(-25 * time.Hour) // a blocked client's requests stop before the monitor
	info.mu.Unlock()

	monitor.cleanupOldDataOnce(time.Now().Add(-24 * time.Hour))
	_, kept := monitor.attackMap.Load(ip)
	assert.True(t, kept, "a block longer than the cleanup cutoff is not ended by the cleanup")
	assert.True(t, monitor.IsIPBlocked(ip))

	endBlock(t, monitor, ip)
	monitor.cleanupOldDataOnce(time.Now().Add(-24 * time.Hour))
	_, kept = monitor.attackMap.Load(ip)
	assert.False(t, kept, "once the block ended the entry is idle like any other")
}

// Check 6.
func TestMonitoringOnBlockingOffNeverBlocks(t *testing.T) {
	monitor := newConfiguredMonitor(t, func(c *SecurityConfig) { c.EnableIPBlocking = false })
	ip := "10.86.3.1"

	bad(monitor, ip, 12)
	for i := 0; i < 6; i++ {
		monitor.TrackRateLimitViolation(rulesRequest(ip, "/api/v1/devices"))
	}

	assert.False(t, monitor.IsIPBlocked(ip))
	info := entry(t, monitor, ip)
	info.mu.Lock()
	assert.False(t, info.Blocked)
	assert.Equal(t, 12, info.SuspiciousRequests, "counters do not reset: nothing was blocked")
	assert.Equal(t, 6, info.RateLimitViolations)
	info.mu.Unlock()

	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(0), metrics.BlockedRequests)
	assert.Equal(t, 0, metrics.BlockedIPs)
	assert.Equal(t, int64(12), metrics.SuspiciousRequests, "counts continue")
	assert.Equal(t, 1, metrics.ActiveThreats)
}

// Check 10.
func TestBlockedIPStaysInMetricsAfterReset(t *testing.T) {
	monitor := newConfiguredMonitor(t, nil)
	ip := "10.86.4.1"

	bad(monitor, ip, 10)
	require.True(t, monitor.IsIPBlocked(ip))
	info := entry(t, monitor, ip)
	info.mu.Lock()
	info.LastSeen = time.Now().Add(-2 * time.Hour) // a blocked client sends nothing the monitor sees
	info.mu.Unlock()

	metrics := monitor.GetMetrics()
	assert.Equal(t, 1, metrics.BlockedIPs)
	assert.Equal(t, 1, metrics.ActiveThreats, "a blocked IP is an active threat for as long as the block lasts")
	require.Len(t, metrics.TopAttackerIPs, 1)
	row := metrics.TopAttackerIPs[0]
	assert.Equal(t, ip, row.IP)
	assert.True(t, row.Blocked)
	assert.Equal(t, 0, row.SuspiciousRequests, "the row shows the counters as reset")
	assert.Empty(t, row.AttackTypes)
	assert.Equal(t, 10, row.TotalRequests, "the request total is history and stays")
	assert.False(t, row.FirstSeen.IsZero())

	endBlock(t, monitor, ip)
	metrics = monitor.GetMetrics()
	assert.Equal(t, 0, metrics.BlockedIPs)
	assert.Equal(t, 0, metrics.ActiveThreats)
	assert.Empty(t, metrics.TopAttackerIPs, "after the block the reset entry has nothing to list")
}

// Trap 7: a request admitted before the block finishes during it.
func TestInFlightRequestDuringBlockDoesNotCount(t *testing.T) {
	monitor := newConfiguredMonitor(t, nil)
	ip := "10.86.5.1"

	bad(monitor, ip, 10)
	require.True(t, monitor.IsIPBlocked(ip))

	bad(monitor, ip, 3)
	monitor.TrackRateLimitViolation(rulesRequest(ip, "/api/v1/devices"))

	info := entry(t, monitor, ip)
	info.mu.Lock()
	assert.Equal(t, 0, info.SuspiciousRequests)
	assert.Equal(t, 0, info.RateLimitViolations)
	assert.Empty(t, info.AttackTypes)
	assert.Equal(t, 0, info.recentCount)
	assert.Equal(t, 13, info.TotalRequests, "the request total is history: all 13 requests came")
	info.mu.Unlock()
	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(13), metrics.TotalRequests, "the monitor-wide statistics still count them")
	assert.Equal(t, int64(13), metrics.SuspiciousRequests)
	assert.Equal(t, int64(1), metrics.RateLimitViolations)

	endBlock(t, monitor, ip)
	bad(monitor, ip, 1)
	assert.False(t, monitor.IsIPBlocked(ip), "check 1 stays true with requests in flight")
}

// Check 3, 4: each threshold is read from the configuration.
func TestShouldBlockIPReadsEachThresholdFromTheConfig(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name   string
		mutate func(*SecurityConfig)
		info   *AttackInfo
		want   bool
	}{
		{"suspicious 3 of 3", func(c *SecurityConfig) { c.SuspiciousThreshold = 3 }, &AttackInfo{SuspiciousRequests: 3}, true},
		{"suspicious 2 of 3", func(c *SecurityConfig) { c.SuspiciousThreshold = 3 }, &AttackInfo{SuspiciousRequests: 2}, false},
		{"suspicious 12 of 15", func(c *SecurityConfig) { c.SuspiciousThreshold = 15 }, &AttackInfo{SuspiciousRequests: 12}, false},
		{"violations 2 of 2", func(c *SecurityConfig) { c.RateViolationThreshold = 2 }, &AttackInfo{RateLimitViolations: 2}, true},
		{"violations 5 of 8", func(c *SecurityConfig) { c.RateViolationThreshold = 8 }, &AttackInfo{RateLimitViolations: 5}, false},
		{"types 2 of 2", func(c *SecurityConfig) { c.AttackTypeThreshold = 2 }, &AttackInfo{AttackTypes: map[string]int{"a": 1, "b": 1}}, true},
		{"types 3 of 4", func(c *SecurityConfig) { c.AttackTypeThreshold = 4 }, &AttackInfo{AttackTypes: map[string]int{"a": 1, "b": 1, "c": 1}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := newConfiguredMonitor(t, tt.mutate)
			assert.Equal(t, tt.want, monitor.shouldBlockIP(tt.info, now))
		})
	}
}

// The hourly limit sizes the ring: nothing is compiled in.
func TestHourlyThresholdSizesTheRing(t *testing.T) {
	for _, limit := range []int{2, 8} {
		monitor := newConfiguredMonitor(t, func(c *SecurityConfig) {
			c.HourlyThreshold = limit
			c.SuspiciousThreshold = 1000 // only the hourly rule may block here
		})
		ip := "10.86.6.1"
		bad(monitor, ip, limit)
		assert.False(t, monitor.IsIPBlocked(ip), "limit %d: %d inside the hour", limit, limit)
		bad(monitor, ip, 1)
		assert.True(t, monitor.IsIPBlocked(ip), "limit %d: one more blocks", limit)
	}
}

// A threshold of zero or less switches that one rule off; the others stay, and nothing panics.
func TestUnsetThresholdDisablesOnlyThatRule(t *testing.T) {
	for _, hourly := range []int{0, -3} {
		monitor := newConfiguredMonitor(t, func(c *SecurityConfig) { c.HourlyThreshold = hourly })
		ip := "10.86.7.1"
		bad(monitor, ip, 9) // over any hourly limit, under the count rule of 10
		assert.False(t, monitor.IsIPBlocked(ip), "hourly %d: the hourly rule is off", hourly)
		info := entry(t, monitor, ip)
		info.mu.Lock()
		assert.Equal(t, 0, info.recentCount, "hourly %d: the ring records nothing", hourly)
		info.mu.Unlock()
		bad(monitor, ip, 1)
		assert.True(t, monitor.IsIPBlocked(ip), "hourly %d: the count rule still blocks", hourly)
	}

	monitor := newConfiguredMonitor(t, func(c *SecurityConfig) {
		c.SuspiciousThreshold, c.RateViolationThreshold, c.AttackTypeThreshold, c.HourlyThreshold = 0, 0, 0, 0
	})
	bad(monitor, "10.86.7.2", 50)
	assert.False(t, monitor.IsIPBlocked("10.86.7.2"), "a literal configuration with no thresholds blocks nobody")
}
