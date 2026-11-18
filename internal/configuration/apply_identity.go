package configuration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// ErrDeviceIdentity is returned when a write to a device is refused because the device at the
// stored address cannot be shown to be the stored device: its MAC differs, it reports none, or the
// record has none. Nothing is sent to the device in that case.
var ErrDeviceIdentity = errors.New("the device at the stored address is not the stored device")

// identityReader is the part of a device client that tells which device answers.
type identityReader interface {
	GetInfo(ctx context.Context) (*shelly.DeviceInfo, error)
}

// normalizeMAC reduces a MAC to its hex digits in upper case, so that AA:BB:CC:DD:EE:FF,
// aa-bb-cc-dd-ee-ff and AABBCCDDEEFF compare equal.
func normalizeMAC(mac string) string {
	return strings.ToUpper(strings.NewReplacer(":", "", "-", "", ".", "", " ", "", "\t", "").Replace(mac))
}

// checkDeviceIdentity reads the device's identity through client and refuses unless it is the
// stored device. A record with no MAC is refused without a device call.
func checkDeviceIdentity(ctx context.Context, client identityReader, storedMAC string) error {
	if normalizeMAC(storedMAC) == "" {
		return checkReportedMAC(storedMAC, "")
	}
	info, err := client.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("could not read the device's identity before writing: %w", err)
	}
	return checkReportedMAC(storedMAC, reportedMAC(info))
}

func reportedMAC(info *shelly.DeviceInfo) string {
	if info == nil {
		return ""
	}
	return info.MAC
}

// checkReportedMAC compares the stored MAC with the one the device reported. The messages name
// the fix, because the caller sees nothing else of the refusal.
func checkReportedMAC(storedMAC, reportedMAC string) error {
	stored, reported := normalizeMAC(storedMAC), normalizeMAC(reportedMAC)
	switch {
	case stored == "":
		return fmt.Errorf("%w: the device record has no MAC address, so the device at this address "+
			"cannot be identified; set it with PUT /api/v1/devices/{id} (field mac)", ErrDeviceIdentity)
	case reported == "":
		return fmt.Errorf("%w: the device did not report a MAC address", ErrDeviceIdentity)
	case stored != reported:
		return fmt.Errorf("%w: the device at this address answers with MAC %s but the stored MAC is %s; "+
			"run discovery (POST /api/v1/discover) to move the record to the device's current address",
			ErrDeviceIdentity, reported, stored)
	}
	return nil
}
