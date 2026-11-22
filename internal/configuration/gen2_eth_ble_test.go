package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly/gen2"
)

// The production client is *gen2.Client (service.go builds it with gen2.NewClient), so the guard's
// status read is there. A wrapper that hid the method would refuse every Ethernet change.
var _ Gen2EthStatusClient = (*gen2.Client)(nil)

// ethBleDevice is a Pro device: its document holds eth and ble, and Eth.GetStatus answers.
type ethBleDevice struct {
	*fakeGen2Device
	status      map[string]interface{}
	statusErr   error
	statusCalls int
	wifi        map[string]interface{}
	wifiErr     error
}

func (d *ethBleDevice) GetWiFiStatus(context.Context) (map[string]interface{}, error) {
	return d.wifi, d.wifiErr
}

func (d *ethBleDevice) GetEthStatus(context.Context) (map[string]interface{}, error) {
	d.statusCalls++
	return d.status, d.statusErr
}

// newEthBleDevice is a Pro device that the manager reaches at managerIP, with Ethernet on static
// 192.168.1.60 and the status ethIP (nil is no address on Ethernet).
func newEthBleDevice(managerIP string, ethIP interface{}) *ethBleDevice {
	base := fakeGen2DeviceWithChannels()
	base.ip = managerIP
	base.doc["eth"] = map[string]interface{}{
		"enable": true, "server_mode": false, "ipv4mode": "static",
		"ip": "192.168.1.60", "netmask": "255.255.255.0", "gw": "192.168.1.1", "nameserver": nil,
	}
	base.doc["ble"] = map[string]interface{}{"rpc": map[string]interface{}{"enable": true, "keep_running": false}}
	// The Wi-Fi station address defaults to the one the manager dials, so a change is allowed where the
	// Ethernet address differs from it; a test that needs another sets dev.wifi.
	return &ethBleDevice{fakeGen2Device: base, status: map[string]interface{}{"ip": ethIP}, wifi: map[string]interface{}{"sta_ip": managerIP}}
}

func ethDesired(enable *bool, mode, ip, netmask, gw *string) *DeviceConfiguration {
	eth := &EthernetConfig{Enable: enable, IPv4Mode: mode}
	if ip != nil || netmask != nil || gw != nil {
		eth.StaticIP = &StaticIPConfig{IP: ip, Netmask: netmask, Gateway: gw}
	}
	return &DeviceConfiguration{Network: &NetworkConfiguration{Ethernet: eth}}
}

func bleDesired(b BLEConfig) *DeviceConfiguration {
	return &DeviceConfiguration{System: &SystemConfiguration{Device: &TypedDeviceConfig{BleConfig: &b}}}
}

func applyTo(t *testing.T, client ShellyClient, desired *DeviceConfiguration) (*ApplyVerifyResult, error) {
	t.Helper()
	svc, _ := newGen2Service(t, desired)
	return svc.ApplyDesiredConfig(context.Background(), 1, client, "SPSW-202XE16EU")
}

func ethCalls(d *fakeGen2Device) []gen2Call {
	var calls []gen2Call
	for _, c := range d.calls {
		if c.method == "Eth.SetConfig" {
			calls = append(calls, c)
		}
	}
	return calls
}

func TestApplyDesiredConfig_Gen2WritesEthernetAndOwesTheRestart(t *testing.T) {
	dev := newEthBleDevice("192.168.1.99", nil)
	desired := ethDesired(BoolPtr(true), StringPtr("static"), StringPtr("192.168.1.70"), StringPtr("255.255.255.0"), StringPtr("192.168.1.1"))

	result, err := applyTo(t, dev, desired)

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "the Ethernet settings read back")
	calls := ethCalls(dev.fakeGen2Device)
	require.Len(t, calls, 1)
	assert.Equal(t, map[string]interface{}{
		"enable": true, "ipv4mode": "static", "ip": "192.168.1.70", "netmask": "255.255.255.0", "gw": "192.168.1.1",
	}, calls[0].config, "only the keys set")
	assert.True(t, result.ApplyResult.RequiresReboot, "every Ethernet change needs a restart, whatever the answer says")
	assert.Equal(t, "192.168.1.70", dev.doc["eth"].(map[string]interface{})["ip"])
}

