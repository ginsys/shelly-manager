package configuration

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// The Gen2 Ethernet and Bluetooth components of a Pro model, written with Eth.SetConfig and
// BLE.SetConfig. The key names and values are read from the Shelly Eth and BLE references and are
// unconfirmed on hardware, as is how the device behaves when an Ethernet change meets a link that is
// the manager's own path.

const (
	gen2EthKey = "eth"
	gen2BLEKey = "ble"
)

// Gen2EthStatusClient is what the Ethernet strand guard needs beyond Gen2ConfigClient: the Eth.GetStatus
// answer, whose ip is the address the device holds on Ethernet, and the WiFi.GetStatus answer, whose
// sta_ip is its station address. *gen2.Client implements both. A client that does not is "cannot tell"
// to the guard.
type Gen2EthStatusClient interface {
	GetEthStatus(ctx context.Context) (map[string]interface{}, error)
	GetWiFiStatus(ctx context.Context) (map[string]interface{}, error)
}

// gen2EthKeys maps each key of the eth section to the desired path that sets it, in the order they are
// named. The first five can move the device off the manager's address; the name server cannot.
var gen2EthKeys = []struct{ key, path string }{
	{"enable", "network.eth.enable"}, {"ipv4mode", "network.eth.ipv4mode"}, {"ip", "network.eth.ip.ip"},
	{"netmask", "network.eth.ip.netmask"}, {"gw", "network.eth.ip.gw"}, {"nameserver", "network.eth.ip.nameserver"},
}

// gen2EthToDoc returns the eth keys the configuration sets.
func gen2EthToDoc(e *EthernetConfig) map[string]interface{} {
	doc := map[string]interface{}{}
	if e == nil {
		return doc
	}
	if e.Enable != nil {
		doc["enable"] = *e.Enable
	}
	if e.IPv4Mode != nil {
		doc["ipv4mode"] = *e.IPv4Mode
	}
	if s := e.StaticIP; s != nil {
		for key, value := range map[string]*string{"ip": s.IP, "netmask": s.Netmask, "gw": s.Gateway, "nameserver": s.Nameserver} {
			if value != nil {
				doc[key] = *value
			}
		}
	}
	return doc
}

// gen2EthFromDoc reads the eth section, or returns nil. A null or empty address is an unset one.
func gen2EthFromDoc(section interface{}) *EthernetConfig {
	raw, ok := section.(map[string]interface{})
	if !ok {
		return nil
	}
	e := &EthernetConfig{}
	if enable, ok := raw["enable"].(bool); ok {
		e.Enable = BoolPtr(enable)
	}
	if mode, ok := raw["ipv4mode"].(string); ok && mode != "" {
		e.IPv4Mode = StringPtr(mode)
	}
	static := &StaticIPConfig{}
	for key, field := range map[string]**string{"ip": &static.IP, "netmask": &static.Netmask, "gw": &static.Gateway, "nameserver": &static.Nameserver} {
		if value, ok := raw[key].(string); ok && value != "" {
			*field = StringPtr(value)
		}
	}
	if *static != (StaticIPConfig{}) {
		e.StaticIP = static
	}
	if *e == (EthernetConfig{}) {
		return nil
	}
	return e
}

// gen2BLEToDoc returns the ble object of the rpc keys the configuration sets. The observer is
// obsolete and the top-level enable was removed in firmware 2.0.0: neither is ever written.
func gen2BLEToDoc(b *BLEConfig) map[string]interface{} {
	rpc := map[string]interface{}{}
	if b != nil && b.RPC != nil {
		if b.RPC.Enable != nil {
			rpc["enable"] = *b.RPC.Enable
		}
		if b.RPC.KeepRunning != nil {
			rpc["keep_running"] = *b.RPC.KeepRunning
		}
	}
	if len(rpc) == 0 {
		return nil
	}
	return map[string]interface{}{"rpc": rpc}
}

