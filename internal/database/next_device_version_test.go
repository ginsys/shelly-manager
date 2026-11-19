package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The next version is the clock, and never less than a millisecond past the stored one: the rule
// that UpdateDeviceIfVersion and the column writers share (ginsys/shelly-manager#580).
func TestNextDeviceVersion(t *testing.T) {
	t.Run("a stored version in the future gives exactly a millisecond past it", func(t *testing.T) {
		stored := time.Now().Add(time.Hour)
		assert.Equal(t, stored.Add(time.Millisecond), nextDeviceVersion(stored))
	})

	t.Run("a stored version at the clock gives at least a millisecond past it", func(t *testing.T) {
		stored := time.Now()
		got := nextDeviceVersion(stored)
		assert.False(t, got.Before(stored.Add(time.Millisecond)), "next %v is before stored+1ms %v", got, stored.Add(time.Millisecond))
	})

	t.Run("a stored version long ago gives the clock", func(t *testing.T) {
		before := time.Now()
		got := nextDeviceVersion(before.Add(-time.Hour))
		after := time.Now()
		assert.False(t, got.Before(before), "next %v is before the call started %v", got, before)
		assert.False(t, got.After(after), "next %v is after the call ended %v", got, after)
	})
}