// A write that changes nothing owes no restart from this rule.
func TestApplyDesiredConfig_Gen2AnUnchangedEthernetOwesNoRestart(t *testing.T) {
	dev := newEthBleDevice("192.168.1.60", "192.168.1.60")
	desired := ethDesired(BoolPtr(true), StringPtr("static"), StringPtr("192.168.1.60"), nil, nil)

	result, err := applyTo(t, dev, desired)

	require.NoError(t, err)
	require.Len(t, ethCalls(dev.fakeGen2Device), 1)
	assert.False(t, result.ApplyResult.RequiresReboot)
}

func TestApplyDesiredConfig_Gen2WritesBLERPCAndNeverTheObserver(t *testing.T) {
	dev := newEthBleDevice("192.168.1.99", nil)

	result, err := applyTo(t, dev, bleDesired(BLEConfig{RPC: &BLERPCConfig{Enable: BoolPtr(true), KeepRunning: BoolPtr(true)}}))

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, "BLE.SetConfig", dev.calls[0].method)
	assert.Equal(t, map[string]interface{}{"rpc": map[string]interface{}{"enable": true, "keep_running": true}}, dev.calls[0].config)
	assert.NotContains(t, dev.calls[0].config, "observer")
	assert.NotContains(t, dev.calls[0].config, "enable", "the top-level enable was removed in firmware 2.0.0")
}

func TestApplyDesiredConfig_Gen2RefusesBLEWordsItHasNoKeyFor(t *testing.T) {
	for name, tc := range map[string]struct {
		ble  BLEConfig
		path string
	}{
		"top-level enable": {BLEConfig{Enable: true}, "system.device.ble.enable"},
		"dst_addr":         {BLEConfig{RPC: &BLERPCConfig{DstAddr: "10.0.0.1"}}, "system.device.ble.rpc.dst_addr"},
		"listen_port":      {BLEConfig{RPC: &BLERPCConfig{ListenPort: 8080}}, "system.device.ble.rpc.listen_port"},
	} {
		t.Run(name, func(t *testing.T) {
			dev := newEthBleDevice("192.168.1.99", nil)

			_, err := applyTo(t, dev, bleDesired(tc.ble))

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings), err.Error())
			assert.Contains(t, err.Error(), tc.path)
			assert.Empty(t, dev.calls)
		})
	}
}

// A plain zero is unset: the stored ble.enable false asks for nothing.
func TestApplyDesiredConfig_Gen2ABLEEnableFalseIsNothing(t *testing.T) {
	dev := newEthBleDevice("192.168.1.99", nil)

	_, err := applyTo(t, dev, bleDesired(BLEConfig{}))

	require.NoError(t, err)
	assert.Empty(t, dev.calls)
}

// Pin: a stored document keeps its Gen1 rpc words through the new type.
func TestBLERPCConfig_KeepsTheStoredWords(t *testing.T) {
	const stored = `{"system":{"device":{"ble":{"enable":true,"rpc":{"dst_addr":"10.0.0.1","listen_port":8080,"enable":true,"keep_running":false}}}}}`
	var config DeviceConfiguration
	require.NoError(t, json.Unmarshal([]byte(stored), &config))
	rpc := config.System.Device.BleConfig.RPC
	assert.Equal(t, "10.0.0.1", rpc.DstAddr)
	assert.Equal(t, 8080, rpc.ListenPort)
	assert.True(t, *rpc.Enable)
	assert.False(t, *rpc.KeepRunning)
	out, err := json.Marshal(config)
	require.NoError(t, err)
	assert.JSONEq(t, stored, string(out))
}

