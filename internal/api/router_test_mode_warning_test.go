package api

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// testModeProtections are the protections the test-mode router leaves out.
// The warning must name every one of them (spelled here on purpose, so an
// emptied or renamed list in the router fails the test).
var testModeProtections = []string{
	"security logging", "security headers",
	"request timeout", "rate limiting", "request size limit",
	"request validation", "CORS", "request logging",
}

// captureHandler returns a handler whose logger writes to the returned buffer.
func captureHandler(t *testing.T) (*Handler, *bytes.Buffer) {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)

	buf := &bytes.Buffer{}
	logger := &logging.Logger{Logger: slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))}
	// A real handler: SetupTestModeRoutes reads handler.MetricsHandler without a nil check.
	return NewHandlerWithLogger(db, nil, nil, nil, logger), buf
}

// warnLines returns the WARN lines of the log that mention "test mode", in any case.
func warnLines(buf *bytes.Buffer) []string {
	var out []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "level=WARN") && strings.Contains(strings.ToLower(line), "test mode") {
			out = append(out, line)
		}
	}
	return out
}

func TestTestModeRouterWarnsWithProtections(t *testing.T) {
	t.Setenv(testModeEnv, "true")
	h, buf := captureHandler(t)

	SetupRoutesWithSecurity(h, h.logger, middleware.DefaultSecurityConfig(), middleware.DefaultValidationConfig())

	lines := warnLines(buf)
	require.Len(t, lines, 1, "log:\n%s", buf.String())
	require.Contains(t, lines[0], "TEST MODE ENABLED")
	for _, p := range testModeProtections {
		require.Contains(t, lines[0], p)
	}
}

func TestSetupTestModeRoutesWarnsWhenCalledDirectly(t *testing.T) {
	h, buf := captureHandler(t)

	SetupTestModeRoutes(h, h.logger)

	lines := warnLines(buf)
	require.Len(t, lines, 1, "log:\n%s", buf.String())
	require.Contains(t, lines[0], "TEST MODE ENABLED")
}

func TestProductionRouterLogsNoTestModeWarning(t *testing.T) {
	t.Setenv(testModeEnv, "")
	h, buf := captureHandler(t)

	SetupRoutesWithSecurity(h, h.logger, middleware.DefaultSecurityConfig(), middleware.DefaultValidationConfig())

	require.Empty(t, warnLines(buf), "log:\n%s", buf.String())
}

// The config field (and a loosely parsed env value such as "1") turns test mode on
// for request validation only; the router stays the production one. The warning
// then names exactly the two relaxed checks and none of the router-level ones.
func TestValidationOnlyTestModeWarnsWithRelaxedChecks(t *testing.T) {
	for _, env := range []string{"", "1"} {
		t.Run("env="+env, func(t *testing.T) {
			t.Setenv(testModeEnv, env)
			h, buf := captureHandler(t)
			valCfg := middleware.DefaultValidationConfig()
			valCfg.TestMode = true

			SetupRoutesWithSecurity(h, h.logger, middleware.DefaultSecurityConfig(), valCfg)

			lines := warnLines(buf)
			require.Len(t, lines, 1, "log:\n%s", buf.String())
			require.Contains(t, lines[0], "suspicious header")
			require.Contains(t, lines[0], "User-Agent")
			require.NotContains(t, lines[0], "TEST MODE ENABLED")
			require.NotContains(t, lines[0], "rate limiting")
		})
	}
}

// With the env var on, the test-mode router is returned before the validation
// config is read, so a true field must not add a second warning.
func TestTestModeRouterWarnsOnceWhenFieldIsAlsoOn(t *testing.T) {
	t.Setenv(testModeEnv, "true")
	h, buf := captureHandler(t)
	valCfg := middleware.DefaultValidationConfig()
	valCfg.TestMode = true

	SetupRoutesWithSecurity(h, h.logger, middleware.DefaultSecurityConfig(), valCfg)

	require.Len(t, warnLines(buf), 1, "log:\n%s", buf.String())
}
