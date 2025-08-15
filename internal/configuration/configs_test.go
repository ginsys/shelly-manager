package configuration

import (
	"encoding/json"
	"testing"
	"time"
)

// TestRelayConfig tests relay configuration
func TestRelayConfig(t *testing.T) {
	autoOn := 300
	autoOff := 600
	maxPower := 2300

	config := RelayConfig{
		DefaultState:  "last",
		ButtonType:    "toggle",
		AutoOn:        &autoOn,
		AutoOff:       &autoOff,
		HasTimer:      true,
		MaxPowerLimit: &maxPower,
		Relays: []SingleRelayConfig{
			{
				ID:           0,
				Name:         "Main Relay",
				DefaultState: "off",
				AutoOn:       &autoOn,
				Schedule:     true,
			},
			{
				ID:           1,
				Name:         "Secondary Relay",
				DefaultState: "on",
				AutoOff:      &autoOff,
				Schedule:     false,
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal RelayConfig: %v", err)
	}

	// Test JSON unmarshaling
	var decoded RelayConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal RelayConfig: %v", err)
	}

	// Verify fields
	if decoded.DefaultState != config.DefaultState {
		t.Errorf("DefaultState mismatch: got %s, want %s", decoded.DefaultState, config.DefaultState)
	}
	if decoded.ButtonType != config.ButtonType {
		t.Errorf("ButtonType mismatch: got %s, want %s", decoded.ButtonType, config.ButtonType)
	}
	if decoded.AutoOn == nil || *decoded.AutoOn != autoOn {
		t.Errorf("AutoOn mismatch: got %v, want %d", decoded.AutoOn, autoOn)
	}
	if decoded.AutoOff == nil || *decoded.AutoOff != autoOff {
		t.Errorf("AutoOff mismatch: got %v, want %d", decoded.AutoOff, autoOff)
	}
	if len(decoded.Relays) != 2 {
		t.Fatalf("Expected 2 relays, got %d", len(decoded.Relays))
	}
	if decoded.Relays[0].Name != "Main Relay" {
		t.Errorf("Relay[0].Name mismatch: got %s, want Main Relay", decoded.Relays[0].Name)
	}
}

// TestPowerMeteringConfig tests power metering configuration
func TestPowerMeteringConfig(t *testing.T) {
	maxPower := 2300
	maxVoltage := 250
	maxCurrent := 10.0
	costPerKWh := 0.25
	resetTime := time.Now()

	config := PowerMeteringConfig{
		MaxPower:          &maxPower,
		MaxVoltage:        &maxVoltage,
		MaxCurrent:        &maxCurrent,
		ProtectionAction:  "off",
		PowerCorrection:   1.05,
		VoltageCorrection: 0.98,
		CurrentCorrection: 1.02,
		EnergyReset:       &resetTime,
		ReportingPeriod:   60,
		CostPerKWh:        &costPerKWh,
		Currency:          "EUR",
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal PowerMeteringConfig: %v", err)
	}

	// Test JSON unmarshaling
	var decoded PowerMeteringConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal PowerMeteringConfig: %v", err)
	}

	// Verify fields
	if decoded.MaxPower == nil || *decoded.MaxPower != maxPower {
		t.Errorf("MaxPower mismatch: got %v, want %d", decoded.MaxPower, maxPower)
	}
	if decoded.MaxVoltage == nil || *decoded.MaxVoltage != maxVoltage {
		t.Errorf("MaxVoltage mismatch: got %v, want %d", decoded.MaxVoltage, maxVoltage)
	}
	if decoded.MaxCurrent == nil || *decoded.MaxCurrent != maxCurrent {
		t.Errorf("MaxCurrent mismatch: got %v, want %f", decoded.MaxCurrent, maxCurrent)
	}
	if decoded.ProtectionAction != config.ProtectionAction {
		t.Errorf("ProtectionAction mismatch: got %s, want %s", decoded.ProtectionAction, config.ProtectionAction)
	}
	if decoded.PowerCorrection != config.PowerCorrection {
		t.Errorf("PowerCorrection mismatch: got %f, want %f", decoded.PowerCorrection, config.PowerCorrection)
	}
	if decoded.CostPerKWh == nil || *decoded.CostPerKWh != costPerKWh {
		t.Errorf("CostPerKWh mismatch: got %v, want %f", decoded.CostPerKWh, costPerKWh)
	}
	if decoded.Currency != config.Currency {
		t.Errorf("Currency mismatch: got %s, want %s", decoded.Currency, config.Currency)
	}
}