func TestApplyDesiredConfig_Gen2EthernetStrandGuard(t *testing.T) {
	static := StringPtr("static")
	dhcp := StringPtr("dhcp")
	for name, tc := range map[string]struct {
		managerIP string
		statusIP  interface{}
		statusErr error
		noStatus  bool // the client has no Eth.GetStatus
		desired   *DeviceConfiguration
		path      string // the refused path, or "" when the write goes out
	}{
		"disabling Ethernet under the manager":   {"192.168.1.60", "192.168.1.60", nil, false, ethDesired(BoolPtr(false), nil, nil, nil, nil), "network.eth.enable"},
		"static to DHCP under the manager":       {"192.168.1.60", "192.168.1.60", nil, false, ethDesired(nil, dhcp, nil, nil, nil), "network.eth.ipv4mode"},
		"a new address under the manager":        {"192.168.1.60", "192.168.1.60", nil, false, ethDesired(nil, nil, StringPtr("192.168.1.70"), nil, nil), "network.eth.ip.ip"},
		"a new netmask under the manager":        {"192.168.1.60", "192.168.1.60", nil, false, ethDesired(nil, nil, nil, StringPtr("255.255.0.0"), nil), "network.eth.ip.netmask"},
		"a new gateway under the manager":        {"192.168.1.60", "192.168.1.60", nil, false, ethDesired(nil, nil, nil, nil, StringPtr("192.168.1.2")), "network.eth.ip.gw"},
		"the same address under the manager":     {"192.168.1.60", "192.168.1.60", nil, false, ethDesired(BoolPtr(true), static, StringPtr("192.168.1.60"), nil, nil), ""},
		"the manager's address in mapped form":   {"192.168.1.60", "::ffff:192.168.1.60", nil, false, ethDesired(BoolPtr(false), nil, nil, nil, nil), "network.eth.enable"},
		"the manager on another interface":       {"192.168.1.99", "192.168.1.60", nil, false, ethDesired(BoolPtr(false), nil, nil, nil, nil), ""},
		"no address on Ethernet":                 {"192.168.1.99", nil, nil, false, ethDesired(BoolPtr(false), nil, nil, nil, nil), ""},
		"the status cannot be read":              {"192.168.1.99", nil, errors.New("timeout"), false, ethDesired(BoolPtr(false), nil, nil, nil, nil), "network.eth.enable"},
		"a client with no status call":           {"192.168.1.99", nil, nil, true, ethDesired(BoolPtr(false), nil, nil, nil, nil), "network.eth.enable"},
		"the manager reaches it by a name":       {"shelly-pro.lan", "192.168.1.60", nil, false, ethDesired(BoolPtr(false), nil, nil, nil, nil), "network.eth.enable"},
		"the manager reaches it by an IPv6 name": {"2001:db8::1", "192.168.1.60", nil, false, ethDesired(BoolPtr(false), nil, nil, nil, nil), "network.eth.enable"},
		"a status address that is not IPv4":      {"192.168.1.99", "2001:db8::60", nil, false, ethDesired(BoolPtr(false), nil, nil, nil, nil), "network.eth.enable"},
		"a name server alone moves nothing":      {"192.168.1.60", "192.168.1.60", nil, false, &DeviceConfiguration{Network: &NetworkConfiguration{Ethernet: &EthernetConfig{StaticIP: &StaticIPConfig{Nameserver: StringPtr("192.168.1.1")}}}}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			dev := newEthBleDevice(tc.managerIP, tc.statusIP)
			dev.statusErr = tc.statusErr
			var client ShellyClient = dev
			if tc.noStatus {
				client = dev.fakeGen2Device
			}

			_, err := applyTo(t, client, tc.desired)

			if tc.path == "" {
				require.NoError(t, err)
				assert.Len(t, ethCalls(dev.fakeGen2Device), 1)
				return
			}
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings), err.Error())
			assert.Contains(t, err.Error(), tc.path)
			assert.Empty(t, dev.calls, "nothing is written")
		})
	}
}