// gen2BLEFromDoc reads ble.rpc, or returns nil.
func gen2BLEFromDoc(section interface{}) *BLEConfig {
	raw, _ := section.(map[string]interface{})
	rpc, ok := raw["rpc"].(map[string]interface{})
	if !ok {
		return nil
	}
	r := &BLERPCConfig{}
	if enable, ok := rpc["enable"].(bool); ok {
		r.Enable = BoolPtr(enable)
	}
	if keep, ok := rpc["keep_running"].(bool); ok {
		r.KeepRunning = BoolPtr(keep)
	}
	if *r == (BLERPCConfig{}) {
		return nil
	}
	return &BLEConfig{RPC: r}
}

// parseIPv4 returns the IPv4 address of a literal, or nil for a name, an IPv6 address or anything else.
// An IPv4-mapped IPv6 literal is that IPv4 address.
func parseIPv4(s string) net.IP {
	if ip := net.ParseIP(s); ip != nil {
		return ip.To4()
	}
	return nil
}

// unmappedEthSettings returns the paths of an Ethernet setting that no device takes: a mode that is not
// dhcp or static, and an address that is not an IPv4 literal (the page names no other form).
func unmappedEthSettings(n *NetworkConfiguration) []string {
	if n == nil || n.Ethernet == nil {
		return nil
	}
	e := n.Ethernet
	var paths []string
	if e.IPv4Mode != nil && *e.IPv4Mode != "dhcp" && *e.IPv4Mode != "static" {
		paths = append(paths, "network.eth.ipv4mode")
	}
	if s := e.StaticIP; s != nil {
		for _, a := range []struct {
			value *string
			path  string
		}{{s.IP, "network.eth.ip.ip"}, {s.Netmask, "network.eth.ip.netmask"}, {s.Gateway, "network.eth.ip.gw"}, {s.Nameserver, "network.eth.ip.nameserver"}} {
			if a.value != nil && (strings.Contains(*a.value, ":") || parseIPv4(*a.value) == nil) {
				paths = append(paths, a.path)
			}
		}
	}
	return paths
}

// unmappedBLESettings returns the paths of a BLE setting that BLE.SetConfig has no key for. The round
// trip names the section too.
func unmappedBLESettings(s *SystemConfiguration) []string {
	if s == nil || s.Device == nil || s.Device.BleConfig == nil {
		return nil
	}
	ble := s.Device.BleConfig
	var paths []string
	if ble.Enable {
		paths = append(paths, "system.device.ble.enable")
	}
	if ble.RPC != nil && ble.RPC.DstAddr != "" {
		paths = append(paths, "system.device.ble.rpc.dst_addr")
	}
	if ble.RPC != nil && ble.RPC.ListenPort != 0 {
		paths = append(paths, "system.device.ble.rpc.listen_port")
	}
	return paths
}

// unreportedEthBLEKeys names, before any write, what the live document cannot take: a component it
// does not report, and a desired key it does not list (a key present with null is reported). Each
// entry is the desired path with the device key beside it.
func unreportedEthBLEKeys(doc, apiConfig map[string]interface{}) []string {
	var missing []string
	if body, ok := apiConfig[gen2EthKey].(map[string]interface{}); ok && len(body) > 0 {
		live, held := doc[gen2EthKey].(map[string]interface{})
		if !held {
			missing = append(missing, "network.eth (the device reports no "+gen2EthKey+" component)")
		}
		for _, k := range gen2EthKeys {
			if _, wanted := body[k.key]; !wanted || !held {
				continue
			}
			if _, ok := live[k.key]; !ok {
				missing = append(missing, k.path+" ("+k.key+")")
			}
		}
	}
	if body, ok := apiConfig[gen2BLEKey].(map[string]interface{}); ok && len(body) > 0 {
		rpc, _ := body["rpc"].(map[string]interface{})
		live, held := doc[gen2BLEKey].(map[string]interface{})
		if !held {
			missing = append(missing, "system.device.ble (the device reports no "+gen2BLEKey+" component)")
		}
		liveRPC, _ := live["rpc"].(map[string]interface{})
		for _, k := range []string{"enable", "keep_running"} {
			if _, wanted := rpc[k]; !wanted || !held {
				continue
			}
			if _, ok := liveRPC[k]; !ok {
				missing = append(missing, "system.device.ble.rpc."+k+" (rpc."+k+")")
			}
		}
	}
	return missing
}

