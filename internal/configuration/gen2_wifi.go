package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
)

// The Gen2 Wi-Fi settings. WiFi.SetConfig takes {"config": {"sta": {...}}}, and the same call takes
// "ap" and "roam". The station, the access point and the roam threshold are written; the fallback
// network (sta1), the access point key field and a station that is switched off are refused before
// the device is touched: the manager reaches the device through the station.

// gen2WiFiPasswordUsable reports whether a password may be sent: an empty one and a mask is never one.
func gen2WiFiPasswordUsable(s string) bool {
	return usableWiFiSecret(s) && s != redactedSecret
}

// gen2WiFiToDoc returns the wifi object of the keys the configuration sets: the station (sta), the
// access point (ap) and the roam threshold (roam). A part with no key is left out.
func gen2WiFiToDoc(w *WiFiConfiguration) map[string]interface{} {
	doc := map[string]interface{}{}
	if sta := gen2StationToDoc(w); len(sta) > 0 {
		doc["sta"] = sta
	}
	if ap := gen2AccessPointToDoc(w.AccessPoint); len(ap) > 0 {
		doc["ap"] = ap
	}
	if w.RoamThreshold != nil {
		doc["roam"] = map[string]interface{}{"rssi_thr": *w.RoamThreshold}
	}
	return doc
}

// gen2AccessPointToDoc returns the keys of the access point that the configuration sets. The
// password is write-only and never sent empty or masked. The key field has no Gen2 meaning.
func gen2AccessPointToDoc(ap *AccessPointConfig) map[string]interface{} {
	doc := map[string]interface{}{}
	if ap == nil {
		return doc
	}
	if ap.Enable != nil {
		doc["enable"] = *ap.Enable
	}
	if ap.SSID != nil {
		doc["ssid"] = *ap.SSID
	}
	if ap.Password != nil && gen2WiFiPasswordUsable(*ap.Password) {
		doc["pass"] = *ap.Password
	}
	return doc
}

// gen2StationToDoc returns the keys of the station that the configuration sets. The password is
// write-only, and the device wants it with every ssid it is given.
func gen2StationToDoc(w *WiFiConfiguration) map[string]interface{} {
	sta := map[string]interface{}{}
	if w.Enable != nil {
		sta["enable"] = *w.Enable
	}
	if w.SSID != nil {
		sta["ssid"] = *w.SSID
	}
	if w.Password != nil && gen2WiFiPasswordUsable(*w.Password) {
		sta["pass"] = *w.Password
	}
	if w.IPv4Mode != nil {
		sta["ipv4mode"] = *w.IPv4Mode
	}
	if s := w.StaticIP; s != nil {
		for key, value := range map[string]*string{"ip": s.IP, "netmask": s.Netmask, "gw": s.Gateway, "nameserver": s.Nameserver} {
			if value != nil {
				sta[key] = *value
			}
		}
	}
	return sta
}

// gen2WiFiFromDoc reads the station, the access point and the roam threshold of a Shelly.GetConfig
// document. A null value is an unset setting, and the device returns no password.
func gen2WiFiFromDoc(section interface{}) *WiFiConfiguration {
	wifi, _ := section.(map[string]interface{})
	w := gen2StationFromDoc(wifi["sta"])
	if w == nil {
		w = &WiFiConfiguration{}
	}
	if raw, ok := wifi["ap"].(map[string]interface{}); ok {
		ap := &AccessPointConfig{}
		if enable, ok := raw["enable"].(bool); ok {
			ap.Enable = BoolPtr(enable)
		}
		if ssid, ok := raw["ssid"].(string); ok && ssid != "" {
			ap.SSID = StringPtr(ssid)
		}
		if *ap != (AccessPointConfig{}) {
			w.AccessPoint = ap
		}
	}
	if roam, ok := wifi["roam"].(map[string]interface{}); ok {
		if threshold, ok := roam["rssi_thr"].(float64); ok && threshold == math.Trunc(threshold) {
			w.RoamThreshold = IntPtr(int(threshold))
		}
	}
	if *w == (WiFiConfiguration{}) {
		return nil
	}
	return w
}