// A moving change is allowed on proof only: no Ethernet address, or the dialled address is the Wi-Fi
// station address. An address that differs from the Ethernet one proves nothing (a forwarded or
// translated address reads the same way), so it is refused unless the station address matches.
func TestApplyDesiredConfig_Gen2EthernetNeedsProofTheManagerIsOnWiFi(t *testing.T) {
	for name, tc := range map[string]struct {
		wifi    map[string]interface{}
		wifiErr error
		refused bool
	}{
		"the dialled address is the station address":     {map[string]interface{}{"sta_ip": "192.168.1.99"}, nil, false},
		"a forwarded address equal to neither":           {map[string]interface{}{"sta_ip": "192.168.1.50"}, nil, true},
		"the station has no address":                     {map[string]interface{}{"sta_ip": nil}, nil, true},
		"the station address is not IPv4":                {map[string]interface{}{"sta_ip": "2001:db8::50"}, nil, true},
		"the station address is missing from the status": {map[string]interface{}{}, nil, true},
		"the Wi-Fi status cannot be read":                {nil, errors.New("timeout"), true},
	} {
		t.Run(name, func(t *testing.T) {
			dev := newEthBleDevice("192.168.1.99", "192.168.1.60")
			dev.wifi, dev.wifiErr = tc.wifi, tc.wifiErr

			_, err := applyTo(t, dev, ethDesired(BoolPtr(false), nil, nil, nil, nil))

			if !tc.refused {
				require.NoError(t, err)
				assert.Len(t, ethCalls(dev.fakeGen2Device), 1)
				return
			}
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings), err.Error())
			assert.Contains(t, err.Error(), "network.eth.enable")
			assert.Empty(t, dev.calls, "nothing is written")
		})
	}
}

// A device in server mode is not the client the stored model describes: every desired Ethernet key is refused.
func TestApplyDesiredConfig_Gen2RefusesEthernetOfADeviceInServerMode(t *testing.T) {
	dev := newEthBleDevice("192.168.1.99", nil)
	dev.doc["eth"].(map[string]interface{})["server_mode"] = true

	_, err := applyTo(t, dev, ethDesired(BoolPtr(false), nil, nil, nil, nil))

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings), err.Error())
	assert.Contains(t, err.Error(), "network.eth")
	assert.Contains(t, err.Error(), "server_mode")
	assert.Empty(t, dev.calls)
}

// Two address moves in one apply can leave the device with no known address: one at a time.
func TestApplyDesiredConfig_Gen2RefusesAnEthernetChangeWithAStationChange(t *testing.T) {
	dev := newEthBleDevice("192.168.1.99", nil)
	desired := ethDesired(BoolPtr(false), nil, nil, nil, nil)
	desired.WiFi = &WiFiConfiguration{SSID: StringPtr("other"), Password: StringPtr("correct-horse-battery")}

	_, err := applyTo(t, dev, desired)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings), err.Error())
	assert.Contains(t, err.Error(), "network.eth")
	assert.Contains(t, err.Error(), "one at a time")
	assert.Empty(t, dev.calls)
}

// A station password is not a network move, but a wrong one strands a device that is reached over Wi-Fi
// just the same, so it is held back from an Ethernet change like a Wi-Fi network change is.
func TestApplyDesiredConfig_Gen2RefusesAnEthernetChangeWithAStationPassword(t *testing.T) {
	dev := newEthBleDevice("192.168.1.99", nil)
	desired := ethDesired(BoolPtr(false), nil, nil, nil, nil)
	desired.WiFi = &WiFiConfiguration{Password: StringPtr("correct-horse-battery")}

	_, err := applyTo(t, dev, desired)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings), err.Error())
	assert.Contains(t, err.Error(), "network.eth")
	assert.Contains(t, err.Error(), "one at a time")
	assert.Empty(t, dev.calls)
}

