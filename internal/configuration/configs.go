package configuration

import "time"

// RelayConfig represents relay switch configuration
type RelayConfig struct {
	DefaultState  *string             `json:"default_state,omitempty"`
	ButtonType    *string             `json:"btn_type,omitempty"`
	AutoOn        *int                `json:"auto_on,omitempty"`
	AutoOff       *int                `json:"auto_off,omitempty"`
	HasTimer      *bool               `json:"has_timer,omitempty"`
	MaxPowerLimit *int                `json:"max_power_limit,omitempty"`
	Relays        []SingleRelayConfig `json:"relays,omitempty"`
}

// SingleRelayConfig represents configuration for a single relay
type SingleRelayConfig struct {
	ID           int     `json:"id"`
	Name         *string `json:"name,omitempty"`
	DefaultState *string `json:"default_state,omitempty"`
	AutoOn       *int    `json:"auto_on,omitempty"`
	AutoOff      *int    `json:"auto_off,omitempty"`
	Schedule     *bool   `json:"schedule,omitempty"`
}

// PowerMeteringConfig represents power metering configuration
type PowerMeteringConfig struct {
	MaxPower          *int       `json:"max_power,omitempty"`
	MaxVoltage        *int       `json:"max_voltage,omitempty"`
	MaxCurrent        *float64   `json:"max_current,omitempty"`
	ProtectionAction  *string    `json:"protection_action,omitempty"`
	PowerCorrection   *float64   `json:"power_correction,omitempty"`
	VoltageCorrection *float64   `json:"voltage_correction,omitempty"`
	CurrentCorrection *float64   `json:"current_correction,omitempty"`
	EnergyReset       *time.Time `json:"energy_reset,omitempty"`
	ReportingPeriod   *int       `json:"reporting_period,omitempty"`
	CostPerKWh        *float64   `json:"cost_per_kwh,omitempty"`
	Currency          *string    `json:"currency,omitempty"`
}

// DimmingConfig represents dimmer configuration
type DimmingConfig struct {
	MinBrightness       *int    `json:"min_brightness,omitempty"`
	MaxBrightness       *int    `json:"max_brightness,omitempty"`
	DefaultBrightness   *int    `json:"default_brightness,omitempty"`
	DefaultState        *bool   `json:"default_state,omitempty"`
	FadeRate            *int    `json:"fade_rate,omitempty"`
	TransitionTime      *int    `json:"transition,omitempty"`
	LeadingEdge         *bool   `json:"leading_edge,omitempty"`
	WarmupTime          *int    `json:"warmup_time,omitempty"`
	MinDimLevel         *int    `json:"min_dim_level,omitempty"`
	NightModeEnabled    *bool   `json:"night_mode,omitempty"`
	NightModeBrightness *int    `json:"night_mode_brightness,omitempty"`
	NightModeStart      *string `json:"night_mode_start,omitempty"`
	NightModeEnd        *string `json:"night_mode_end,omitempty"`
}

// RollerConfig represents roller shutter/blind configuration
type RollerConfig struct {
	MotorDirection     *string `json:"motor_direction,omitempty"`
	MotorSpeed         *int    `json:"motor_speed,omitempty"`
	CalibrationState   *string `json:"calibration_state,omitempty"`
	MaxOpenTime        *int    `json:"max_open_time,omitempty"`
	MaxCloseTime       *int    `json:"max_close_time,omitempty"`
	DefaultPosition    *int    `json:"default_position,omitempty"`
	CurrentPosition    *int    `json:"current_position,omitempty"`
	PositioningEnabled *bool   `json:"positioning_enabled,omitempty"`
	ObstacleDetection  *bool   `json:"obstacle_detection,omitempty"`
	ObstaclePower      *int    `json:"obstacle_power,omitempty"`
	SafetySwitch       *bool   `json:"safety_switch,omitempty"`
	SwapInputs         *bool   `json:"swap_inputs,omitempty"`
	InputMode          *string `json:"input_mode,omitempty"`
	ButtonHoldTime     *int    `json:"button_hold_time,omitempty"`
	TiltEnabled        *bool   `json:"tilt_enabled,omitempty"`
	TiltPosition       *int    `json:"tilt_position,omitempty"`
	MaxTiltTime        *int    `json:"max_tilt_time,omitempty"`
}

// InputConfig represents input/button configuration
type InputConfig struct {
	Type             *string             `json:"type,omitempty"`
	Mode             *string             `json:"mode,omitempty"`
	Inverted         *bool               `json:"inverted,omitempty"`
	DebounceTime     *int                `json:"debounce_time,omitempty"`
	LongPushTime     *int                `json:"long_push_time,omitempty"`
	MultiPushTime    *int                `json:"multi_push_time,omitempty"`
	SinglePushAction *string             `json:"single_push_action,omitempty"`
	DoublePushAction *string             `json:"double_push_action,omitempty"`
	TriplePushAction *string             `json:"triple_push_action,omitempty"`
	LongPushAction   *string             `json:"long_push_action,omitempty"`
	Inputs           []SingleInputConfig `json:"inputs,omitempty"`
}