// TestDimmingConfig tests dimming configuration
func TestDimmingConfig(t *testing.T) {
	config := DimmingConfig{
		MinBrightness:       10,
		MaxBrightness:       100,
		DefaultBrightness:   50,
		DefaultState:        true,
		FadeRate:            100,
		TransitionTime:      1000,
		LeadingEdge:         false,
		WarmupTime:          500,
		MinDimLevel:         5,
		NightModeEnabled:    true,
		NightModeBrightness: 20,
		NightModeStart:      "22:00",
		NightModeEnd:        "06:00",
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal DimmingConfig: %v", err)
	}

	// Test JSON unmarshaling
	var decoded DimmingConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal DimmingConfig: %v", err)
	}

	// Verify fields
	if decoded.MinBrightness != config.MinBrightness {
		t.Errorf("MinBrightness mismatch: got %d, want %d", decoded.MinBrightness, config.MinBrightness)
	}
	if decoded.MaxBrightness != config.MaxBrightness {
		t.Errorf("MaxBrightness mismatch: got %d, want %d", decoded.MaxBrightness, config.MaxBrightness)
	}
	if decoded.DefaultBrightness != config.DefaultBrightness {
		t.Errorf("DefaultBrightness mismatch: got %d, want %d", decoded.DefaultBrightness, config.DefaultBrightness)
	}
	if decoded.NightModeEnabled != config.NightModeEnabled {
		t.Errorf("NightModeEnabled mismatch: got %v, want %v", decoded.NightModeEnabled, config.NightModeEnabled)
	}
	if decoded.NightModeStart != config.NightModeStart {
		t.Errorf("NightModeStart mismatch: got %s, want %s", decoded.NightModeStart, config.NightModeStart)
	}
}

// TestRollerConfig tests roller shutter configuration
func TestRollerConfig(t *testing.T) {
	defaultPos := 50
	obstaclePower := 200
	tiltPos := 45
	maxTiltTime := 1500

	config := RollerConfig{
		MotorDirection:     "normal",
		MotorSpeed:         30,
		CalibrationState:   "calibrated",
		MaxOpenTime:        25,
		MaxCloseTime:       24,
		DefaultPosition:    &defaultPos,
		CurrentPosition:    75,
		PositioningEnabled: true,
		ObstacleDetection:  true,
		ObstaclePower:      &obstaclePower,
		SafetySwitch:       true,
		SwapInputs:         false,
		InputMode:          "two_buttons",
		ButtonHoldTime:     800,
		TiltEnabled:        true,
		TiltPosition:       &tiltPos,
		MaxTiltTime:        &maxTiltTime,
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal RollerConfig: %v", err)
	}

	// Test JSON unmarshaling
	var decoded RollerConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal RollerConfig: %v", err)
	}

	// Verify fields
	if decoded.MotorDirection != config.MotorDirection {
		t.Errorf("MotorDirection mismatch: got %s, want %s", decoded.MotorDirection, config.MotorDirection)
	}
	if decoded.CalibrationState != config.CalibrationState {
		t.Errorf("CalibrationState mismatch: got %s, want %s", decoded.CalibrationState, config.CalibrationState)
	}
	if decoded.DefaultPosition == nil || *decoded.DefaultPosition != defaultPos {
		t.Errorf("DefaultPosition mismatch: got %v, want %d", decoded.DefaultPosition, defaultPos)
	}
	if decoded.ObstaclePower == nil || *decoded.ObstaclePower != obstaclePower {
		t.Errorf("ObstaclePower mismatch: got %v, want %d", decoded.ObstaclePower, obstaclePower)
	}
	if decoded.TiltEnabled != config.TiltEnabled {
		t.Errorf("TiltEnabled mismatch: got %v, want %v", decoded.TiltEnabled, config.TiltEnabled)
	}
	if decoded.TiltPosition == nil || *decoded.TiltPosition != tiltPos {
		t.Errorf("TiltPosition mismatch: got %v, want %d", decoded.TiltPosition, tiltPos)
	}
}