// refuseEthernet stops, before any write, an Ethernet write that the live device cannot take or that
// could cut the manager off. It reports whether the write changes a value the device holds, because
// the page says every Ethernet change needs a restart. The device is read for its status only when a
// key that can move it changes. When the guard cannot tell how the manager reaches the device, it refuses.
func refuseEthernet(ctx context.Context, client ShellyClient, doc, apiConfig map[string]interface{}, stationMoves bool) (changed bool, err error) {
	body, ok := apiConfig[gen2EthKey].(map[string]interface{})
	if !ok || len(body) == 0 {
		return false, nil
	}
	live, _ := doc[gen2EthKey].(map[string]interface{})
	refuse := func(paths []string, why string) error {
		return &refusedBeforeWriteError{fmt.Errorf("%w: %s: %s", ErrUnsupportedSettings, strings.Join(paths, ", "), why)}
	}
	if serving, _ := live["server_mode"].(bool); serving {
		return false, refuse([]string{"network.eth"}, "the device runs its Ethernet in server mode (eth.server_mode), which the stored settings do not model")
	}

	// The effective mode, address and gateway: a static mode needs both, desired or already held.
	has := func(key string) bool {
		if _, set := body[key]; set {
			return true
		}
		value, _ := live[key].(string)
		return value != ""
	}
	mode, _ := body["ipv4mode"].(string)
	if _, set := body["ipv4mode"]; !set {
		mode, _ = live["ipv4mode"].(string)
	}
	if mode == "static" {
		var paths []string
		if !has("ip") {
			paths = append(paths, "network.eth.ip.ip")
		}
		if !has("gw") {
			paths = append(paths, "network.eth.ip.gw")
		}
		if len(paths) > 0 {
			return false, refuse(paths, "a static Ethernet address needs an address and a gateway, desired or already set on the device")
		}
	}

	var moves []string
	for _, k := range gen2EthKeys {
		value, written := body[k.key]
		if !written || relayValuesEqual(value, live[k.key]) {
			continue
		}
		changed = true
		if k.key != "nameserver" {
			moves = append(moves, k.path)
		}
	}
	if len(moves) == 0 {
		return changed, nil
	}
	if stationMoves {
		return false, refuse(moves, "an Ethernet change cannot be applied together with a change of the Wi-Fi network or a station password; apply them one at a time")
	}
	cannotTell := func(why string) error {
		return refuse(moves, "the manager cannot tell whether it reaches the device over Ethernet ("+why+")")
	}
	status, ok := client.(Gen2EthStatusClient)
	if !ok {
		return false, cannotTell("the client has no Eth.GetStatus")
	}
	answer, err := status.GetEthStatus(ctx)
	if err != nil {
		return false, cannotTell("Eth.GetStatus failed")
	}
	managerIP := parseIPv4(client.GetIP())
	raw, reported := answer["ip"]
	if managerIP == nil || !reported {
		return false, cannotTell("its own address or the status ip is missing or not an IPv4 address")
	}
	if raw == nil {
		return changed, nil // no address on Ethernet: the manager's connection is not on it
	}
	text, _ := raw.(string)
	statusIP := parseIPv4(text)
	if statusIP == nil {
		return false, cannotTell("the status ip is not an IPv4 address")
	}
	if statusIP.Equal(managerIP) {
		return false, refuse(moves, "the manager reaches the device on its Ethernet address, and this change could cut that path")
	}
	// An address that differs from the Ethernet one proves nothing: a forwarded or translated address
	// reads the same way. Only the Wi-Fi station address proves the manager is not on Ethernet.
	wifi, err := status.GetWiFiStatus(ctx)
	if err != nil {
		return false, cannotTell("WiFi.GetStatus failed")
	}
	station, _ := wifi["sta_ip"].(string)
	if staIP := parseIPv4(station); staIP == nil || !staIP.Equal(managerIP) {
		return false, cannotTell("its address " + client.GetIP() + " is neither the Ethernet address " + text + " nor the Wi-Fi address " + station + " (empty: none)")
	}
	return changed, nil
}
