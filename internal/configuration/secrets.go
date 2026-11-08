package configuration

// The API never returns a stored secret. A client that saves the body it read therefore sends it
// without the secret. KeepStoredSecrets and StoredSecretPaths let a save keep what the client
// could not see.

// KeepStoredSecrets copies each secret of stored into next where next leaves it out, and
// creates the section in next if needed. An explicit empty string in next clears the secret.
// The secrets are wifi.password, wifi.ap.pass, mqtt.pass and auth.pass.
func KeepStoredSecrets(next, stored *DeviceConfiguration) {
	if next == nil {
		return
	}
	if stored == nil {
		stored = &DeviceConfiguration{}
	}
	var wifiPass, apPass, mqttPass, authPass *string
	if stored.WiFi != nil {
		wifiPass = stored.WiFi.Password
		if stored.WiFi.AccessPoint != nil {
			apPass = stored.WiFi.AccessPoint.Password
		}
	}
	if stored.MQTT != nil {
		mqttPass = stored.MQTT.Password
	}
	if stored.Auth != nil {
		authPass = stored.Auth.Password
	}

	if (secretSet(wifiPass) || secretSet(apPass)) && next.WiFi == nil {
		next.WiFi = &WiFiConfiguration{}
	}
	if secretSet(apPass) && next.WiFi.AccessPoint == nil {
		next.WiFi.AccessPoint = &AccessPointConfig{}
	}
	if secretSet(mqttPass) && next.MQTT == nil {
		next.MQTT = &MQTTConfiguration{}
	}
	if secretSet(authPass) && next.Auth == nil {
		next.Auth = &AuthConfiguration{}
	}

	if next.WiFi != nil {
		keepSecret(&next.WiFi.Password, wifiPass)
		if next.WiFi.AccessPoint != nil {
			keepSecret(&next.WiFi.AccessPoint.Password, apPass)
		}
	}
	if next.MQTT != nil {
		keepSecret(&next.MQTT.Password, mqttPass)
	}
	if next.Auth != nil {
		keepSecret(&next.Auth.Password, authPass)
	}
}

// StoredSecretPaths lists the paths of the secrets that config holds, in the form the
// configuration schema uses, for example "mqtt.pass".
func StoredSecretPaths(config *DeviceConfiguration) []string {
	paths := []string{}
	if config == nil {
		return paths
	}
	if config.WiFi != nil {
		if secretSet(config.WiFi.Password) {
			paths = append(paths, "wifi.password")
		}
		if config.WiFi.AccessPoint != nil && secretSet(config.WiFi.AccessPoint.Password) {
			paths = append(paths, "wifi.ap.pass")
		}
	}
	if config.MQTT != nil && secretSet(config.MQTT.Password) {
		paths = append(paths, "mqtt.pass")
	}
	if config.Auth != nil && secretSet(config.Auth.Password) {
		paths = append(paths, "auth.pass")
	}
	return paths
}

// keepSecret sets *dst to a copy of stored when *dst is nil, and to nil when *dst is empty.
func keepSecret(dst **string, stored *string) {
	switch {
	case *dst == nil:
		if secretSet(stored) {
			v := *stored
			*dst = &v
		}
	case **dst == "":
		*dst = nil
	}
}

func secretSet(s *string) bool {
	return s != nil && *s != ""
}