func TestApplyDesiredConfig_Gen2RefusesStaticEthernetItCannotAddress(t *testing.T) {
	static := StringPtr("static")
	for name, tc := range map[string]struct {
		live    map[string]interface{} // keys of the live eth section to replace
		desired *DeviceConfiguration
		path    string
	}{
		"no gateway": {map[string]interface{}{"ipv4mode": "dhcp", "ip": nil, "netmask": nil, "gw": nil},
			ethDesired(nil, static, StringPtr("192.168.1.70"), nil, nil), "network.eth.ip.gw"},
		"no address": {map[string]interface{}{"ipv4mode": "dhcp", "ip": nil, "netmask": nil, "gw": nil},
			ethDesired(nil, static, nil, nil, StringPtr("192.168.1.1")), "network.eth.ip.ip"},
		"an invalid address": {nil, ethDesired(nil, nil, StringPtr("300.1.1.1"), nil, nil), "network.eth.ip.ip"},
		"an IPv6 gateway":    {nil, ethDesired(nil, nil, nil, nil, StringPtr("fe80::1")), "network.eth.ip.gw"},
		"an unknown mode":    {nil, ethDesired(nil, StringPtr("pppoe"), nil, nil, nil), "network.eth.ipv4mode"},
	} {
		t.Run(name, func(t *testing.T) {
			dev := newEthBleDevice("192.168.1.99", nil)
			for key, value := range tc.live {
				dev.doc["eth"].(map[string]interface{})[key] = value
			}

			_, err := applyTo(t, dev, tc.desired)

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings), err.Error())
			assert.Contains(t, err.Error(), tc.path)
			assert.Empty(t, dev.calls)
		})
	}
}

// A gateway the apply leaves alone and the device holds satisfies the rule.
func TestApplyDesiredConfig_Gen2StaticEthernetUsesTheLiveGateway(t *testing.T) {
	dev := newEthBleDevice("192.168.1.99", nil)

	_, err := applyTo(t, dev, ethDesired(nil, nil, StringPtr("192.168.1.70"), nil, nil))

	require.NoError(t, err)
	require.Len(t, ethCalls(dev.fakeGen2Device), 1)
}

func TestApplyDesiredConfig_Gen2RefusesWhatTheLiveDeviceDoesNotReport(t *testing.T) {
	t.Run("no eth component", func(t *testing.T) {
		dev := newEthBleDevice("192.168.1.99", nil)
		delete(dev.doc, "eth")

		_, err := applyTo(t, dev, ethDesired(BoolPtr(false), nil, nil, nil, nil))

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrUnsupportedSettings), err.Error())
		assert.Contains(t, err.Error(), "network.eth")
		assert.Empty(t, dev.calls)
	})
	t.Run("no ble component", func(t *testing.T) {
		dev := newEthBleDevice("192.168.1.99", nil)
		delete(dev.doc, "ble")

		_, err := applyTo(t, dev, bleDesired(BLEConfig{RPC: &BLERPCConfig{Enable: BoolPtr(true)}}))

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrUnsupportedSettings), err.Error())
		assert.Contains(t, err.Error(), "system.device.ble")
		assert.Empty(t, dev.calls)
	})
	t.Run("a key absent from the live eth section, with its device key beside the path", func(t *testing.T) {
		dev := newEthBleDevice("192.168.1.99", nil)
		delete(dev.doc["eth"].(map[string]interface{}), "nameserver")

		_, err := applyTo(t, dev, &DeviceConfiguration{Network: &NetworkConfiguration{Ethernet: &EthernetConfig{StaticIP: &StaticIPConfig{Nameserver: StringPtr("192.168.1.1")}}}})

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrUnsupportedSettings), err.Error())
		assert.Contains(t, err.Error(), "network.eth.ip.nameserver (nameserver)")
		assert.Empty(t, dev.calls)
	})
	t.Run("a key reported as null is written", func(t *testing.T) {
		dev := newEthBleDevice("192.168.1.99", nil)

		_, err := applyTo(t, dev, &DeviceConfiguration{Network: &NetworkConfiguration{Ethernet: &EthernetConfig{StaticIP: &StaticIPConfig{Nameserver: StringPtr("192.168.1.1")}}}})

		require.NoError(t, err)
		require.Len(t, ethCalls(dev.fakeGen2Device), 1)
	})
	t.Run("keep_running on firmware that does not report it", func(t *testing.T) {
		dev := newEthBleDevice("192.168.1.99", nil)
		delete(dev.doc["ble"].(map[string]interface{})["rpc"].(map[string]interface{}), "keep_running")

		_, err := applyTo(t, dev, bleDesired(BLEConfig{RPC: &BLERPCConfig{KeepRunning: BoolPtr(true)}}))

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrUnsupportedSettings), err.Error())
		assert.Contains(t, err.Error(), "system.device.ble.rpc.keep_running (rpc.keep_running)")
		assert.Empty(t, dev.calls)
	})
	t.Run("a device that cannot be read", func(t *testing.T) {
		dev := newEthBleDevice("192.168.1.99", nil)
		dev.getErr = errors.New("timeout")

		_, err := applyTo(t, dev, ethDesired(BoolPtr(false), nil, nil, nil, nil))

		require.Error(t, err)
		assert.Empty(t, dev.calls)
	})
}

