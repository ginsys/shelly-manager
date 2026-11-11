package sync

import "testing"

func exportDataWithSecrets() *ExportData {
	return &ExportData{
		Devices:        []DeviceData{{Settings: map[string]interface{}{"auth_pass": "d1", "model": "SHSW-1"}}},
		Configurations: []ConfigurationData{{Config: map[string]interface{}{"mqtt": map[string]interface{}{"pass": "c1", "server": "mqtt.example.net"}}}},
		Templates: []TemplateData{{
			Config:    map[string]interface{}{"wifi": map[string]interface{}{"password": "t1"}},
			Variables: map[string]interface{}{"api_token": "v1", "room": "kitchen"},
		}},
	}
}

func TestMaskExportSecrets_MasksEveryRecordKind(t *testing.T) {
	for name, config := range map[string]map[string]interface{}{
		"no config":       nil,
		"true":            {"exclude_sensitive": true},
		"mistyped string": {"exclude_sensitive": "false"},
	} {
		data := exportDataWithSecrets()
		maskExportSecrets(data, config)

		if got := data.Devices[0].Settings["auth_pass"]; got != RedactedValue {
			t.Errorf("%s: device setting = %v, want the marker", name, got)
		}
		mqtt := data.Configurations[0].Config["mqtt"].(map[string]interface{})
		if mqtt["pass"] != RedactedValue || mqtt["server"] != "mqtt.example.net" {
			t.Errorf("%s: device configuration = %v", name, mqtt)
		}
		if got := data.Templates[0].Config["wifi"].(map[string]interface{})["password"]; got != RedactedValue {
			t.Errorf("%s: template secret = %v, want the marker", name, got)
		}
		if data.Templates[0].Variables["api_token"] != RedactedValue || data.Templates[0].Variables["room"] != "kitchen" {
			t.Errorf("%s: template variables = %v", name, data.Templates[0].Variables)
		}
	}
}

func TestMaskExportSecrets_FalseKeepsEverything(t *testing.T) {
	data := exportDataWithSecrets()
	maskExportSecrets(data, map[string]interface{}{"exclude_sensitive": false})
	if data.Devices[0].Settings["auth_pass"] != "d1" || data.Templates[0].Variables["api_token"] != "v1" {
		t.Error("a full export must keep the saved secrets")
	}
}