// TestInputConfig tests input configuration
func TestInputConfig(t *testing.T) {
	config := InputConfig{
		Type:             "button",
		Mode:             "momentary",
		Inverted:         false,
		DebounceTime:     50,
		LongPushTime:     1000,
		MultiPushTime:    500,
		SinglePushAction: "toggle",
		DoublePushAction: "on",
		TriplePushAction: "off",
		LongPushAction:   "dim",
		Inputs: []SingleInputConfig{
			{
				ID:               0,
				Name:             "Button 1",
				Type:             "button",
				Mode:             "momentary",
				Inverted:         false,
				SinglePushAction: "toggle",
				LongPushAction:   "off",
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal InputConfig: %v", err)
	}

	// Test JSON unmarshaling
	var decoded InputConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal InputConfig: %v", err)
	}

	// Verify fields
	if decoded.Type != config.Type {
		t.Errorf("Type mismatch: got %s, want %s", decoded.Type, config.Type)
	}
	if decoded.Mode != config.Mode {
		t.Errorf("Mode mismatch: got %s, want %s", decoded.Mode, config.Mode)
	}
	if decoded.LongPushTime != config.LongPushTime {
		t.Errorf("LongPushTime mismatch: got %d, want %d", decoded.LongPushTime, config.LongPushTime)
	}
	if decoded.SinglePushAction != config.SinglePushAction {
		t.Errorf("SinglePushAction mismatch: got %s, want %s", decoded.SinglePushAction, config.SinglePushAction)
	}
	if len(decoded.Inputs) != 1 {
		t.Fatalf("Expected 1 input, got %d", len(decoded.Inputs))
	}
	if decoded.Inputs[0].Name != "Button 1" {
		t.Errorf("Input[0].Name mismatch: got %s, want Button 1", decoded.Inputs[0].Name)
	}
}

// TestLEDConfig tests LED configuration
func TestLEDConfig(t *testing.T) {
	config := LEDConfig{
		Enabled:             true,
		Mode:                "status",
		Brightness:          75,
		NightModeEnabled:    true,
		NightModeBrightness: 10,
		NightModeStart:      "22:00",
		NightModeEnd:        "07:00",
		PowerIndication:     true,
		NetworkIndication:   true,
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal LEDConfig: %v", err)
	}

	// Test JSON unmarshaling
	var decoded LEDConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal LEDConfig: %v", err)
	}

	// Verify fields
	if decoded.Enabled != config.Enabled {
		t.Errorf("Enabled mismatch: got %v, want %v", decoded.Enabled, config.Enabled)
	}
	if decoded.Mode != config.Mode {
		t.Errorf("Mode mismatch: got %s, want %s", decoded.Mode, config.Mode)
	}
	if decoded.Brightness != config.Brightness {
		t.Errorf("Brightness mismatch: got %d, want %d", decoded.Brightness, config.Brightness)
	}
	if decoded.PowerIndication != config.PowerIndication {
		t.Errorf("PowerIndication mismatch: got %v, want %v", decoded.PowerIndication, config.PowerIndication)
	}
}

