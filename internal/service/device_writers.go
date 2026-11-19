package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/ginsys/shelly-manager/internal/database"
)

// The writers in this file hold clientMu, reload the row and apply only their own change, so they
// cannot put an older login back over one that UpdateDeviceAuth saved after the writer's first read
// (ginsys/shelly-manager#496). UpdateDeviceKeepingLogin saves the device's own fields, versioned,
// for the device PUT. The others write only the columns they own (ginsys/shelly-manager#565).

// deviceLoginKeys are the Settings keys that hold the login of a device.
var deviceLoginKeys = []string{"auth_enabled", "auth_user", "auth_pass", "auth"}

// decodeSettings returns the document in a Settings text. An empty text is an empty document.
func decodeSettings(text string) (map[string]any, error) {
	settings := map[string]any{}
	if text == "" {
		return settings, nil
	}
	if err := json.Unmarshal([]byte(text), &settings); err != nil {
		return nil, err
	}
	if settings == nil {
		settings = map[string]any{}
	}
	return settings, nil
}

// sameLogin reports whether two settings documents hold the same login. A missing auth_enabled
// reads as false, because the validation of a device PUT adds that default.
func sameLogin(a, b map[string]any) bool {
	for _, key := range deviceLoginKeys {
		av, aok := a[key]
		bv, bok := b[key]
		if key == "auth_enabled" {
			if !aok {
				av = false
			}
			if !bok {
				bv = false
			}
		} else if aok != bok {
			return false
		}
		if !reflect.DeepEqual(av, bv) {
			return false
		}
	}
	return true
}

// keepStoredLogin returns the incoming settings with the login of the stored settings, when the
// incoming login equals the login of basis, the record that the incoming settings came from. An
// incoming login that differs from basis is a change on purpose and stays.
func keepStoredLogin(incoming, basis, stored string) (string, error) {
	in, err := decodeSettings(incoming)
	if err != nil {
		return "", err
	}
	base, err := decodeSettings(basis)
	if err != nil {
		return incoming, nil
	}
	current, err := decodeSettings(stored)
	if err != nil {
		return incoming, nil
	}
	if !sameLogin(in, base) {
		return incoming, nil
	}
	for _, key := range deviceLoginKeys {
		if value, ok := current[key]; ok {
			in[key] = value
		} else {
			delete(in, key)
		}
	}
	merged, err := json.Marshal(in)
	if err != nil {
		return "", err
	}
	return string(merged), nil
}

// ErrDeviceChanged means a save carried an updated_at that is not the stored one: the device was
// saved after the client read it (ginsys/shelly-manager#502).
var ErrDeviceChanged = errors.New("the device changed after it was read")

// UpdateDeviceKeepingLogin saves the incoming device row for a plain device PUT. basis is the
// stored record that the request was read against.
//
// A request that carries updated_at is checked against the stored one. If that time is not the
// stored one, the write does nothing and returns ErrDeviceChanged. A request with no updated_at is
// an older client or a script: its save is versioned on the stored row. Where it leaves the login
// as it was in basis, the row keeps the login that is stored now. A request that changes the login
// saves it.
func (s *ShellyService) UpdateDeviceKeepingLogin(basis, incoming *database.Device) error {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	stored, err := s.DB.GetDevice(incoming.ID)
	if err != nil {
		return err
	}
	if !incoming.UpdatedAt.IsZero() && !incoming.UpdatedAt.Equal(stored.UpdatedAt) {
		return ErrDeviceChanged
	}
	if stored.Settings != basis.Settings {
		merged, mergeErr := keepStoredLogin(incoming.Settings, basis.Settings, stored.Settings)
		if mergeErr != nil {
			return fmt.Errorf("failed to merge device settings: %w", mergeErr)
		}
		incoming.Settings = merged
	}
	// The compare above is not enough: a writer that does not take clientMu can commit before this
	// save. So the save repeats the compare in its own UPDATE, against the value that the database
	// gave us (ginsys/shelly-manager#509). A body with no updated_at is saved the same way, against
	// the stored row (ginsys/shelly-manager#572). The save never writes template_ids, overrides,
	// desired_config or config_applied: each has its own endpoint.
	err = s.DB.UpdateDeviceIfVersion(incoming, stored.UpdatedAt)
	if errors.Is(err, database.ErrDeviceVersionChanged) {
		return ErrDeviceChanged
	}
	if err != nil {
		return err
	}
	// The caller returns this object, and the client sends its updated_at back. The save stamps
	// updated_at in memory with the clock's full precision, and a database can store less
	// (PostgreSQL keeps microseconds), so the next PUT would be a false 409. A writer that does
	// not take clientMu can also save between the save and this read. So the whole row comes
	// from this one read, and its fields and its version always belong together.
	saved, err := s.DB.GetDevice(incoming.ID)
	if err != nil {
		return fmt.Errorf("device saved, but reading it back failed: %w", err)
	}
	*incoming = *saved
	return nil
}

// backfillDiscoverySettings stores the model, generation and auth flag that a discovery found in
// the device's settings, and keeps every other key, the login included. It updates device to the
// row that it wrote.
func (s *ShellyService) backfillDiscoverySettings(device *database.Device, model string, generation int, authEnabled bool) error {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	record, err := s.DB.GetDevice(device.ID)
	if err != nil {
		return err
	}
	// A settings text that does not parse starts a new document, as it always did here.
	settings, decodeErr := decodeSettings(record.Settings)
	if decodeErr != nil {
		settings = map[string]any{}
	}
	// The text as stored, in the form that json.Marshal writes it: a pass that changes no key and
	// no value writes nothing, so it leaves updated_at, the version of the device PUT, alone
	// (ginsys/shelly-manager#583). A text that does not parse never equals the new one.
	var unchanged []byte
	if decodeErr == nil {
		unchanged, _ = json.Marshal(settings)
	}
	settings["model"] = model
	settings["gen"] = generation
	settings["auth_enabled"] = authEnabled
	if _, ok := settings["auth_user"]; !ok {
		settings["auth_user"] = ""
	}
	if _, ok := settings["auth_pass"]; !ok {
		settings["auth_pass"] = ""
	}
	updated, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	if decodeErr == nil && bytes.Equal(unchanged, updated) {
		*device = *record
		return nil
	}
	if err = s.DB.UpdateDeviceSettings(record.ID, string(updated)); err != nil {
		return err
	}
	// The save stamps updated_at in the database, so the row comes back from one read.
	saved, err := s.DB.GetDevice(record.ID)
	if err != nil {
		return err
	}
	*device = *saved
	return nil
}
