package configuration

import "time"

// RelayConfig represents relay switch configuration
type RelayConfig struct {
	// Basic settings
	DefaultState string `json:"default_state"` // "on", "off", "last", "switch"
	ButtonType   string `json:"btn_type"`      // "momentary", "toggle", "edge", "detached"
	
	// Timers
	AutoOn  *int `json:"auto_on"`  // seconds, nil = disabled
	AutoOff *int `json:"auto_off"` // seconds, nil = disabled
	
	// Advanced settings
	HasTimer      bool `json:"has_timer"`       // supports scheduling
	MaxPowerLimit *int `json:"max_power_limit"` // Watts, for PM devices
	
	// Multi-relay settings (for devices with multiple relays)
	Relays []SingleRelayConfig `json:"relays,omitempty"`
}

// SingleRelayConfig represents configuration for a single relay
type SingleRelayConfig struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	DefaultState string `json:"default_state"`
	AutoOn       *int   `json:"auto_on"`
	AutoOff      *int   `json:"auto_off"`
	Schedule     bool   `json:"schedule"`
}

// PowerMeteringConfig represents power metering configuration
type PowerMeteringConfig struct {
	// Limits and protection
	MaxPower         *int     `json:"max_power"`          // Watts
	MaxVoltage       *int     `json:"max_voltage"`        // Volts
	MaxCurrent       *float64 `json:"max_current"`        // Amps
	ProtectionAction string   `json:"protection_action"`  // "off", "restart", "alert"
	
	// Calibration
	PowerCorrection   float64 `json:"power_correction"`   // multiplier
	VoltageCorrection float64 `json:"voltage_correction"` // multiplier
	CurrentCorrection float64 `json:"current_correction"` // multiplier
	
	// Energy tracking
	EnergyReset     *time.Time `json:"energy_reset"`      // last reset timestamp
	ReportingPeriod int        `json:"reporting_period"`  // seconds between reports
	
	// Cost tracking (optional)
	CostPerKWh     *float64 `json:"cost_per_kwh,omitempty"`
	Currency       string   `json:"currency,omitempty"`
}

// DimmingConfig represents dimmer configuration
type DimmingConfig struct {
	// Brightness settings
	MinBrightness     int  `json:"min_brightness"`     // 1-100
	MaxBrightness     int  `json:"max_brightness"`     // 1-100
	DefaultBrightness int  `json:"default_brightness"` // 1-100
	DefaultState      bool `json:"default_state"`      // on/off at power up
	
	// Transition settings
	FadeRate       int `json:"fade_rate"`       // ms per step
	TransitionTime int `json:"transition"`      // ms for transitions
	
	// Advanced settings
	LeadingEdge   bool   `json:"leading_edge"`   // dimming type
	WarmupTime    int    `json:"warmup_time"`    // ms
	MinDimLevel   int    `json:"min_dim_level"`  // hardware minimum %
	NightModeEnabled bool `json:"night_mode"`
	NightModeBrightness int `json:"night_mode_brightness"`
	NightModeStart string `json:"night_mode_start"` // "22:00"
	NightModeEnd   string `json:"night_mode_end"`   // "06:00"
}

// RollerConfig represents roller shutter/blind configuration
type RollerConfig struct {
	// Motor settings
	MotorDirection   string `json:"motor_direction"`   // "normal", "reverse"
	MotorSpeed       int    `json:"motor_speed"`       // RPM
	CalibrationState string `json:"calibration_state"` // "not_calibrated", "calibrating", "calibrated"
	
	// Position settings
	MaxOpenTime      int  `json:"max_open_time"`      // seconds
	MaxCloseTime     int  `json:"max_close_time"`     // seconds
	DefaultPosition  *int `json:"default_position"`    // 0-100, nil = no default
	CurrentPosition  int  `json:"current_position"`    // 0-100
	PositioningEnabled bool `json:"positioning_enabled"`
	
	// Safety settings
	ObstacleDetection bool   `json:"obstacle_detection"`
	ObstaclePower     *int   `json:"obstacle_power"` // Watts threshold
	SafetySwitch      bool   `json:"safety_switch"`
	
	// Button configuration
	SwapInputs       bool   `json:"swap_inputs"`
	InputMode        string `json:"input_mode"` // "one_button", "two_buttons", "detached"
	ButtonHoldTime   int    `json:"button_hold_time"` // ms
	
	// Tilt settings (for venetian blinds)
	TiltEnabled      bool `json:"tilt_enabled"`
	TiltPosition     *int `json:"tilt_position"`     // -100 to 100
	MaxTiltTime      *int `json:"max_tilt_time"`      // ms
}