// gen2StationFromDoc reads the wifi.sta object, or returns nil.
func gen2StationFromDoc(section interface{}) *WiFiConfiguration {
	raw, ok := section.(map[string]interface{})
	if !ok {
		return nil
	}
	w := &WiFiConfiguration{}
	if enable, ok := raw["enable"].(bool); ok {
		w.Enable = BoolPtr(enable)
	}
	if ssid, ok := raw["ssid"].(string); ok && ssid != "" {
		w.SSID = StringPtr(ssid)
	}
	if mode, ok := raw["ipv4mode"].(string); ok && mode != "" {
		w.IPv4Mode = StringPtr(mode)
	}
	static := &StaticIPConfig{}
	for key, field := range map[string]**string{"ip": &static.IP, "netmask": &static.Netmask, "gw": &static.Gateway, "nameserver": &static.Nameserver} {
		if value, ok := raw[key].(string); ok && value != "" {
			*field = StringPtr(value)
		}
	}
	if *static != (StaticIPConfig{}) {
		w.StaticIP = static
	}
	return w
}

// unmappedWiFiSettings returns the paths of the Wi-Fi settings that the Gen2 apply does not write,
// and the rest of the section for the round trip (without the write-only password), or nil when
// nothing else is set.
func unmappedWiFiSettings(w *WiFiConfiguration) ([]string, *WiFiConfiguration) {
	if w == nil {
		return nil, nil
	}
	rest := *w
	var paths []string
	// The password is write-only: no read returns it, so the round trip cannot compare it, and a
	// section that holds only a password is a station write like any other.
	rest.Password = nil
	if rest.Enable != nil && !*rest.Enable {
		paths, rest.Enable = append(paths, "wifi.enable"), nil
	}
	// The roaming flag and the recovery reboot are Gen1 settings: Gen2 writes the threshold only.
	if rest.RoamEnable != nil {
		paths, rest.RoamEnable = append(paths, "wifi.roam_enable"), nil
	}
	if rest.RecoveryReboot != nil {
		paths, rest.RecoveryReboot = append(paths, "wifi.recovery_reboot"), nil
	}
	if ap := rest.AccessPoint; ap != nil {
		// The access point has one secret, pass, and it is write-only too. The key field has no
		// Gen2 meaning and is refused by name.
		if ap.Key != nil {
			paths = append(paths, "wifi.ap.key")
		}
		kept := *ap
		kept.Key, kept.Password = nil, nil
		rest.AccessPoint = &kept
		if kept == (AccessPointConfig{}) {
			rest.AccessPoint = nil
		}
	}
	if rest == (WiFiConfiguration{}) {
		return paths, nil
	}
	return paths, &rest
}

// gen2WiFiStation returns the station keys of a converted document, or nil.
func gen2WiFiStation(apiConfig map[string]interface{}) map[string]interface{} {
	wifi, _ := apiConfig["wifi"].(map[string]interface{})
	sta, _ := wifi["sta"].(map[string]interface{})
	return sta
}

// gen2StationNetworkKeys are the station keys whose change can move the device to another network
// or address. The password does not, and neither does the name server (Gen1 skips both too).
var gen2StationNetworkKeys = []string{"enable", "ssid", "ipv4mode", "ip", "netmask", "gw"}

// gen2StationNetworkChanged reports whether the station write moves the device's network: a
// network key it sets differs from what the device reported before the write. Only then can a
// device that stops answering have moved.
func gen2StationNetworkChanged(sta, live map[string]interface{}) bool {
	for _, key := range gen2StationNetworkKeys {
		if value, written := sta[key]; written && !relayValuesEqual(value, live[key]) {
			return true
		}
	}
	return false
}

// gen2WiFiPart returns the keys of one part (ap, roam) of a converted document, or nil.
func gen2WiFiPart(apiConfig map[string]interface{}, part string) map[string]interface{} {
	wifi, _ := apiConfig["wifi"].(map[string]interface{})
	keys, _ := wifi[part].(map[string]interface{})
	return keys
}

