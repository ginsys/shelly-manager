package configuration

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// anotherMAC is a MAC that is not testDeviceMAC, in the colon spelling a record may hold.
const anotherMAC = "11:22:33:44:55:66"

// identityRig is one generation's apply fixture: a service whose record stores testDeviceMAC, a
// fake device that reports it, and a count of everything the device was sent.
type identityRig struct {
	svc        *ConfigurationService
	repo       *mockRepository
	client     ShellyClient
	device     *applyTestClient // the identity the fake reports: mac, infoErr, infoNil
	deviceType string
	writes     func() int
}

var identityRigs = map[string]func(t *testing.T) identityRig{
	"gen1": func(t *testing.T) identityRig {
		svc, repo := newServiceWithDesiredName(t, "tv-office")
		dev := newFakeGen1Device()
		return identityRig{svc: svc, repo: repo, client: dev, device: &dev.applyTestClient, deviceType: "SHPLG-S",
			writes: func() int {
				return dev.setCalls + len(dev.relayWrites) + len(dev.cloudWrites) + len(dev.loginWrites) + dev.reboots
			}}
	},
	"gen2": func(t *testing.T) identityRig {
		svc, repo := newGen2Service(t, systemDesired())
		dev := newFakeGen2Device()
		return identityRig{svc: svc, repo: repo, client: dev, device: &dev.applyTestClient, deviceType: "SNSW-001X16EU",
			writes: func() int { return len(dev.calls) + dev.legacyCalls + dev.reboots }}
	},
}

// forEachGeneration runs a case once per generation, each on a fresh fixture.
func forEachGeneration(t *testing.T, fn func(t *testing.T, rig identityRig)) {
	t.Helper()
	for name, build := range identityRigs {
		t.Run(name, func(t *testing.T) { fn(t, build(t)) })
	}
}

func (r identityRig) apply(t *testing.T) (*ApplyVerifyResult, error) {
	t.Helper()
	return r.svc.ApplyDesiredConfig(context.Background(), 1, r.client, r.deviceType)
}

// A device that answers at the stored address with another MAC is another device: nothing is
// written, and neither the applied flag nor the owed reboot changes, whichever state it was in.
func TestApplyDesiredConfig_RefusesADeviceWithAnotherMAC(t *testing.T) {
	for _, applied := range []bool{false, true} {
		t.Run(fmt.Sprintf("applied=%t", applied), func(t *testing.T) {
			forEachGeneration(t, func(t *testing.T, rig identityRig) {
				rig.repo.devices[1].MAC = anotherMAC
				rig.repo.devices[1].ConfigApplied = applied
				rig.repo.devices[1].PendingReboot = true

				result, err := rig.apply(t)

				require.Error(t, err)
				assert.Nil(t, result)
				assert.ErrorIs(t, err, ErrDeviceIdentity)
				assert.Contains(t, err.Error(), testDeviceMAC, "the MAC the device reported")
				assert.Contains(t, err.Error(), "112233445566", "the stored MAC, normalised")
				assert.Contains(t, err.Error(), "POST /api/v1/discover", "the message names the fix")
				assert.Equal(t, 0, rig.writes(), "nothing is sent to the other device")
				assert.Equal(t, applied, rig.repo.devices[1].ConfigApplied, "the flag stays as it was")
				assert.True(t, rig.repo.devices[1].PendingReboot, "an owed reboot stays owed")
			})
		})
	}
}

// Case, separators and dots do not make two MACs different.
func TestApplyDesiredConfig_AppliesWhenTheMACMatchesInAnySpelling(t *testing.T) {
	for _, stored := range []string{"aa:bb:cc:dd:ee:ff", "AA-BB-CC-DD-EE-FF", "aabbccddeeff", "AABB.CCDD.EEFF", " AA:BB:CC:DD:EE:FF "} {
		t.Run(stored, func(t *testing.T) {
			forEachGeneration(t, func(t *testing.T, rig identityRig) {
				rig.repo.devices[1].MAC = stored

				result, err := rig.apply(t)

				require.NoError(t, err)
				assert.True(t, result.ConfigApplied)
				assert.Greater(t, rig.writes(), 0)
				assert.True(t, rig.repo.devices[1].ConfigApplied)
			})
		})
	}
}

