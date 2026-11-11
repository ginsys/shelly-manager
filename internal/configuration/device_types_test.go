package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The list the API returns is the list the converter supports. A model added to the converter
// must get a readable name here, or this test names it.
func TestDeviceTypeInfosCoverEverySupportedType(t *testing.T) {
	converter := NewGen1Converter(nil)
	infos := SupportedDeviceTypeInfos()

	var codes []string
	for _, info := range infos {
		codes = append(codes, info.DeviceType)
		assert.NotEmpty(t, info.Name, info.DeviceType+" has no readable name")
		assert.Equal(t, converter.Generation(), info.Generation, info.DeviceType)
	}
	assert.Equal(t, converter.SupportedDeviceTypes(), codes)
}

func TestIsSupportedDeviceType(t *testing.T) {
	for _, deviceType := range NewGen1Converter(nil).SupportedDeviceTypes() {
		assert.True(t, IsSupportedDeviceType(deviceType), deviceType)
	}
	require.False(t, IsSupportedDeviceType(""))
	require.False(t, IsSupportedDeviceType("SHXX-NOPE"))
	require.False(t, IsSupportedDeviceType("shplg-s"), "a code is matched exactly")
}