// InputConfig represents input/button configuration
type InputConfig struct {
	// Input type and mode
	Type         string `json:"type"`          // "button", "switch", "analog"
	Mode         string `json:"mode"`          // "momentary", "toggle", "edge", "detached"
	Inverted     bool   `json:"inverted"`      // invert input logic
	
	// Debounce and timing
	DebounceTime int `json:"debounce_time"` // ms
	LongPushTime int `json:"long_push_time"` // ms
	MultiPushTime int `json:"multi_push_time"` // ms window for multi-click
	
	// Actions
	SinglePushAction string `json:"single_push_action"`
	DoublePushAction string `json:"double_push_action"`
	TriplePushAction string `json:"triple_push_action"`
	LongPushAction   string `json:"long_push_action"`
	
	// Multiple inputs (for multi-input devices)
	Inputs []SingleInputConfig `json:"inputs,omitempty"`
}

// SingleInputConfig represents a single input configuration
type SingleInputConfig struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	Mode             string `json:"mode"`
	Inverted         bool   `json:"inverted"`
	SinglePushAction string `json:"single_push_action"`
	LongPushAction   string `json:"long_push_action"`
}

// LEDConfig represents LED indicator configuration
type LEDConfig struct {
	// Basic settings
	Enabled          bool   `json:"enabled"`
	Mode             string `json:"mode"`       // "on", "off", "status", "network"
	Brightness       int    `json:"brightness"` // 0-100
	NightModeEnabled bool   `json:"night_mode"`
	
	// Night mode settings
	NightModeBrightness int    `json:"night_mode_brightness"`
	NightModeStart      string `json:"night_mode_start"` // "22:00"
	NightModeEnd        string `json:"night_mode_end"`   // "06:00"
	
	// Status indication
	PowerIndication    bool `json:"power_indication"`    // show power state
	NetworkIndication  bool `json:"network_indication"`  // show network status
}

// ColorConfig represents RGB/W color control configuration
type ColorConfig struct {
	// Color mode
	Mode            string `json:"mode"` // "color", "white", "color_white"
	DefaultColor    *Color `json:"default_color,omitempty"`
	DefaultWhite    *int   `json:"default_white,omitempty"` // color temperature
	
	// Effects
	EffectsEnabled  bool     `json:"effects_enabled"`
	ActiveEffect    *int     `json:"active_effect,omitempty"`
	EffectSpeed     int      `json:"effect_speed"` // 0-100
	CustomEffects   []Effect `json:"custom_effects,omitempty"`
	
	// Calibration
	RedCalibration   float64 `json:"red_calibration"`
	GreenCalibration float64 `json:"green_calibration"`
	BlueCalibration  float64 `json:"blue_calibration"`
	WhiteCalibration float64 `json:"white_calibration"`
}

// Color represents an RGB color
type Color struct {
	Red   int `json:"r"` // 0-255
	Green int `json:"g"` // 0-255
	Blue  int `json:"b"` // 0-255
}

// Effect represents a lighting effect
type Effect struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // "static", "fade", "pulse", "rainbow"
	Colors      []Color `json:"colors,omitempty"`
	Speed       int    `json:"speed"`
	Brightness  int    `json:"brightness"`
}

// TempProtectionConfig represents temperature protection configuration
type TempProtectionConfig struct {
	// Temperature thresholds
	MaxTemp         float64 `json:"max_temp"`         // Celsius
	MinTemp         float64 `json:"min_temp"`         // Celsius
	WarningTemp     float64 `json:"warning_temp"`     // Celsius
	
	// Actions
	OverheatAction  string `json:"overheat_action"`  // "off", "alert", "reduce_power"
	FreezeAction    string `json:"freeze_action"`    // "off", "alert", "heat"
	
	// Hysteresis
	Hysteresis      float64 `json:"hysteresis"`      // degrees
	CheckInterval   int     `json:"check_interval"`   // seconds
}

