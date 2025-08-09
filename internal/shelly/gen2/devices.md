# Shelly Gen2+ Device Capabilities

## Device Types and Their Specific Features

> **Note**: Gen2+ devices use JSON-RPC protocol instead of REST endpoints

### 🔌 Plus Series - Relay/Switch Devices

#### **Shelly Plus 1**
- **Model ID**: `shellyplus1`
- **Generation**: 2
- **Channels**: 1 relay (dry contact)
- **Features**:
  - Single relay control
  - No power metering
  - Bluetooth support
  - Scripting support (mJS)
- **RPC Methods**:
  - ✅ `Switch.GetStatus` - Get switch status
  - ✅ `Switch.Set` - Control switch
  - ✅ `Switch.GetConfig` - Get configuration
  - ⚠️ `Switch.SetConfig` - Set configuration
  - ✅ `Shelly.GetDeviceInfo` - Device information
  - ✅ `Shelly.GetStatus` - Full status

#### **Shelly Plus 1PM**
- **Model ID**: `shellyplus1pm`
- **Generation**: 2
- **Channels**: 1 relay with power metering
- **Features**:
  - Single relay control
  - Real-time power monitoring
  - Energy accumulation
  - Overpower/overtemperature protection
  - Bluetooth support
- **RPC Methods**:
  - ✅ `Switch.GetStatus` - Get switch status with power
  - ✅ `Switch.Set` - Control switch
  - ❌ `Switch.ResetCounters` - Reset energy counters
  - ❌ `PM.GetStatus` - Detailed power metrics
  - ⚠️ `Switch.SetConfig` - Set configuration

#### **Shelly Plus 2PM**
- **Model ID**: `shellyplus2pm`
- **Generation**: 2
- **Channels**: 2 relays OR 1 roller/cover
- **Features**:
  - Dual relay mode or roller shutter mode
  - Power metering on both channels
  - Overpower/overtemperature protection
  - Mode switching (relay/cover)
- **RPC Methods**:
  - ✅ `Switch.GetStatus` - Get switch status (relay mode)
  - ✅ `Switch.Set` - Control switches
  - ❌ `Cover.GetStatus` - Get cover position
  - ❌ `Cover.Open` - Open cover
  - ❌ `Cover.Close` - Close cover
  - ❌ `Cover.Stop` - Stop cover
  - ❌ `Cover.GoToPosition` - Set position
  - ❌ `Cover.Calibrate` - Calibration
  - ❌ `Sys.SetConfig` - Mode switching

### 💡 Plus Series - Lighting Control

#### **Shelly Plus Wall Dimmer**
- **Model ID**: `shellyplusiw4`
- **Generation**: 2
- **Features**:
  - Wall-mounted dimmer
  - Brightness control
  - Leading/trailing edge
- **RPC Methods**:
  - ❌ `Light.GetStatus` - Get light status
  - ❌ `Light.Set` - Set brightness
  - ❌ `Light.GetConfig` - Get configuration
  - ❌ `Light.SetConfig` - Set configuration

#### **Shelly Plus 0-10V Dimmer**
- **Model ID**: `shellyplus010v`
- **Generation**: 2
- **Features**:
  - 0-10V dimming control
  - Industrial standard
- **RPC Methods**:
  - ❌ `Light.GetStatus` - Get status
  - ❌ `Light.Set` - Set level (0-10V)
  - ❌ `Light.GetConfig` - Get configuration

#### **Shelly Plus RGBW PM**
- **Model ID**: `shellyplusrgbwpm`
- **Generation**: 2
- **Channels**: 4 PWM outputs
- **Features**:
  - 3 profiles: RGBW, RGB, 4x White
  - Power monitoring
  - Effects support
  - Min/Max brightness settings
- **RPC Methods**:
  - ❌ `RGBW.GetStatus` - Get RGBW status
  - ❌ `RGBW.Set` - Set colors/brightness
  - ❌ `RGBW.SetConfig` - Configure mode
  - ❌ `RGB.GetStatus` - RGB mode status
  - ❌ `RGB.Set` - Set RGB values
  - ❌ `Light.GetStatus` - White mode status
  - ❌ `Light.Set` - Set white channels

### 🎛️ Plus Series - Input/Control Devices

#### **Shelly Plus i4**
- **Model ID**: `shellyplusi4`
- **Generation**: 2
- **Channels**: 4 digital inputs
- **Features**:
  - Input-only device (no relays)
  - Scene activation
  - Multi-action support (up to 4 per button)
  - Event types: single, double, triple, long
