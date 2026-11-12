package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The capabilities route lists roller for the model the device schema offers a roller section
// for, and for no other: both read configuration.HasRollerMode.
func TestGetDeviceCapabilities_RollerFollowsTheDeviceSchema(t *testing.T) {
	h := &Handler{}

	assert.Contains(t, h.getDeviceCapabilities("SHSW-25", 1), "roller")
	assert.NotContains(t, h.getDeviceCapabilities("SHPLG-S", 1), "roller")
	assert.NotContains(t, h.getDeviceCapabilities("SHSW-1", 1), "roller")
}
