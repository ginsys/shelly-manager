# Shelly Gen1 Device Capabilities

## Device Types and Their Specific Features

### 🔌 Basic Relay/Switch Devices

#### **Shelly 1**
- **Type ID**: `SHSW-1`
- **Channels**: 1 relay (dry contact)
- **Features**: 
  - Single relay control
  - No power metering
  - Supports DC power (12-60V DC) or AC (110-240V AC)
- **API Endpoints**:
  - ✅ `/relay/0` - Control relay
  - ✅ `/settings/relay/0` - Configure relay
  - ❌ Power metering - N/A

#### **Shelly 1PM**
- **Type ID**: `SHSW-PM`
- **Channels**: 1 relay with power metering
- **Features**:
  - Single relay control
  - Power consumption monitoring
  - Overpower/overtemperature protection
- **API Endpoints**:
  - ✅ `/relay/0` - Control relay
  - ✅ `/settings/relay/0` - Configure relay
  - ✅ `/meter/0` - Power consumption data
  - ⚠️ `/settings/max_power` - Set power limit (partial)

#### **Shelly 1L**
- **Type ID**: `SHSW-L`
- **Channels**: 1 relay (no neutral required)
- **Features**:
  - Works without neutral wire
  - Lower power handling (4.1A max)
  - No power metering
- **API Endpoints**:
  - ✅ `/relay/0` - Control relay
  - ✅ `/settings/relay/0` - Configure relay

#### **Shelly 2.5**
- **Type ID**: `SHSW-25`
- **Channels**: 2 relays OR 1 roller shutter
- **Features**:
  - Dual relay mode or roller shutter mode
  - Power metering on both channels
  - Overpower/overtemperature protection
- **API Endpoints**:
  - ✅ `/relay/0`, `/relay/1` - Control relays (relay mode)
  - ❌ `/roller/0` - Control roller (roller mode)
  - ✅ `/meter/0`, `/meter/1` - Power consumption
  - ✅ `/settings/roller/0` - Roller configuration (written and read back by apply)
  - ✅ `/settings/relay/0`, `/settings/relay/1` - Relay configuration
  - ✅ `/settings` `mode=relay|roller` - Switch between relay/roller mode (`system.device.mode`; apply writes it first and reads it back, unconfirmed on hardware)

### 💡 Lighting Control Devices

#### **Shelly Dimmer 2**
- **Type ID**: `SHDM-2`
- **Channels**: 1 dimmable output
- **Features**:
  - Brightness control (1-100%)
  - Leading/trailing edge configuration
  - Power metering
  - Calibration mode
- **API Endpoints**:
  - ❌ `/light/0` - Control brightness
  - ✅ `/settings/light/0` - Channel settings (`light.lights`: apply writes the keys that are set and reads them back, unconfirmed on hardware; the device-level settings are applied on `POST /settings` (`dimming`, with `leading_edge` only where the device reports `pulse_mode_detected`); night mode and warm-up are written to `/settings/night_mode` and `/settings/warm_up` and read back from `GET /settings`, unconfirmed on hardware)
  - ✅ `/meter/0` - Power consumption
  - ❌ `/settings/light/0/calibration` - Calibration settings

#### **Shelly RGBW2**
- **Type ID**: `SHRGBW2`
- **Channels**: 4 PWM outputs
- **Features**:
  - RGB + White control
  - Multiple modes: Color, White, 4x White
  - Effects support
  - Power metering
- **API Endpoints**:
  - ❌ `/color/0` - RGB control (color mode)
  - ❌ `/white/0-3` - White channel control
  - ✅ `/settings/color/0` - Color settings (`light.lights`: apply writes the keys that are set and reads them back, unconfirmed on hardware)
  - ✅ `/settings/white/0-3` - White settings (same, one entry per channel; `btn_type` and `btn_reverse` on channel 0 only)
  - ✅ `/meter/0` - Power consumption
  - ✅ `/settings` `mode=color|white` - Switch modes (`system.device.mode`; apply writes it first and reads it back, unconfirmed on hardware; the page has no `/settings/mode` path)

