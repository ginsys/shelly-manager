package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
)

// The typed read of a first-generation document is pinned byte for byte, so a change that adds a
// second-generation reader cannot move it. The inputs are real-shaped Gen1 /settings documents, and
// each golden file is the answer {configuration, conversion_info} that the read gave before the change.
// Run with UPDATE_GOLDEN=1 to write the golden files.

type typedGolden struct {
	Configuration  any `json:"configuration"`
	ConversionInfo any `json:"conversion_info"`
}

// The Gen1 warning that lists the keys it stored in the raw part writes them in map order, which
// differs from run to run. The golden holds that one list sorted, so it compares everything else
// byte for byte; the order of the list is not part of what is pinned.
func sortRawKeyWarnings(info *ConfigurationConversionInfo) {
	const prefix = "Unconverted settings stored in raw field: "
	for i, w := range info.Warnings {
		if rest, ok := strings.CutPrefix(w, prefix); ok {
			keys := strings.Split(rest, ", ")
			sort.Strings(keys)
			info.Warnings[i] = prefix + strings.Join(keys, ", ")
		}
	}
}

func TestConvertToTypedConfig_Gen1Golden(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		device *database.Device
	}{
		{"shsw25", "shsw25.settings.json", &database.Device{
			Type: "SHSW-25", Firmware: "20230913-112316/v1.14.0-gcb84623",
			Settings: `{"model":"SHSW-25","gen":1,"auth_enabled":true}`}},
		{"shplg-s", "shplg-s.settings.json", &database.Device{
			Type: "SHPLG-S", Firmware: "20230913-112234/v1.14.0-gcb84623",
			Settings: `{"model":"SHPLG-S","gen":1,"auth_enabled":false}`}},
		// What POST /config/convert-to-typed builds when the request names a model and a generation.
		{"shsw25-convert-generation-1", "shsw25.settings.json", &database.Device{Type: "SHSW-25", Firmware: "v1.0.0"}},
		// What it builds when the request names neither: no settings and no firmware.
		{"shsw25-convert-no-generation", "shsw25.settings.json", &database.Device{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "typed_config", c.input))
			require.NoError(t, err)

			// A first-generation document has no top-level sys object: the second-generation reader is
			// chosen by that object, so a fixture that had one would not be a Gen1 fixture.
			var doc map[string]any
			require.NoError(t, json.Unmarshal(input, &doc))
			assert.NotContains(t, doc, "sys")

			h := &Handler{}
			typed, info, err := h.convertToTypedConfig(input, c.device)
			require.NoError(t, err)
			sortRawKeyWarnings(info)
			got, err := json.MarshalIndent(typedGolden{typed, info}, "", "  ")
			require.NoError(t, err)
			got = append(got, '\n')

			path := filepath.Join("testdata", "typed_config", c.name+".golden.json")
			if os.Getenv("UPDATE_GOLDEN") != "" {
				require.NoError(t, os.WriteFile(path, got, 0o644))
			}
			want, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, string(want), string(got))
		})
	}
}
