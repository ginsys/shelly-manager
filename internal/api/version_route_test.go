package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The UI's axios client prefixes every call with /api/v1, so the version check in App.vue and
// BackupManagementPage reaches GET /api/v1/version. The root GET /version stays for health
// tooling. Both must answer through the production router.
func TestVersionIsServedUnderAPIAndRoot(t *testing.T) {
	t.Setenv("SHELLY_SECURITY_VALIDATION_TEST_MODE", "")
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	h := NewHandlerWithLogger(db, nil, testNotificationHandler(t, db), nil, logger)
	router := SetupRoutesWithSecurity(h, logger, middleware.DefaultSecurityConfig(), middleware.DefaultValidationConfig())
	sm, ok := h.securityMonitor.(*middleware.SecurityMonitor)
	require.True(t, ok, "production router did not build a security monitor")
	t.Cleanup(sm.Close)

	for _, path := range []string{"/api/v1/version", "/version"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		// The security middleware rejects an empty user agent (ginsys/shelly-manager#282).
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) shelly-manager-test")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "%s: %s", path, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"api_version"`, path)
	}
}
