package configuration

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errGuardTripped = errors.New("guard tripped")

// countingGuard counts its calls and answers with err.
type countingGuard struct {
	calls int
	err   error
}

func (g *countingGuard) check(context.Context) error {
	g.calls++
	return g.err
}

// A guard that fails stops the apply before its first write, on both generations, and the error
// carries the guard's cause and says nothing was written.
func TestApplyConfig_AFailingWriteGuardWritesNothing(t *testing.T) {
	multi := &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("lamp")}},
		MQTT:   &MQTTConfiguration{Enable: BoolPtr(true), Server: StringPtr("broker.local"), Port: IntPtr(1883)},
	}

	t.Run("gen1", func(t *testing.T) {
		client := newApplyTestClient()
		applier := NewConfigApplier(NewGen1Converter(nil), nil)
		guard := &countingGuard{err: errGuardTripped}
		applier.beforeWrite = guard.check

		_, err := applier.ApplyConfig(context.Background(), client, multi, "SHPLG-S")

		require.ErrorIs(t, err, errGuardTripped)
		assert.Contains(t, err.Error(), "nothing was written")
		assert.Equal(t, 1, guard.calls)
		assert.Empty(t, client.configs, "no write reached the device")
	})

	t.Run("gen2", func(t *testing.T) {
		dev := newFakeGen2Device()
		applier := NewConfigApplier(NewGen2Converter(nil), nil)
		guard := &countingGuard{err: errGuardTripped}
		applier.beforeWrite = guard.check

		_, err := applier.ApplyConfig(context.Background(), dev, multi, "SNSW-001X16EU")

		require.ErrorIs(t, err, errGuardTripped)
		assert.Contains(t, err.Error(), "nothing was written")
		assert.Equal(t, 1, guard.calls)
		assert.Empty(t, dev.calls, "no write reached the device")
	})
}

// A passing guard is called once per apply, however many sections it writes, and a nil guard
// changes nothing.
func TestApplyConfig_APassingWriteGuardRunsOncePerApply(t *testing.T) {
	multi := &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("lamp")}},
		MQTT:   &MQTTConfiguration{Enable: BoolPtr(true), Server: StringPtr("broker.local"), Port: IntPtr(1883)},
	}

	t.Run("gen1", func(t *testing.T) {
		client := newApplyTestClient()
		applier := NewConfigApplier(NewGen1Converter(nil), nil)
		guard := &countingGuard{}
		applier.beforeWrite = guard.check

		result, err := applier.ApplyConfig(context.Background(), client, multi, "SHPLG-S")

		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 1, guard.calls)
		assert.NotEmpty(t, client.configs)
	})

	t.Run("gen2", func(t *testing.T) {
		dev := newFakeGen2Device()
		applier := NewConfigApplier(NewGen2Converter(nil), nil)
		guard := &countingGuard{}
		applier.beforeWrite = guard.check

		result, err := applier.ApplyConfig(context.Background(), dev, multi, "SNSW-001X16EU")

		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 1, guard.calls)
		assert.NotEmpty(t, dev.calls)
	})

	t.Run("nil guard", func(t *testing.T) {
		client := newApplyTestClient()
		applier := NewConfigApplier(NewGen1Converter(nil), nil)

		result, err := applier.ApplyConfig(context.Background(), client, multi, "SHPLG-S")

		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.NotEmpty(t, client.configs)
	})
}