// refuseStrandingAccessPoint stops an apply that switches the access point off while the station
// is not certain to stay: the device would be left with no way back. The station is steady when
// the device reports it enabled with a network and the apply does not touch it: neither its network
// nor its password, because a wrong password leaves the device unable to join. An access point
// that is already off is not switched off by this apply.
func refuseStrandingAccessPoint(ap, liveAP, liveStation map[string]interface{}, stationTouched bool) error {
	if enable, set := ap["enable"].(bool); !set || enable {
		return nil
	}
	if off, known := liveAP["enable"].(bool); known && !off {
		return nil
	}
	ssid, _ := liveStation["ssid"].(string)
	if on, _ := liveStation["enable"].(bool); on && ssid != "" && !stationTouched {
		return nil
	}
	return &refusedBeforeWriteError{fmt.Errorf("%w: wifi.ap.enable: switching the access point off needs a steady station: the device's station must be enabled with a network, and this apply must not change its network or its password", ErrUnsupportedSettings)}
}

// refuseOpenAccessPoint stops an apply that switches an access point on without a password: a
// Shelly access point is open until it has one. An access point that already runs is not touched,
// and one the device reports as closed has its password already.
func refuseOpenAccessPoint(ap, liveAP map[string]interface{}) error {
	if enable, set := ap["enable"].(bool); !set || !enable {
		return nil
	}
	if running, _ := liveAP["enable"].(bool); running {
		return nil
	}
	if _, hasPassword := ap["pass"]; hasPassword {
		return nil
	}
	if open, known := liveAP["is_open"].(bool); known && !open {
		return nil
	}
	return &refusedBeforeWriteError{fmt.Errorf("%w: wifi.ap.enable: switching on an open access point is refused; set wifi.ap.pass", ErrUnsupportedSettings)}
}

// prepareWiFiStation reads the device before the first write when the station sets a network key
// or the access point is switched on or off, and stops an apply that would leave the device
// unreachable or its access point open. The settings are never written blind, so a device that
// cannot be read is refused. A new network needs its password: the device keeps none it was not
// sent. The network the device is already on needs none, and its ssid is left out of the write,
// because the device wants a password with every ssid it is given. It reports whether the write
// changes the station network. A write of the password or the name server alone reads nothing and
// changes no network.
func prepareWiFiStation(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}) (bool, error) {
	sta := gen2WiFiStation(apiConfig)
	ap := gen2WiFiPart(apiConfig, "ap")
	_, apSwitched := ap["enable"].(bool)
	sets := apSwitched
	for _, key := range gen2StationNetworkKeys {
		if _, ok := sta[key]; ok {
			sets = true
		}
	}
	if !sets {
		return false, nil
	}
	live, err := client.GetConfig(ctx)
	if err != nil {
		return false, &refusedBeforeWriteError{fmt.Errorf("could not read the device before writing the Wi-Fi settings: %w", err)}
	}
	var doc map[string]interface{}
	if err = json.Unmarshal(live.Raw, &doc); err != nil {
		return false, &refusedBeforeWriteError{fmt.Errorf("could not read the device before writing the Wi-Fi settings: %w", err)}
	}
	liveStation := gen2WiFiStation(doc)
	changed := gen2StationNetworkChanged(sta, liveStation)
	liveAP := gen2WiFiPart(doc, "ap")
	_, sendsPassword := sta["pass"]
	if err = refuseStrandingAccessPoint(ap, liveAP, liveStation, changed || sendsPassword); err != nil {
		return false, err
	}
	if err = refuseOpenAccessPoint(ap, liveAP); err != nil {
		return false, err
	}

	ssid, ok := sta["ssid"].(string)
	if !ok {
		return changed, nil
	}
	if _, hasPassword := sta["pass"]; hasPassword {
		return changed, nil
	}
	if current, _ := liveStation["ssid"].(string); current != ssid {
		return false, &refusedBeforeWriteError{fmt.Errorf("%w: wifi.ssid: a new network needs its password, and none that can be sent is set", ErrUnsupportedSettings)}
	}
	delete(sta, "ssid")
	return changed, nil
}