// TestColorConfig tests color control configuration
func TestColorConfig(t *testing.T) {
	defaultWhite := 4000
	activeEffect := 1

	config := ColorConfig{
		Mode: "color_white",
		DefaultColor: &Color{
			Red:   255,
			Green: 128,
			Blue:  64,
		},
		DefaultWhite:     &defaultWhite,
		EffectsEnabled:   true,
		ActiveEffect:     &activeEffect,
		EffectSpeed:      50,
		RedCalibration:   1.0,
		GreenCalibration: 0.95,
		BlueCalibration:  1.05,
		WhiteCalibration: 1.0,
		CustomEffects: []Effect{
			{
				ID:         1,
				Name:       "Rainbow",
				Type:       "rainbow",
				Speed:      75,
				Brightness: 100,
			},
			{
				ID:   2,
				Name: "Pulse",
				Type: "pulse",
				Colors: []Color{
					{Red: 255, Green: 0, Blue: 0},
					{Red: 0, Green: 255, Blue: 0},
					{Red: 0, Green: 0, Blue: 255},
				},
				Speed:      30,
				Brightness: 80,
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal ColorConfig: %v", err)
	}

	// Test JSON unmarshaling
	var decoded ColorConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ColorConfig: %v", err)
	}

	// Verify fields
	if decoded.Mode != config.Mode {
		t.Errorf("Mode mismatch: got %s, want %s", decoded.Mode, config.Mode)
	}
	if decoded.DefaultColor == nil {
		t.Error("DefaultColor should not be nil")
	} else if decoded.DefaultColor.Red != 255 {
		t.Errorf("DefaultColor.Red mismatch: got %d, want 255", decoded.DefaultColor.Red)
	}
	if decoded.DefaultWhite == nil || *decoded.DefaultWhite != defaultWhite {
		t.Errorf("DefaultWhite mismatch: got %v, want %d", decoded.DefaultWhite, defaultWhite)
	}
	if len(decoded.CustomEffects) != 2 {
		t.Fatalf("Expected 2 custom effects, got %d", len(decoded.CustomEffects))
	}
	if decoded.CustomEffects[1].Name != "Pulse" {
		t.Errorf("CustomEffects[1].Name mismatch: got %s, want Pulse", decoded.CustomEffects[1].Name)
	}
	if len(decoded.CustomEffects[1].Colors) != 3 {
		t.Errorf("Expected 3 colors in Pulse effect, got %d", len(decoded.CustomEffects[1].Colors))
	}
}

// TestScheduleConfig tests schedule configuration
func TestScheduleConfig(t *testing.T) {
	config := ScheduleConfig{
		Enabled: true,
		Schedules: []Schedule{
			{
				ID:      1,
				Name:    "Morning On",
				Enabled: true,
				Time:    "06:30",
				Days:    []string{"mon", "tue", "wed", "thu", "fri"},
				Action:  "on",
				Target:  "relay:0",
			},
			{
				ID:      2,
				Name:    "Evening Dim",
				Enabled: true,
				Time:    "sunset-30",
				Days:    []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"},
				Action:  "dim:30",
				Target:  "light:0",
			},
			{
				ID:      3,
				Name:    "Night Off",
				Enabled: true,
				Time:    "23:00",
				Days:    []string{"sun", "mon", "tue", "wed", "thu"},
				Action:  "off",
				Target:  "all",
			},
		},
		UseSunriseSunset: true,
		Latitude:         52.520008,
		Longitude:        13.404954,
		Timezone:         "Europe/Berlin",
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal ScheduleConfig: %v", err)
	}

	// Test JSON unmarshaling
	var decoded ScheduleConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ScheduleConfig: %v", err)
	}

	// Verify fields
	if decoded.Enabled != config.Enabled {
		t.Errorf("Enabled mismatch: got %v, want %v", decoded.Enabled, config.Enabled)
	}
	if len(decoded.Schedules) != 3 {
		t.Fatalf("Expected 3 schedules, got %d", len(decoded.Schedules))
	}
	if decoded.Schedules[0].Name != "Morning On" {
		t.Errorf("Schedule[0].Name mismatch: got %s, want Morning On", decoded.Schedules[0].Name)
	}
	if decoded.Schedules[1].Time != "sunset-30" {
		t.Errorf("Schedule[1].Time mismatch: got %s, want sunset-30", decoded.Schedules[1].Time)
	}
	if decoded.UseSunriseSunset != config.UseSunriseSunset {
		t.Errorf("UseSunriseSunset mismatch: got %v, want %v", decoded.UseSunriseSunset, config.UseSunriseSunset)
	}
	if decoded.Latitude != config.Latitude {
		t.Errorf("Latitude mismatch: got %f, want %f", decoded.Latitude, config.Latitude)
	}
}

