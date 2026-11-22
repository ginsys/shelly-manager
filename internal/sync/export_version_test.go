package sync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/version"
)

// An export says which application version made it, from the one variable a build stamps.
func TestLoadExportData_MetadataCarriesTheStampedVersion(t *testing.T) {
	engine, _ := findResultEngine(t)

	old := version.Version
	t.Cleanup(func() { version.Version = old })
	for _, stamped := range []string{old, "v9.9.9-test"} {
		version.Version = stamped
		data, err := engine.loadExportData(context.Background(), ExportFilters{})
		require.NoError(t, err)
		assert.Equal(t, stamped, data.Metadata.SystemVersion)
	}
}
