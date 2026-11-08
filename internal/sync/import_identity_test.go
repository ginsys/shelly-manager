package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateImportIdentitiesRefusesWhatApplyImportRefuses(t *testing.T) {
	cases := map[string]*ExportData{
		"empty device MAC":       {Devices: []DeviceData{{MAC: ""}}},
		"blank device MAC":       {Devices: []DeviceData{{MAC: " \t"}}},
		"empty template name":    {Templates: []TemplateData{{Name: ""}}},
		"blank template name":    {Templates: []TemplateData{{Name: "  "}}},
		"empty discovered MAC":   {DiscoveredDevices: []DiscoveredDeviceData{{MAC: ""}}},
		"blank discovered MAC":   {DiscoveredDevices: []DiscoveredDeviceData{{MAC: "  "}}},
		"blank after a good one": {Devices: []DeviceData{{MAC: "aabbccddeeff"}, {MAC: " "}}},
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateImportIdentities(data)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidImportData)
		})
	}
}

func TestValidateImportIdentitiesAcceptsNamedRecords(t *testing.T) {
	data := &ExportData{
		Devices:           []DeviceData{{MAC: "aabbccddeeff"}},
		Templates:         []TemplateData{{Name: "t1"}},
		DiscoveredDevices: []DiscoveredDeviceData{{MAC: "aabbccddee00"}},
	}
	assert.NoError(t, ValidateImportIdentities(data))
	assert.NoError(t, ValidateImportIdentities(nil))
	assert.NoError(t, ValidateImportIdentities(&ExportData{}))
}