// SingleInputConfig represents a single input configuration
type SingleInputConfig struct {
	ID               int     `json:"id"`
	Name             *string `json:"name,omitempty"`
	Type             *string `json:"type,omitempty"`
	Mode             *string `json:"mode,omitempty"`
	Inverted         *bool   `json:"inverted,omitempty"`
	SinglePushAction *string `json:"single_push_action,omitempty"`
	LongPushAction   *string `json:"long_push_action,omitempty"`
}

// LEDConfig represents LED indicator configuration
type LEDConfig struct {
	Enabled             *bool   `json:"enabled,omitempty"`
	Mode                *string `json:"mode,omitempty"`
	Brightness          *int    `json:"brightness,omitempty"`
	NightModeEnabled    *bool   `json:"night_mode,omitempty"`
	NightModeBrightness *int    `json:"night_mode_brightness,omitempty"`
	NightModeStart      *string `json:"night_mode_start,omitempty"`
	NightModeEnd        *string `json:"night_mode_end,omitempty"`
	PowerIndication     *bool   `json:"power_indication,omitempty"`
	NetworkIndication   *bool   `json:"network_indication,omitempty"`
}

// ColorConfig represents RGB/W color control configuration
type ColorConfig struct {
	// Color mode
	Mode         string `json:"mode"` // "color", "white", "color_white"
	DefaultColor *Color `json:"default_color,omitempty"`
	DefaultWhite *int   `json:"default_white,omitempty"` // color temperature

	// Effects
	EffectsEnabled bool     `json:"effects_enabled"`
	ActiveEffect   *int     `json:"active_effect,omitempty"`
	EffectSpeed    int      `json:"effect_speed"` // 0-100
	CustomEffects  []Effect `json:"custom_effects,omitempty"`

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
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Type       string  `json:"type"` // "static", "fade", "pulse", "rainbow"
	Colors     []Color `json:"colors,omitempty"`
	Speed      int     `json:"speed"`
	Brightness int     `json:"brightness"`
}

// TempProtectionConfig represents temperature protection configuration
type TempProtectionConfig struct {
	// Temperature thresholds
	MaxTemp     float64 `json:"max_temp"`     // Celsius
	MinTemp     float64 `json:"min_temp"`     // Celsius
	WarningTemp float64 `json:"warning_temp"` // Celsius

	// Actions
	OverheatAction string `json:"overheat_action"` // "off", "alert", "reduce_power"
	FreezeAction   string `json:"freeze_action"`   // "off", "alert", "heat"

	// Hysteresis
	Hysteresis    float64 `json:"hysteresis"`     // degrees
	CheckInterval int     `json:"check_interval"` // seconds
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
	Time    string   `json:"time"`   // "HH:MM" or "sunrise+30" or "sunset-15"
	Days    []string `json:"days"`   // ["mon", "tue", "wed", ...]
	Action  string   `json:"action"` // "on", "off", "toggle", "dim:50"
	Target  string   `json:"target"` // component ID or "all"
}

// CoIoTConfig represents CoIoT protocol configuration
type CoIoTConfig struct {
	Enabled bool   `json:"enabled"`
	Server  string `json:"server"`
	Port    int    `json:"port"`
	Period  int    `json:"period"` // seconds
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
	Name        string   `json:"name"`
	StartTime   string   `json:"start_time"` // "HH:MM"
	EndTime     string   `json:"end_time"`   // "HH:MM"
	Days        []string `json:"days"`       // ["mon", "tue", ...]
	PricePerKWh float64  `json:"price_per_kwh"`
}

// MotionConfig represents motion sensor configuration
type MotionConfig struct {
	Enabled          bool `json:"enabled"`
	Sensitivity      int  `json:"sensitivity"`       // 1-100
	BlindTime        int  `json:"blind_time"`        // seconds after trigger
	DetectionTimeout int  `json:"detection_timeout"` // seconds to clear

	// Actions
	OnMotionAction string `json:"on_motion_action"`
	OnClearAction  string `json:"on_clear_action"`

	// LED indication
	LEDIndication bool `json:"led_indication"`
}

// SensorConfig represents environmental sensor configuration
type SensorConfig struct {
	// Temperature sensor
	TempUnit      string  `json:"temp_unit"` // "C" or "F"
	TempOffset    float64 `json:"temp_offset"`
	TempReporting int     `json:"temp_reporting"` // seconds

	// Humidity sensor
	HumidityOffset    float64 `json:"humidity_offset"`
	HumidityReporting int     `json:"humidity_reporting"` // seconds

	// Lux sensor
	LuxOffset    float64 `json:"lux_offset"`
	LuxReporting int     `json:"lux_reporting"` // seconds

	// Thresholds for alerts
	TempMin     *float64 `json:"temp_min,omitempty"`
	TempMax     *float64 `json:"temp_max,omitempty"`
	HumidityMin *float64 `json:"humidity_min,omitempty"`
	HumidityMax *float64 `json:"humidity_max,omitempty"`
	LuxMin      *float64 `json:"lux_min,omitempty"`
	LuxMax      *float64 `json:"lux_max,omitempty"`
}
