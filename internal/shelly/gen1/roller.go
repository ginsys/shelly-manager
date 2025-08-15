package gen1

import (
	"context"
	"fmt"
	"time"
)

// RollerPosition represents the position of a roller shutter
type RollerPosition struct {
	CurrentPos    int     `json:"current_pos"`    // 0-100 (0=closed, 100=open)
	State         string  `json:"state"`          // "open", "close", "stop"
	Power         float64 `json:"power"`          // Power consumption
	IsValid       bool    `json:"is_valid"`       // Power measurement valid
	LastDirection string  `json:"last_direction"` // Last movement direction
	Calibrated    bool    `json:"calibrated"`     // Calibration status
	Positioning   bool    `json:"positioning"`    // Currently positioning
}

// RollerSettings represents roller configuration
type RollerSettings struct {
	MaxTime           int    `json:"maxtime"`            // Max time for full open/close (seconds)
	DefaultState      string `json:"default_state"`      // "open", "close", "stop"
	SwapInputs        bool   `json:"swap"`               // Swap input buttons
	SwapOutputs       bool   `json:"swap_outputs"`       // Swap motor outputs
	InputMode         string `json:"input_mode"`         // "openclose" or "onebutton"
	ButtonType        string `json:"button_type"`        // "momentary", "toggle", "detached"
	FavPos            int    `json:"fav_pos"`            // Favorite position 0-100
	ObstructionDetect bool   `json:"obstruction_detect"` // Obstruction detection
	ObstructionAction string `json:"obstruction_action"` // "stop" or "reverse"
	ObstructionPower  int    `json:"obstruction_power"`  // Power threshold for obstruction
	SafetySwitch      bool   `json:"safety_switch"`      // Safety switch enabled
}

// SetRollerPosition sets the roller shutter to a specific position
func (c *Client) SetRollerPosition(ctx context.Context, channel int, position int) error {
	// Ensure position is within valid range
	if position < 0 {
		position = 0
	} else if position > 100 {
		position = 100
	}

	url := fmt.Sprintf("http://%s/roller/%d", c.ip, channel)
	params := map[string]interface{}{
		"go":         "to_pos",
		"roller_pos": position,
	}
	return c.postForm(ctx, url, params)
}

// OpenRoller opens the roller shutter
func (c *Client) OpenRoller(ctx context.Context, channel int) error {
	url := fmt.Sprintf("http://%s/roller/%d", c.ip, channel)
	params := map[string]interface{}{
		"go": "open",
	}
	return c.postForm(ctx, url, params)
}

// CloseRoller closes the roller shutter
func (c *Client) CloseRoller(ctx context.Context, channel int) error {
	url := fmt.Sprintf("http://%s/roller/%d", c.ip, channel)
	params := map[string]interface{}{
		"go": "close",
	}
	return c.postForm(ctx, url, params)
}

// StopRoller stops the roller shutter movement
func (c *Client) StopRoller(ctx context.Context, channel int) error {
	url := fmt.Sprintf("http://%s/roller/%d", c.ip, channel)
	params := map[string]interface{}{
		"go": "stop",
	}
	return c.postForm(ctx, url, params)
}

// GetRollerStatus gets the current roller position and status
func (c *Client) GetRollerStatus(ctx context.Context, channel int) (*RollerPosition, error) {
	url := fmt.Sprintf("http://%s/roller/%d", c.ip, channel)

	var position RollerPosition
	if err := c.getJSON(ctx, url, &position); err != nil {
		return nil, err
	}

	return &position, nil
}

// GetRollerSettings gets roller configuration
func (c *Client) GetRollerSettings(ctx context.Context, channel int) (*RollerSettings, error) {
	url := fmt.Sprintf("http://%s/settings/roller/%d", c.ip, channel)

	var settings RollerSettings
	if err := c.getJSON(ctx, url, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// SetRollerSettings updates roller configuration
func (c *Client) SetRollerSettings(ctx context.Context, channel int, settings *RollerSettings) error {
	url := fmt.Sprintf("http://%s/settings/roller/%d", c.ip, channel)

	params := make(map[string]interface{})

	// Only include non-zero values
	if settings.MaxTime > 0 {
		params["maxtime"] = settings.MaxTime
	}
	if settings.DefaultState != "" {
		params["default_state"] = settings.DefaultState
	}
	if settings.InputMode != "" {
		params["input_mode"] = settings.InputMode
	}
	if settings.ButtonType != "" {
		params["button_type"] = settings.ButtonType
	}
	if settings.FavPos >= 0 && settings.FavPos <= 100 {
		params["fav_pos"] = settings.FavPos
	}
	if settings.ObstructionPower > 0 {
		params["obstruction_power"] = settings.ObstructionPower
		params["obstruction_detect"] = settings.ObstructionDetect
		params["obstruction_action"] = settings.ObstructionAction
	}

	// Boolean values need explicit setting
	params["swap"] = settings.SwapInputs
	params["swap_outputs"] = settings.SwapOutputs
	params["safety_switch"] = settings.SafetySwitch

	return c.postForm(ctx, url, params)
}

// CalibrateRoller starts roller calibration
func (c *Client) CalibrateRoller(ctx context.Context, channel int) error {
	url := fmt.Sprintf("http://%s/roller/%d", c.ip, channel)
	params := map[string]interface{}{
		"go": "calibrate",
	}
	return c.postForm(ctx, url, params)
}

// SetRollerFavoritePosition sets the roller to its favorite position
func (c *Client) SetRollerFavoritePosition(ctx context.Context, channel int) error {
	url := fmt.Sprintf("http://%s/roller/%d", c.ip, channel)
	params := map[string]interface{}{
		"go": "to_fav",
	}
	return c.postForm(ctx, url, params)
}

// SetDeviceMode switches between relay and roller mode (Shelly 2.5)
func (c *Client) SetDeviceMode(ctx context.Context, mode string) error {
	// mode should be "relay" or "roller"
	if mode != "relay" && mode != "roller" {
		return fmt.Errorf("invalid mode: %s (must be 'relay' or 'roller')", mode)
	}

	url := fmt.Sprintf("http://%s/settings", c.ip)
	params := map[string]interface{}{
		"mode": mode,
	}

	// Mode change requires reboot
	if err := c.postForm(ctx, url, params); err != nil {
		return err
	}

	// Wait a moment then reboot
	time.Sleep(500 * time.Millisecond)
	return c.Reboot(ctx)
}