// A device that cannot be told from another one is not written to: no stored MAC, a stored MAC of
// blanks, a device that reports none, one that answers nothing, and one that fails the read.
func TestApplyDesiredConfig_RefusesADeviceItCannotIdentify(t *testing.T) {
	cases := []struct {
		name     string
		set      func(r identityRig)
		identity bool // the refusal is ErrDeviceIdentity
		contains string
	}{
		{"no stored MAC", func(r identityRig) { r.repo.devices[1].MAC = "" }, true, "PUT /api/v1/devices/{id}"},
		{"a stored MAC of blanks", func(r identityRig) { r.repo.devices[1].MAC = "  " }, true, "PUT /api/v1/devices/{id}"},
		{"the device reports no MAC", func(r identityRig) { r.device.mac = "" }, true, "did not report"},
		{"GetInfo answers nothing", func(r identityRig) { r.device.infoNil = true }, true, "did not report"},
		{"GetInfo fails", func(r identityRig) { r.device.infoErr = errors.New("connection refused") }, false, "could not read the device's identity"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			forEachGeneration(t, func(t *testing.T, rig identityRig) {
				rig.repo.devices[1].ConfigApplied = true
				tc.set(rig)

				_, err := rig.apply(t)

				require.Error(t, err)
				assert.Equal(t, tc.identity, errors.Is(err, ErrDeviceIdentity))
				assert.Contains(t, err.Error(), tc.contains)
				assert.Equal(t, 0, rig.writes())
				assert.True(t, rig.repo.devices[1].ConfigApplied, "the flag stays as it was")
			})
		})
	}
}

// A device that rejects the stored login is an auth failure to the caller (502), not an identity
// refusal, so the read error stays unwrappable.
func TestApplyDesiredConfig_AnAuthFailureOnTheIdentityReadStaysAnAuthFailure(t *testing.T) {
	forEachGeneration(t, func(t *testing.T, rig identityRig) {
		rig.device.infoErr = fmt.Errorf("401: %w", shelly.ErrAuthFailed)

		_, err := rig.apply(t)

		require.Error(t, err)
		assert.True(t, shelly.IsAuthError(err))
		assert.False(t, errors.Is(err, ErrDeviceIdentity))
		assert.Equal(t, 0, rig.writes())
	})
}

// With login hooks the apply writes through the client the hooks build, so that is the client whose
// identity counts: the one handed in may be another device's.
func TestApplyDesiredConfig_ReadsTheIdentityThroughTheClientThatWrites(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	handedIn := newFakeGen1Device() // reports the stored MAC
	writer := newFakeGen1Device()
	writer.mac = "112233445566"
	svc.SetLoginHooks(&fakeLoginHooks{dev: writer})

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, handedIn, "SHPLG-S")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDeviceIdentity)
	assert.Equal(t, 0, writer.setCalls, "the client that would write belongs to another device")
	assert.Equal(t, 0, handedIn.setCalls)
	assert.False(t, repo.devices[1].ConfigApplied)
}

// A refused identity builds the client, reads, and stops: no login is stored and none is written.
func TestApplyDesiredConfig_AMismatchStoresNoLoginAndWritesNone(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	repo.devices[1].MAC = anotherMAC
	setDesired(t, repo, loginDesired(BoolPtr(true), StringPtr("admin"), StringPtr(testLoginPassword)))
	dev := newFakeGen1Device()
	hooks := &fakeLoginHooks{dev: dev}
	svc.SetLoginHooks(hooks)

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDeviceIdentity)
	assert.Equal(t, []string{"client"}, hooks.calls)
	assert.Empty(t, hooks.stored)
	assert.Empty(t, dev.loginWrites)
	assert.False(t, dev.loginOn)
}

// The client is built before the applied flag is cleared, so a client that cannot be built writes
// nothing and leaves the flag as it was (it used to leave it cleared).
func TestApplyDesiredConfig_AClientThatCannotBeBuiltLeavesTheFlagAlone(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	repo.devices[1].ConfigApplied = true
	dev := newFakeGen1Device()
	svc.SetLoginHooks(&fakeLoginHooks{dev: dev, clientErr: errors.New("no credentials")})

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not build a client")
	assert.Equal(t, 0, dev.setCalls)
	assert.True(t, repo.devices[1].ConfigApplied)
}