// Pin: a Gen1 apply keeps refusing Ethernet, a Gen2 device keeps refusing the Wi-Fi of the typed network section.
func TestApplyDesiredConfig_EthernetStaysRefusedWhereItIsNotWritten(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "x")
	setStoredDesired(repo, `{"network":{"eth":{"enable":false}}}`)

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, newFakeGen1Device(), "SHPLG-S")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings), err.Error())

	dev := newEthBleDevice("192.168.1.99", nil)
	_, err = applyTo(t, dev, &DeviceConfiguration{Network: &NetworkConfiguration{WiFi: &TypedWiFiConfig{STA: &WiFiSTAConfig{Enable: BoolPtr(true)}}}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings), err.Error())
	assert.Empty(t, dev.calls)
}

// One GetConfig serves every pre-write check, and the guard adds one Eth.GetStatus beside it.
func TestApplyDesiredConfig_Gen2EthernetReadsOneDocumentAndOneStatus(t *testing.T) {
	dev := newEthBleDevice("192.168.1.99", nil)
	desired := ethDesired(nil, nil, StringPtr("192.168.1.70"), nil, nil)
	desired.System = &SystemConfiguration{Device: &TypedDeviceConfig{BleConfig: &BLEConfig{RPC: &BLERPCConfig{Enable: BoolPtr(false)}}}}
	desired.Relay = &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, VoltageLimit: IntPtr(250)}}}
	desired.Input = &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("momentary")}}}

	_, err := applyTo(t, dev, desired)

	require.NoError(t, err)
	assert.Equal(t, 1, dev.getsBeforeFirstWrite, "one GetConfig before the first write")
	assert.Equal(t, 1, dev.statusCalls, "one Eth.GetStatus")
}

// A Wi-Fi setting beside Ethernet reads the device once too: the Wi-Fi check and the Ethernet guard
// share one document, so the station answer they compare is one snapshot.
func TestApplyDesiredConfig_Gen2EthernetWithAWiFiSettingReadsOneDocument(t *testing.T) {
	dev := newEthBleDevice("192.168.1.99", nil)
	dev.doc["wifi"] = map[string]interface{}{"sta": map[string]interface{}{"ssid": "home", "enable": true}}
	desired := ethDesired(nil, nil, StringPtr("192.168.1.70"), nil, nil)
	desired.WiFi = &WiFiConfiguration{Enable: BoolPtr(true)}

	_, err := applyTo(t, dev, desired)

	require.NoError(t, err)
	assert.Equal(t, 1, dev.getsBeforeFirstWrite, "one GetConfig before the first write, Wi-Fi included")
}

// No Ethernet key desired: no status read at all.
func TestApplyDesiredConfig_Gen2ReadsNoEthernetStatusWithoutAnEthernetChange(t *testing.T) {
	dev := newEthBleDevice("192.168.1.99", nil)

	_, err := applyTo(t, dev, bleDesired(BLEConfig{RPC: &BLERPCConfig{Enable: BoolPtr(false)}}))

	require.NoError(t, err)
	assert.Equal(t, 0, dev.statusCalls)
}
