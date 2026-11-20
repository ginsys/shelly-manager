package configuration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestSubstituteVariables_InvalidJSONLogCarriesNoRenderedPassword(t *testing.T) {
	// The password is rendered unquoted, so the output is not valid JSON and the engine logs the failure.
	// The rendered output holds the password, so it must not reach the log.
	logFile := filepath.Join(t.TempDir(), "template.log")
	logger, err := logging.New(logging.Config{Level: logging.LevelDebug, Format: "json", Output: logFile})
	require.NoError(t, err)
	engine := NewTemplateEngine(logger)

	ctx := &TemplateContext{}
	ctx.Auth.Password = "s3cret-marker"

	_, subErr := engine.SubstituteVariables(json.RawMessage(`{"auth": {"pass": {{.Auth.Password}}}}`), ctx)

	require.Error(t, subErr)
	assert.NotContains(t, subErr.Error(), "s3cret-marker")
	logged, readErr := os.ReadFile(logFile)
	require.NoError(t, readErr)
	require.Contains(t, string(logged), "Template substitution resulted in invalid JSON", "the control: the failure was logged at all")
	assert.NotContains(t, string(logged), "s3cret-marker")
}
