package configuration

import (
	"context"
	"errors"
	"fmt"
)

// ErrUnsupportedGeneration is returned when the device generation has no
// configuration converter yet. Only Gen1 devices are supported for apply and
// verify today.
var ErrUnsupportedGeneration = errors.New("applying configuration is not supported for this device generation")

// ApplyDesiredConfig sends the device's desired configuration to the device,
// then reads it back and compares. The device is marked config_applied only
// when the read-back matches, so a partial or failed apply stays pending.
func (s *ConfigurationService) ApplyDesiredConfig(ctx context.Context, deviceID uint, client ShellyClient, deviceType string) (*ApplyVerifyResult, error) {
	if client.GetGeneration() != 1 {
		return nil, fmt.Errorf("%w: generation %d", ErrUnsupportedGeneration, client.GetGeneration())
	}

	desired, _, err := s.GetDesiredConfig(deviceID)
	if err != nil {
		return nil, err
	}

	verifier := NewConfigVerifier(NewGen1Converter(s.logger), s.logger)
	result, err := verifier.ApplyAndVerify(ctx, client, desired, deviceType)
	if err != nil {
		return nil, err
	}

	if result.ConfigApplied {
		if err := s.repo.SetDeviceConfigApplied(deviceID, true); err != nil {
			return nil, fmt.Errorf("configuration applied but could not record it: %w", err)
		}
	}

	return result, nil
}

// VerifyDesiredConfig reads the live configuration from the device and
// compares it with the desired configuration. It changes nothing on the device.
func (s *ConfigurationService) VerifyDesiredConfig(ctx context.Context, deviceID uint, client ShellyClient, deviceType string) (*VerifyResult, error) {
	if client.GetGeneration() != 1 {
		return nil, fmt.Errorf("%w: generation %d", ErrUnsupportedGeneration, client.GetGeneration())
	}

	desired, _, err := s.GetDesiredConfig(deviceID)
	if err != nil {
		return nil, err
	}

	verifier := NewConfigVerifier(NewGen1Converter(s.logger), s.logger)
	return verifier.VerifyOnly(ctx, client, desired, deviceType)
}