#### **Shelly Bulb**
- **Type ID**: `SHBLB-1` (the Duo `SHBDUO-1` and the Vintage `SHVIN-1` are not supported)
- **Channels**: 1 light channel, mode white or color
- **Features**:
  - RGBW LEDs in color mode, color temperature and brightness in white mode
  - Seven effects (0 to 6)
- **API Endpoints**:
  - ❌ `/light/0` - Control (color, brightness, temperature, timer)
  - ✅ `/settings/light/0` - Channel settings (`light.lights`: `default_state`, `auto_on`, `auto_off`, `effect`, `schedule`, `schedule_rules`; apply writes the keys that are set and reads them back, unconfirmed on hardware; `reset` is never sent)
  - ✅ `/settings` `mode=white|color` - Switch modes (`system.device.mode`; apply writes it first and reads it back, unconfirmed on hardware)

### 🏠 Other Devices

#### **Shelly i3**
- **Type ID**: `SHIX3-1`
- **Channels**: 3 inputs
- **Features**:
  - Input detection (short, long, double, triple press)
  - Scene activation
  - No outputs (input only)
- **API Endpoints**:
  - ✅ `/status` - Input states
  - ✅ `/settings/input/0-2` - Configure inputs (button type and reverse flag)
  - ❌ `/settings/actions` - Not written by the manager for this model yet: its action names are not confirmed, so a desired configuration with actions is refused for it

#### **Shelly Plug/Plug S**
- **Type ID**: `SHPLG-1`, `SHPLG-S`
- **Channels**: 1 relay
- **Features**:
  - Plug-in form factor
  - Power metering (Plug S)
  - LED ring indicator
- **API Endpoints**:
  - ✅ `/relay/0` - Control relay
  - ✅ `/meter/0` - Power consumption (Plug S only)
  - ❌ `/settings/led` - LED configuration

### 🌡️ Sensors (Limited Control)

#### **Shelly H&T**
- **Type ID**: `SHHT-1`
- **Features**: Temperature & humidity sensor
- **Note**: Battery powered, sleeps most of the time

#### **Shelly Flood**
- **Type ID**: `SHWT-1`
- **Features**: Water leak detection
- **Note**: Battery powered, wake on event

#### **Shelly Door/Window 2**
- **Type ID**: `SHDW-2`
- **Features**: Open/close detection, vibration, tilt
- **Note**: Battery powered, wake on event

## Implementation Status Summary

### ✅ Currently Implemented
- Basic relay control (on/off)
- Status reading
- Power meter reading
- Basic configuration reading

### ⚠️ Partially Implemented
- Configuration writing
- Authentication setup
- Advanced relay settings

### ❌ Not Implemented
- Dimmer control
- RGB/White control
- Roller shutter control
- Input configuration
- LED control
- Effects
- Calibration
- Mode switching
- Schedule management
- Scene control

## Priority Implementation Order

To support the most common devices first, I recommend:

1. **Complete Relay Devices** (Shelly 1, 1PM, 2.5)
   - Finish configuration writing
   - Add overpower protection settings
   - Complete authentication

2. **Roller Shutter Support** (Shelly 2.5 in roller mode)
   - Position control
   - Calibration
   - Safety features

3. **Dimmer Support** (Shelly Dimmer 2)
   - Brightness control
   - Calibration
   - Transition effects

4. **RGBW Support** (Shelly RGBW2)
   - Color control
   - White control
   - Mode switching
   - Effects

## Which Devices Do You Own?

Please let me know which specific Gen1 devices you have, so we can prioritize their implementation:

- [ ] Shelly 1
- [ ] Shelly 1PM
- [ ] Shelly 1L
- [ ] Shelly 2.5
- [ ] Shelly Dimmer 2
- [ ] Shelly RGBW2
- [ ] Shelly i3
- [ ] Shelly Plug/Plug S
- [ ] Other: ___________