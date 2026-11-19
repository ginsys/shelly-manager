package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Each block rule of shouldBlockIP fires at its own stated threshold (ginsys/shelly-manager#666).

func newRulesMonitor(t *testing.T) *SecurityMonitor {
	t.Helper()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text", Output: "stdout"})
	require.NoError(t, err)
	monitor := NewSecurityMonitor(DefaultSecurityConfig(), logger)
	t.Cleanup(monitor.Close)
	return monitor
}

func rulesRequest(ip, path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = ip + ":4000"
	req.Header.Set("User-Agent", chainUserAgent)
	return req
}

func TestShouldBlockIPRateRuleIsARollingHour(t *testing.T) {
	monitor := newRulesMonitor(t)
	now := time.Now()

	tests := []struct {
		name  string
		times []time.Duration // how long before now each suspicious request came
		want  bool
	}{
		{"5 inside the hour", []time.Duration{50 * time.Minute, 40 * time.Minute, 30 * time.Minute, 20 * time.Minute, 10 * time.Minute}, false},
		{"6 inside the hour", []time.Duration{50 * time.Minute, 40 * time.Minute, 30 * time.Minute, 20 * time.Minute, 10 * time.Minute, time.Minute}, true},
		{"6 where the oldest is just over an hour old", []time.Duration{61 * time.Minute, 40 * time.Minute, 30 * time.Minute, 20 * time.Minute, 10 * time.Minute, time.Minute}, false},
		{"6 spread over a day", []time.Duration{24 * time.Hour, 20 * time.Hour, 15 * time.Hour, 10 * time.Hour, 5 * time.Hour, time.Minute}, false},
		{"8 where only the last 6 are inside the hour", []time.Duration{9 * time.Hour, 8 * time.Hour, 50 * time.Minute, 40 * time.Minute, 30 * time.Minute, 20 * time.Minute, 10 * time.Minute, time.Minute}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &AttackInfo{AttackTypes: map[string]int{}}
			for _, ago := range tt.times {
				info.recordSuspiciousTime(now.Add(-ago))
			}
			assert.Equal(t, tt.want, monitor.shouldBlockIP(info, now))
		})
	}
}

func TestShouldBlockIPOtherRulesKeepTheirThresholds(t *testing.T) {
	monitor := newRulesMonitor(t)
	now := time.Now()

	assert.False(t, monitor.shouldBlockIP(&AttackInfo{SuspiciousRequests: 9}, now), "9 suspicious requests")
	assert.True(t, monitor.shouldBlockIP(&AttackInfo{SuspiciousRequests: 10}, now), "10 suspicious requests")
	assert.False(t, monitor.shouldBlockIP(&AttackInfo{RateLimitViolations: 4}, now), "4 violations")
	assert.True(t, monitor.shouldBlockIP(&AttackInfo{RateLimitViolations: 5}, now), "5 violations")
	assert.False(t, monitor.shouldBlockIP(&AttackInfo{AttackTypes: map[string]int{"a": 1, "b": 1}}, now), "2 attack types")
	assert.True(t, monitor.shouldBlockIP(&AttackInfo{AttackTypes: map[string]int{"a": 1, "b": 1, "c": 1}}, now), "3 attack types")
}

// A 429 counts once, as a rate limit violation, never also as suspicious for its status.
func TestTrackRequestCountsA429OnceAsAViolation(t *testing.T) {
	monitor := newRulesMonitor(t)
	ip := "10.66.1.1"

	monitor.TrackRequest(rulesRequest(ip, "/api/v1/devices"), http.StatusTooManyRequests, time.Millisecond)

	value, ok := monitor.attackMap.Load(ip)
	require.True(t, ok)
	info := value.(*AttackInfo)
	assert.Equal(t, 1, info.RateLimitViolations)
	assert.Equal(t, 0, info.SuspiciousRequests)
	assert.False(t, monitor.IsIPBlocked(ip))

	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(1), metrics.RateLimitViolations)
	assert.Equal(t, int64(0), metrics.SuspiciousRequests)
}