func TestGormConfigRepository_ReadsTheStoredMAC(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Create(&Device{ID: 1, Name: "plug", Type: "SHPLG-S", IP: "192.168.1.9", MAC: "AA:BB:CC:DD:EE:FF"}).Error)
	repo := NewGormConfigRepositoryOn(db, nil)

	one, err := repo.GetDevice(1)
	require.NoError(t, err)
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", one.MAC)

	all, err := repo.GetDevices()
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", all[0].MAC)
}

// exportFixture stores a device with the given MAC and a configuration to export to it.
func exportFixture(t *testing.T, storedMAC string) (*Service, *gorm.DB) {
	t.Helper()
	service, db := setupTestService(t)
	require.NoError(t, db.Create(&Device{ID: 1, Name: "plug", Type: "SHSW-1", IP: "192.168.1.100", MAC: storedMAC}).Error)
	require.NoError(t, db.Create(&DeviceConfig{DeviceID: 1, Config: []byte(`{"name": "TestDevice"}`), SyncStatus: "pending"}).Error)
	return service, db
}

// Export writes the stored configuration with no apply behind it, so it owes the same check: a
// device that answers with another MAC, or that cannot be identified, gets no SetConfig, and the
// stored sync status and history stay as they were.
func TestExportToDevice_RefusesADeviceItCannotIdentify(t *testing.T) {
	cases := []struct {
		name     string
		stored   string
		info     *shelly.DeviceInfo
		contains string
	}{
		{"another MAC, Gen1", "AA:BB:CC:DD:EE:FF", &shelly.DeviceInfo{MAC: "112233445566", Generation: 1}, "POST /api/v1/discover"},
		{"another MAC, Gen2", "AA:BB:CC:DD:EE:FF", &shelly.DeviceInfo{MAC: "112233445566", Generation: 2}, "POST /api/v1/discover"},
		{"no stored MAC", "", &shelly.DeviceInfo{MAC: testDeviceMAC, Generation: 1}, "PUT /api/v1/devices/{id}"},
		{"the device reports no MAC", "AA:BB:CC:DD:EE:FF", &shelly.DeviceInfo{Generation: 1}, "did not report"},
		{"GetInfo answers nothing", "AA:BB:CC:DD:EE:FF", nil, "did not report"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, db := exportFixture(t, tc.stored)
			client := new(mockShellyClient)
			// A device that is written to accepts the write; the assertion below is what refuses it.
			client.On("SetConfig", mock.Anything, mock.Anything).Return(nil).Maybe()
			if tc.info == nil {
				client.On("GetInfo", mock.Anything).Return(nil, nil)
			} else {
				client.On("GetInfo", mock.Anything).Return(tc.info, nil)
			}

			err := service.ExportToDevice(1, client)

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrDeviceIdentity)
			assert.Contains(t, err.Error(), tc.contains)
			client.AssertNotCalled(t, "SetConfig", mock.Anything, mock.Anything)

			var stored DeviceConfig
			require.NoError(t, db.Where("device_id = ?", 1).First(&stored).Error)
			assert.Equal(t, "pending", stored.SyncStatus)
			assert.Nil(t, stored.LastSynced)
			var history []ConfigHistory
			require.NoError(t, db.Where("device_id = ?", 1).Find(&history).Error)
			assert.Empty(t, history)
		})
	}
}

// A GetInfo failure is the device's own error, not an identity refusal.
func TestExportToDevice_AGetInfoFailureIsNotAnIdentityRefusal(t *testing.T) {
	service, _ := exportFixture(t, "AA:BB:CC:DD:EE:FF")
	client := new(mockShellyClient)
	client.On("SetConfig", mock.Anything, mock.Anything).Return(nil).Maybe()
	client.On("GetInfo", mock.Anything).Return(nil, fmt.Errorf("401: %w", shelly.ErrAuthFailed))

	err := service.ExportToDevice(1, client)

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrDeviceIdentity))
	assert.True(t, shelly.IsAuthError(err))
	client.AssertNotCalled(t, "SetConfig", mock.Anything, mock.Anything)
}