// TestEnergyMeterConfig tests energy meter configuration
func TestEnergyMeterConfig(t *testing.T) {
	dailyLimit := 10.5
	monthlyLimit := 300.0

	config := EnergyMeterConfig{
		ReportingInterval: 300,
		RetentionDays:     365,
		DailyLimitKWh:     &dailyLimit,
		MonthlyLimitKWh:   &monthlyLimit,
		AlertEmail:        "admin@example.com",
		Tariffs: []EnergyTariff{
			{
				Name:        "Day Rate",
				StartTime:   "06:00",
				EndTime:     "22:00",
				Days:        []string{"mon", "tue", "wed", "thu", "fri"},
				PricePerKWh: 0.30,
			},
			{
				Name:        "Night Rate",
				StartTime:   "22:00",
				EndTime:     "06:00",
				Days:        []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"},
				PricePerKWh: 0.15,
			},
			{
				Name:        "Weekend Rate",
				StartTime:   "00:00",
				EndTime:     "23:59",
				Days:        []string{"sat", "sun"},
				PricePerKWh: 0.20,
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal EnergyMeterConfig: %v", err)
	}

	// Test JSON unmarshaling
	var decoded EnergyMeterConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal EnergyMeterConfig: %v", err)
	}

	// Verify fields
	if decoded.ReportingInterval != config.ReportingInterval {
		t.Errorf("ReportingInterval mismatch: got %d, want %d", decoded.ReportingInterval, config.ReportingInterval)
	}
	if decoded.DailyLimitKWh == nil || *decoded.DailyLimitKWh != dailyLimit {
		t.Errorf("DailyLimitKWh mismatch: got %v, want %f", decoded.DailyLimitKWh, dailyLimit)
	}
	if len(decoded.Tariffs) != 3 {
		t.Fatalf("Expected 3 tariffs, got %d", len(decoded.Tariffs))
	}
	if decoded.Tariffs[0].Name != "Day Rate" {
		t.Errorf("Tariff[0].Name mismatch: got %s, want Day Rate", decoded.Tariffs[0].Name)
	}
	if decoded.Tariffs[1].PricePerKWh != 0.15 {
		t.Errorf("Tariff[1].PricePerKWh mismatch: got %f, want 0.15", decoded.Tariffs[1].PricePerKWh)
	}
}

// TestSensorConfig tests sensor configuration
func TestSensorConfig(t *testing.T) {
	tempMin := 10.0
	tempMax := 35.0
	humidityMin := 30.0
	humidityMax := 70.0
	luxMin := 100.0
	luxMax := 10000.0

	config := SensorConfig{
		TempUnit:          "C",
		TempOffset:        -0.5,
		TempReporting:     60,
		HumidityOffset:    2.0,
		HumidityReporting: 120,
		LuxOffset:         -10.0,
		LuxReporting:      300,
		TempMin:           &tempMin,
		TempMax:           &tempMax,
		HumidityMin:       &humidityMin,
		HumidityMax:       &humidityMax,
		LuxMin:            &luxMin,
		LuxMax:            &luxMax,
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal SensorConfig: %v", err)
	}

	// Test JSON unmarshaling
	var decoded SensorConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal SensorConfig: %v", err)
	}

	// Verify fields
	if decoded.TempUnit != config.TempUnit {
		t.Errorf("TempUnit mismatch: got %s, want %s", decoded.TempUnit, config.TempUnit)
	}
	if decoded.TempOffset != config.TempOffset {
		t.Errorf("TempOffset mismatch: got %f, want %f", decoded.TempOffset, config.TempOffset)
	}
	if decoded.TempMin == nil || *decoded.TempMin != tempMin {
		t.Errorf("TempMin mismatch: got %v, want %f", decoded.TempMin, tempMin)
	}
	if decoded.HumidityMax == nil || *decoded.HumidityMax != humidityMax {
		t.Errorf("HumidityMax mismatch: got %v, want %f", decoded.HumidityMax, humidityMax)
	}
	if decoded.LuxReporting != config.LuxReporting {
		t.Errorf("LuxReporting mismatch: got %d, want %d", decoded.LuxReporting, config.LuxReporting)
	}
}
