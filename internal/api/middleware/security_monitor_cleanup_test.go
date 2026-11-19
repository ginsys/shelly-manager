package middleware

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A request and the hourly cleanup agree on whether an IP's entry is alive (ginsys/shelly-manager#678):
// an entry the cleanup deleted is never written into by a request that loaded it just before.

// idleCutoff is the cleanup's real cutoff: entries idle for more than a day go.
func idleCutoff() time.Time { return time.Now().Add(-24 * time.Hour) }

// ageEntry makes the entry of ip look idle for two days.
func ageEntry(sm *SecurityMonitor, ip string) {
	info := sm.attackInfoFor(ip)
	info.mu.Lock()
	info.LastSeen = time.Now().Add(-48 * time.Hour)
	info.mu.Unlock()
}

func liveEntry(t *testing.T, sm *SecurityMonitor, ip string) *AttackInfo {
	t.Helper()
	value, ok := sm.attackMap.Load(ip)
	require.True(t, ok, "the map holds an entry for %s", ip)
	return value.(*AttackInfo)
}

// A tracker that loaded the entry before the cleanup deleted it gets a live entry, not the deleted one.
func TestLockLiveReplacesAnEntryTheCleanupDeleted(t *testing.T) {
	sm := newRulesMonitor(t)
	ip := "10.78.0.1"
	stale := sm.attackInfoFor(ip) // what a tracker loaded
	ageEntry(sm, ip)
	sm.cleanupOldDataOnce(idleCutoff())
	_, ok := sm.attackMap.Load(ip)
	require.False(t, ok, "the cleanup deleted the idle entry")

	got := sm.lockLive(stale, ip)
	got.TotalRequests++ // the request's count
	got.mu.Unlock()

	assert.NotSame(t, stale, got, "the deleted entry is not used")
	live := liveEntry(t, sm, ip)
	assert.Same(t, live, got, "the entry the request used is the one the map holds")
	assert.Equal(t, 1, live.TotalRequests, "the request's count is in the live entry")
	assert.Zero(t, stale.TotalRequests, "and not in the deleted one")
}

// The retry replaces only the deleted entry: other IPs' entries stay the same objects.
func TestLockLiveLeavesOtherEntriesAlone(t *testing.T) {
	sm := newRulesMonitor(t)
	other := sm.attackInfoFor("10.78.0.3")
	stale := sm.attackInfoFor("10.78.0.2")
	ageEntry(sm, "10.78.0.2")
	sm.cleanupOldDataOnce(idleCutoff())

	sm.lockLive(stale, "10.78.0.2").mu.Unlock()

	assert.Same(t, other, liveEntry(t, sm, "10.78.0.3"))
	assert.NotSame(t, stale, liveEntry(t, sm, "10.78.0.2"))
}

// A live entry is used as it is: no replacement, no second entry.
func TestLockLiveKeepsALiveEntry(t *testing.T) {
	sm := newRulesMonitor(t)
	ip := "10.78.0.4"
	info := sm.attackInfoFor(ip)

	got := sm.lockLive(info, ip)
	got.mu.Unlock()

	assert.Same(t, info, got)
	assert.Same(t, info, liveEntry(t, sm, ip))
}

// Through the real entry points: a request after the cleanup deleted the entry starts a live one.
func TestTrackRequestAfterTheCleanupCountsInTheLiveEntry(t *testing.T) {
	sm := newRulesMonitor(t)
	ip := "10.78.0.5"
	sm.TrackRequest(rulesRequest(ip, "/api/v1/devices"), http.StatusOK, time.Millisecond)
	ageEntry(sm, ip)
	sm.cleanupOldDataOnce(idleCutoff())
	sm.TrackRequest(rulesRequest(ip, "/api/v1/devices"), http.StatusOK, time.Millisecond)

	assert.Equal(t, 1, liveEntry(t, sm, ip).TotalRequests)
}

// Where another tracker already stored a replacement for the deleted entry, the retry uses it: there is
// still one entry for the IP, and the count lands there.
func TestLockLiveUsesAReplacementAnotherTrackerStored(t *testing.T) {
	sm := newRulesMonitor(t)
	ip := "10.78.0.6"
	stale := sm.attackInfoFor(ip)
	ageEntry(sm, ip)
	sm.cleanupOldDataOnce(idleCutoff())
	replacement := sm.attackInfoFor(ip) // the other tracker's new entry

	got := sm.lockLive(stale, ip)
	got.mu.Unlock()

	assert.Same(t, replacement, got)
	assert.Same(t, replacement, liveEntry(t, sm, ip))
}
