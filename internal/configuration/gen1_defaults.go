package configuration

// gen1FieldDefaults holds the one default the device schema shows for each field that apply
// writes to a Gen1 device (a path of gen1ApplicablePaths). A default is listed here only where a
// source states it: the Shelly Gen1 API reference (https://shelly-api-docs.shelly.cloud/gen1/) or,
// for the MQTT port, the MQTT standard. Any other applied field shows none, and
// TestGen1AppliedFieldsHaveADefaultOrAreNamed lists them, so a default that no source gives is never
// invented. markApplicable reads this table. A default is a form hint only: apply sends what the
// configuration holds, and a port left empty is not sent (the converter writes the bare host).
var gen1FieldDefaults = map[string]interface{}{
	// OASIS MQTT 5.0 section 4.2, which registers TCP port 1883 with IANA for MQTT (paraphrase).
	// The Gen1 reference gives no default port.
	"mqtt.port": 1883,
	// Reference, "HTTP server": "HTTP authentication is disabled by default."
	"auth.enable": false,
	// Reference, "Availability and announces": "Device state is reported periodically, every 30
	// seconds by default."
	"mqtt.update_period": 30,
	// Reference, "SNTP time sync": "The default NTP server is time.google.com".
	"system.sntp.server": "time.google.com",
}
