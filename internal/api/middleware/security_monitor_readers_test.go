package middleware

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The readers of the monitor (GetMetrics and the cleanup) take the same per-IP lock as the writers,
// and GetMetrics hands out copies (ginsys/shelly-manager#671).

// GetMetrics returns snapshots: the caller encodes them after GetMetrics returns, while the monitor
// keeps writing the maps it read from.
func TestGetMetricsReturnsCopies(t *testing.T) {
	monitor := newRulesMonitor(t)
	ip := "10.71.0.1"
	monitor.TrackRequest(rulesRequest(ip, "/api/v1/devices"), http.StatusBadRequest, time.Millisecond)

	first := monitor.GetMetrics()
	require.Len(t, first.TopAttackerIPs, 1)
	first.TopAttackerIPs[0].AttackTypes["planted"] = 9
	first.AttacksByType["planted"] = 9
	first.AlertsByLevel["planted"] = 9

	value, ok := monitor.attackMap.Load(ip)
	require.True(t, ok)
	info := value.(*AttackInfo)
	info.mu.Lock()
	_, shared := info.AttackTypes["planted"]
	info.mu.Unlock()
	assert.False(t, shared, "a summary's AttackTypes is a copy of the entry's map")

	second := monitor.GetMetrics()
	assert.NotContains(t, second.AttacksByType, "planted", "AttacksByType is a copy of the monitor's map")
	assert.NotContains(t, second.AlertsByLevel, "planted", "AlertsByLevel is a copy of the monitor's map")
	require.Len(t, second.TopAttackerIPs, 1)
	assert.NotContains(t, second.TopAttackerIPs[0].AttackTypes, "planted")
}

// One IP is tracked, read through GetMetrics (whose result is iterated, as the handler's encoder does)
// and swept by the cleanup at the same time. Under -race an unlocked reader or a shared map fails
// here; a reader that takes the per-IP lock while holding the statistics lock stalls here (the writers
// take the per-IP lock first and the statistics lock second), and the bounded wait reports the stall.
func TestMonitorReadersAreRaceFreeForOneIP(t *testing.T) {
	monitor := newRulesMonitor(t)
	ip := "10.71.0.2"
	const (
		writers    = 4
		readers    = 3
		iterations = 400
	)

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				// A 400 is suspicious, so every call writes AttackTypes and the statistics maps.
				monitor.TrackRequest(rulesRequest(ip, "/api/v1/devices"), http.StatusBadRequest, time.Millisecond)
			}
		}()
	}
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				metrics := monitor.GetMetrics()
				for _, summary := range metrics.TopAttackerIPs {
					for range summary.AttackTypes {
					}
				}
				for range metrics.AttacksByType {
				}
				for range metrics.AlertsByLevel {
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			// A cutoff in the past keeps the entry: the cleanup only reads LastSeen.
			monitor.cleanupOldDataOnce(time.Now().Add(-24 * time.Hour))
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the readers and writers of one IP stalled for 30 s: a lock order deadlock")
	}

	value, ok := monitor.attackMap.Load(ip)
	require.True(t, ok, "the cleanup kept the entry")
	info := value.(*AttackInfo)
	info.mu.Lock()
	defer info.mu.Unlock()
	assert.Equal(t, writers*iterations, info.TotalRequests)
}