- **RPC Methods**:
  - ✅ `Input.GetStatus` - Get input states
  - ❌ `Input.GetConfig` - Get input configuration
  - ❌ `Input.SetConfig` - Configure inputs
  - ❌ `Webhook.Create` - Setup webhooks for events

#### **Shelly Plus i4 DC**
- **Model ID**: `shellyplusi4dc`
- **Generation**: 2
- **Features**:
  - DC powered version of Plus i4
  - 5-24V DC input
- **RPC Methods**: Same as Plus i4

### 🔌 Plus Series - Other Devices

#### **Shelly Plus Plug S**
- **Model ID**: `shellyplusplugs`
- **Generation**: 2
- **Features**:
  - Plug-in form factor
  - Power monitoring
  - LED ring indicator
  - Bluetooth support
- **RPC Methods**:
  - ✅ `Switch.GetStatus` - Get switch status
  - ✅ `Switch.Set` - Control switch
  - ❌ `PM.GetStatus` - Power metrics
  - ❌ `LED.GetConfig` - LED configuration
  - ❌ `LED.SetConfig` - Configure LED

#### **Shelly Plus H&T**
- **Model ID**: `shellyplusht`
- **Generation**: 2
- **Features**:
  - Temperature & humidity sensor
  - Battery powered
  - Bluetooth support
- **RPC Methods**:
  - ❌ `Temperature.GetStatus` - Get temperature
  - ❌ `Humidity.GetStatus` - Get humidity
  - ❌ `DevicePower.GetStatus` - Battery status

### 🏭 Pro Series - Professional DIN Rail Devices

#### **Shelly Pro 1**
- **Model ID**: `shellypro1`
- **Generation**: 2
- **Features**:
  - DIN rail mount
  - Ethernet + WiFi
  - Potential-free contacts
  - Professional grade
  - 5-year warranty
- **RPC Methods**:
  - ✅ `Switch.GetStatus` - Get switch status
  - ✅ `Switch.Set` - Control switch
  - ✅ `Eth.GetStatus` - Ethernet status
  - ❌ `Eth.GetConfig` - Ethernet configuration
  - ❌ `Eth.SetConfig` - Configure ethernet

#### **Shelly Pro 1PM**
- **Model ID**: `shellypro1pm`
- **Generation**: 2
- **Features**:
  - DIN rail mount
  - Ethernet + WiFi
  - Power monitoring
  - Overpower protection
- **RPC Methods**:
  - All Pro 1 methods plus:
  - ❌ `PM.GetStatus` - Power metrics
  - ❌ `PM.GetData` - Historical data
  - ❌ `PM.ResetCounters` - Reset counters

#### **Shelly Pro 2PM**
- **Model ID**: `shellypro2pm`
- **Generation**: 2
- **Channels**: 2 relays
- **Features**:
  - DIN rail mount
  - Dual relay with power monitoring
  - Ethernet + WiFi
  - Professional grade
- **RPC Methods**:
  - ✅ `Switch.GetStatus` - Get switch status (id: 0,1)
  - ✅ `Switch.Set` - Control switches
  - ❌ `PM.GetStatus` - Power metrics per channel

#### **Shelly Pro 3**
- **Model ID**: `shellypro3`
- **Generation**: 2
- **Channels**: 3 relays
- **Features**:
  - DIN rail mount
  - 3 independent circuits
  - Can handle 3-phase systems
  - Ethernet + WiFi
- **RPC Methods**:
  - ✅ `Switch.GetStatus` - Get switch status (id: 0,1,2)
  - ✅ `Switch.Set` - Control switches
  - ❌ `Sys.GetConfig` - System configuration

#### **Shelly Pro 4PM**
- **Model ID**: `shellypro4pm`
- **Generation**: 2
- **Channels**: 4 relays
- **Features**:
  - DIN rail mount
  - 4x16A relay outputs
  - Power monitoring per channel
  - Ethernet + WiFi
  - Scripting support
- **RPC Methods**:
  - ✅ `Switch.GetStatus` - Get switch status (id: 0,1,2,3)
  - ✅ `Switch.Set` - Control switches
  - ❌ `PM.GetStatus` - Power metrics per channel
  - ❌ `Script.List` - List scripts
  - ❌ `Script.Start` - Start script
  - ❌ `Script.Stop` - Stop script

#### **Shelly Pro 3EM**
- **Model ID**: `shellypro3em`
- **Generation**: 2
- **Features**:
  - DIN rail mount
  - 3-phase energy meter
  - No built-in relays
  - 1% accuracy
  - 60-day data storage
  - Real-time clock
  - Ethernet + WiFi