// Other signals on the same request still count: a 429 from a client with no User-Agent is one
// violation and one suspicious request, because the empty User-Agent is its own signal.
func TestTrackRequestA429KeepsItsOtherSuspiciousSignals(t *testing.T) {
	monitor := newRulesMonitor(t)
	ip := "10.66.1.2"
	req := rulesRequest(ip, "/api/v1/devices")
	req.Header.Del("User-Agent")

	monitor.TrackRequest(req, http.StatusTooManyRequests, time.Millisecond)

	value, ok := monitor.attackMap.Load(ip)
	require.True(t, ok)
	info := value.(*AttackInfo)
	assert.Equal(t, 1, info.RateLimitViolations)
	assert.Equal(t, 1, info.SuspiciousRequests)
}

func TestTrackRateLimitViolationBlocksAtTheFifthAndCreatesTheEntry(t *testing.T) {
	monitor := newRulesMonitor(t)
	ip := "10.66.1.3"

	for i := 1; i <= 4; i++ {
		monitor.TrackRateLimitViolation(rulesRequest(ip, "/api/v1/devices"))
		assert.False(t, monitor.IsIPBlocked(ip), fmt.Sprintf("violation %d", i))
	}
	monitor.TrackRateLimitViolation(rulesRequest(ip, "/api/v1/devices"))
	assert.True(t, monitor.IsIPBlocked(ip), "the 5th violation blocks")
}

// One IP from many goroutines: the writers (TrackRequest, TrackRateLimitViolation, IsIPBlocked) share
// the per-IP lock. Meaningful under -race.
func TestTrackingOneIPConcurrentlyIsRaceFree(t *testing.T) {
	monitor := newRulesMonitor(t)
	ip := "10.66.1.4"
	done := make(chan struct{})

	for w := 0; w < 8; w++ {
		go func(w int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 50; j++ {
				switch j % 3 {
				case 0:
					monitor.TrackRequest(rulesRequest(ip, "/api/v1/devices"), http.StatusBadRequest, time.Millisecond)
				case 1:
					monitor.TrackRateLimitViolation(rulesRequest(ip, "/api/v1/devices"))
				default:
					monitor.IsIPBlocked(ip)
				}
			}
		}(w)
	}
	for w := 0; w < 8; w++ {
		<-done
	}
	assert.True(t, monitor.IsIPBlocked(ip))
}

// A block made by direct violations must survive the hourly cleanup: the entry carries a LastSeen.
func TestCleanupKeepsAnEntryBlockedByDirectViolations(t *testing.T) {
	monitor := newRulesMonitor(t)
	ip := "10.66.1.5"

	for i := 0; i < 5; i++ {
		monitor.TrackRateLimitViolation(rulesRequest(ip, "/api/v1/devices"))
	}
	require.True(t, monitor.IsIPBlocked(ip))

	monitor.cleanupOldDataOnce(time.Now().Add(-24 * time.Hour))

	_, kept := monitor.attackMap.Load(ip)
	assert.True(t, kept, "a freshly blocked entry is not dropped by the cleanup")
	assert.True(t, monitor.IsIPBlocked(ip))
}

// A new entry is visible to the cleanup the moment attackInfoFor returns it, before any tracking
// call sets LastSeen: it must already carry one.
func TestNewEntryCarriesLastSeenWhenItIsPublished(t *testing.T) {
	monitor := newRulesMonitor(t)
	ip := "10.66.1.6"

	info := monitor.attackInfoFor(ip)
	assert.False(t, info.LastSeen.IsZero())
	assert.False(t, info.FirstSeen.IsZero())

	monitor.cleanupOldDataOnce(time.Now().Add(-24 * time.Hour))
	_, kept := monitor.attackMap.Load(ip)
	assert.True(t, kept, "a just-created entry is not dropped by the cleanup")
}

func TestTrustedIPIsNeverTrackedForViolations(t *testing.T) {
	monitor := newRulesMonitor(t)

	for i := 0; i < 10; i++ {
		monitor.TrackRequest(rulesRequest("127.0.0.1", "/api/v1/devices"), http.StatusTooManyRequests, time.Millisecond)
	}
	assert.False(t, monitor.IsIPBlocked("127.0.0.1"))
	_, tracked := monitor.attackMap.Load("127.0.0.1")
	assert.False(t, tracked)
}
