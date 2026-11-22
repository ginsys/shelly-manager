package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/version"
)

// A template reads the application version from the one variable a build stamps, never a literal.
func TestCreateTemplateContext_SystemVersionIsTheStampedVersion(t *testing.T) {
	logger, err := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	require.NoError(t, err)
	engine := NewTemplateEngine(logger)

	old := version.Version
	t.Cleanup(func() { version.Version = old })
	for _, stamped := range []string{old, "v9.9.9-test"} {
		version.Version = stamped
		assert.Equal(t, stamped, engine.CreateTemplateContext(nil, nil).System.Version)
	}
}