- **RPC Methods**:
  - ❌ `EM.GetStatus` - Get energy data
  - ❌ `EM.GetData` - Historical data (1-min intervals)
  - ❌ `EMData.GetRecords` - Retrieve stored records
  - ❌ `EM.ResetCounters` - Reset accumulators

### 🔧 Common Gen2+ Features

#### **System Methods** (All devices)
- ✅ `Shelly.GetDeviceInfo` - Device information
- ✅ `Shelly.GetStatus` - Full device status
- ✅ `Shelly.GetConfig` - Full configuration
- ⚠️ `Shelly.SetConfig` - Update configuration
- ❌ `Shelly.ListMethods` - List available methods
- ❌ `Shelly.CheckForUpdate` - Check firmware
- ❌ `Shelly.Update` - Update firmware
- ❌ `Shelly.Reboot` - Reboot device
- ❌ `Shelly.FactoryReset` - Factory reset
- ❌ `Shelly.ResetWiFiConfig` - Reset WiFi
- ❌ `Shelly.ResetAuthConfig` - Reset auth
- ❌ `Shelly.SetAuth` - Set authentication

#### **WiFi Methods**
- ✅ `WiFi.GetStatus` - WiFi status
- ❌ `WiFi.GetConfig` - WiFi configuration
- ❌ `WiFi.SetConfig` - Configure WiFi
- ❌ `WiFi.Scan` - Scan networks

#### **Bluetooth Methods** (Plus/Pro devices)
- ❌ `BLE.GetStatus` - Bluetooth status
- ❌ `BLE.GetConfig` - Bluetooth configuration
- ❌ `BLE.SetConfig` - Configure Bluetooth

#### **Cloud Methods**
- ❌ `Cloud.GetStatus` - Cloud connection status
- ❌ `Cloud.GetConfig` - Cloud configuration
- ❌ `Cloud.SetConfig` - Configure cloud

#### **Scripting** (Selected devices)
- ❌ `Script.Create` - Create script
- ❌ `Script.GetCode` - Get script code
- ❌ `Script.PutCode` - Update script code
- ❌ `Script.List` - List scripts
- ❌ `Script.Start` - Start script
- ❌ `Script.Stop` - Stop script
- ❌ `Script.Delete` - Delete script

#### **Webhooks**
- ❌ `Webhook.Create` - Create webhook
- ❌ `Webhook.Update` - Update webhook
- ❌ `Webhook.Delete` - Delete webhook
- ❌ `Webhook.List` - List webhooks

## Implementation Status Summary

### ✅ Currently Implemented
- Basic device info retrieval
- Switch control (on/off)
- Status reading
- Basic configuration reading

### ⚠️ Partially Implemented
- Configuration writing
- Authentication (using basic instead of digest)

### ❌ Not Implemented
- Power monitoring details
- Cover/roller control
- Light/dimmer control
- RGB/White control
- Input configuration
- Ethernet configuration
- Scripting support
- Webhook management
- Cloud configuration
- Bluetooth configuration
- Energy meter operations
- Historical data retrieval
- Firmware updates

## Key Differences from Gen1

1. **Protocol**: JSON-RPC instead of REST
2. **Authentication**: Digest auth (RFC 2617) instead of basic auth
3. **Components**: Component-based architecture (Switch:0, Input:1, etc.)
4. **Scripting**: mJS scripting support
5. **Connectivity**: Bluetooth + Ethernet (Pro) in addition to WiFi
6. **Events**: WebSocket event streaming
7. **Storage**: Local data storage (Pro 3EM: 60 days)

## Priority Implementation Order

1. **Complete RPC Infrastructure**
   - Proper digest authentication
   - WebSocket support for events
   - Error handling for RPC errors

2. **Switch/Relay Devices** (Plus 1/1PM/2PM, Pro series)
   - Complete configuration management
   - Power monitoring details
   - Overpower protection settings

3. **Cover/Roller Support** (Plus 2PM)
   - Position control
   - Calibration
   - Safety features

4. **Energy Monitoring** (Pro 3EM)
   - Real-time data
   - Historical data retrieval
   - Multi-phase support

5. **Advanced Features**
   - Scripting support
   - Webhook management
   - Ethernet configuration (Pro)

## Notes

- All Gen2+ devices support TLS encryption
- Pro devices include 5-year warranty vs 3-year for Plus
- Pro devices are DIN rail mountable for professional installations
- Ethernet on Pro devices requires power-off for cable changes
- Some devices support multiple operation modes (e.g., Plus 2PM: relay vs cover)