// ScheduleConfig represents scheduling configuration
type ScheduleConfig struct {
	Enabled   bool       `json:"enabled"`
	Schedules []Schedule `json:"schedules"`
	
	// Sunrise/sunset support
	UseSunriseSunset bool    `json:"use_sunrise_sunset"`
	Latitude         float64 `json:"latitude,omitempty"`
	Longitude        float64 `json:"longitude,omitempty"`
	
	// Timezone
	Timezone string `json:"timezone"`
}

// Schedule represents a single schedule entry
type Schedule struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	Enabled bool     `json:"enabled"`
	Time    string   `json:"time"`     // "HH:MM" or "sunrise+30" or "sunset-15"
	Days    []string `json:"days"`     // ["mon", "tue", "wed", ...]
	Action  string   `json:"action"`   // "on", "off", "toggle", "dim:50"
	Target  string   `json:"target"`   // component ID or "all"
}

// CoIoTConfig represents CoIoT protocol configuration
type CoIoTConfig struct {
	Enabled        bool   `json:"enabled"`
	Server         string `json:"server"`
	Port           int    `json:"port"`
	Period         int    `json:"period"` // seconds
}

// EnergyMeterConfig represents energy consumption tracking
type EnergyMeterConfig struct {
	// Measurement settings
	ReportingInterval int `json:"reporting_interval"` // seconds
	RetentionDays     int `json:"retention_days"`     // days to keep history
	
	// Tariffs
	Tariffs []EnergyTariff `json:"tariffs,omitempty"`
	
	// Alerts
	DailyLimitKWh   *float64 `json:"daily_limit_kwh,omitempty"`
	MonthlyLimitKWh *float64 `json:"monthly_limit_kwh,omitempty"`
	AlertEmail      string   `json:"alert_email,omitempty"`
}

// EnergyTariff represents time-based energy pricing
type EnergyTariff struct {
	Name      string   `json:"name"`
	StartTime string   `json:"start_time"` // "HH:MM"
	EndTime   string   `json:"end_time"`   // "HH:MM"
	Days      []string `json:"days"`       // ["mon", "tue", ...]
	PricePerKWh float64 `json:"price_per_kwh"`
}

// MotionConfig represents motion sensor configuration
type MotionConfig struct {
	Enabled          bool `json:"enabled"`
	Sensitivity      int  `json:"sensitivity"`      // 1-100
	BlindTime        int  `json:"blind_time"`       // seconds after trigger
	DetectionTimeout int  `json:"detection_timeout"` // seconds to clear
	
	// Actions
	OnMotionAction  string `json:"on_motion_action"`
	OnClearAction   string `json:"on_clear_action"`
	
	// LED indication
	LEDIndication   bool `json:"led_indication"`
}

// SensorConfig represents environmental sensor configuration
type SensorConfig struct {
	// Temperature sensor
	TempUnit         string  `json:"temp_unit"` // "C" or "F"
	TempOffset       float64 `json:"temp_offset"`
	TempReporting    int     `json:"temp_reporting"` // seconds
	
	// Humidity sensor
	HumidityOffset   float64 `json:"humidity_offset"`
	HumidityReporting int    `json:"humidity_reporting"` // seconds
	
	// Lux sensor
	LuxOffset        float64 `json:"lux_offset"`
	LuxReporting     int     `json:"lux_reporting"` // seconds
	
	// Thresholds for alerts
	TempMin          *float64 `json:"temp_min,omitempty"`
	TempMax          *float64 `json:"temp_max,omitempty"`
	HumidityMin      *float64 `json:"humidity_min,omitempty"`
	HumidityMax      *float64 `json:"humidity_max,omitempty"`
	LuxMin           *float64 `json:"lux_min,omitempty"`
	LuxMax           *float64 `json:"lux_max,omitempty"`
}