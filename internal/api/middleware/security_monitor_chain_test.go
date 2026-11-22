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

// The monitor's block rules, tested through the middleware that production runs
// (ginsys/shelly-manager#666). The chain copies the order of internal/api/router.go: step 2 IP
// blocking, step 3 security monitoring, step 7 rate limiting, then the handler. The real router cannot
// be built here, because SetupRoutesWithSecurity needs a MetricsHandler.

const (
	chainBadPath    = "/api/v1/bad"            // the handler answers 400
	chainNormalPath = "/api/v1/devices"        // the handler answers 200
	chainBulkPath   = "/api/v1/config/bulk-go" // a path with the bulk limit of 20 an hour
	chainUserAgent  = "Mozilla/5.0 (X11; Linux x86_64)"
)

type monitorChain struct {
	handler http.Handler
	monitor *SecurityMonitor
}

func newMonitorChain(t *testing.T) *monitorChain {
	t.Helper()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text", Output: "stdout"})
	require.NoError(t, err)
	cfg := DefaultSecurityConfig()
	monitor := NewSecurityMonitor(cfg, logger)
	t.Cleanup(monitor.Close)

	app := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == chainBadPath {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := IPBlockingMiddleware(cfg, monitor, logger)(
		MonitoringMiddleware(cfg, monitor, logger)(
			RateLimitMiddleware(cfg, logger)(app)))
	return &monitorChain{handler: handler, monitor: monitor}
}

// get sends one GET from ip to path and returns the status.
func (c *monitorChain) get(ip, path string) int {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = ip + ":4000"
	req.Header.Set("User-Agent", chainUserAgent)
	rr := httptest.NewRecorder()
	c.handler.ServeHTTP(rr, req)
	return rr.Code
}

func (c *monitorChain) getN(ip, path string, n int) {
	for i := 0; i < n; i++ {
		c.get(ip, path)
	}
}

func TestMonitorOneBadRequestDoesNotBlockTheClient(t *testing.T) {
	c := newMonitorChain(t)
	ip := "10.66.0.1"

	assert.Equal(t, http.StatusBadRequest, c.get(ip, chainBadPath))
	assert.Equal(t, http.StatusOK, c.get(ip, chainNormalPath), "one 400 is no reason to block every route")
}

func TestMonitorPathRateLimitRefusalDoesNotBlockTheClient(t *testing.T) {
	c := newMonitorChain(t)
	ip := "10.66.0.2"

	c.getN(ip, chainBulkPath, 20)
	assert.Equal(t, http.StatusTooManyRequests, c.get(ip, chainBulkPath), "the 21st bulk call is refused")
	assert.Equal(t, http.StatusOK, c.get(ip, chainNormalPath), "one refusal blocks no other route")
}

func TestMonitorBlocksAfterFiveRateLimitRefusals(t *testing.T) {
	c := newMonitorChain(t)
	ip := "10.66.0.3"

	c.getN(ip, chainBulkPath, 20)
	c.getN(ip, chainBulkPath, 4) // refusals 1 to 4
	assert.Equal(t, http.StatusOK, c.get(ip, chainNormalPath), "4 refusals stay under the threshold of 5")

	assert.Equal(t, http.StatusTooManyRequests, c.get(ip, chainBulkPath)) // refusal 5
	assert.Equal(t, http.StatusForbidden, c.get(ip, chainNormalPath), "the 5th refusal blocks the IP")
}

func TestMonitorBlocksOnlyAfterSixSuspiciousRequestsInAnHour(t *testing.T) {
	c := newMonitorChain(t)
	ip := "10.66.0.4"

	c.getN(ip, chainBadPath, 5)
	assert.Equal(t, http.StatusOK, c.get(ip, chainNormalPath), "5 suspicious requests are not more than 5 an hour")

	assert.Equal(t, http.StatusBadRequest, c.get(ip, chainBadPath)) // the 6th
	assert.Equal(t, http.StatusForbidden, c.get(ip, chainNormalPath), "the 6th inside the hour blocks")
}

// A block starts the IP's counting over (ginsys/shelly-manager#861, Serge's answer: reset on block).
// Where the block ends, the counters that caused it are gone, so the IP is served and one more
// refusal does not block it again; it takes a fresh 5 refusals. Before that change the counters
// stayed and the next tracked request blocked the IP again.
func TestMonitorBlockEndsWithAFreshCount(t *testing.T) {
	c := newMonitorChain(t)
	ip := "10.66.0.5"

	c.getN(ip, chainBulkPath, 25)
	require.Equal(t, http.StatusForbidden, c.get(ip, chainNormalPath), "blocked after 5 refusals")

	value, ok := c.monitor.attackMap.Load(ip)
	require.True(t, ok)
	info := value.(*AttackInfo)
	info.mu.Lock()
	info.BlockedUntil = time.Now().Add(-time.Minute) // the hour has passed
	info.mu.Unlock()

	assert.Equal(t, http.StatusOK, c.get(ip, chainNormalPath), "the block ended, this request is served")
	assert.Equal(t, http.StatusTooManyRequests, c.get(ip, chainBulkPath), "one more refusal is served as a refusal")
	assert.Equal(t, http.StatusOK, c.get(ip, chainNormalPath),
		fmt.Sprintf("and the counters (%d violations) do not block the IP again", info.RateLimitViolations))
}
