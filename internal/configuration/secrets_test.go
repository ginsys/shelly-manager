package configuration

import (
	"encoding/json"
	"reflect"
	"testing"
)

func mustConfig(t *testing.T, body string) *DeviceConfiguration {
	t.Helper()
	var c DeviceConfiguration
	if err := json.Unmarshal([]byte(body), &c); err != nil {
		t.Fatalf("unmarshal %s: %v", body, err)
	}
	return &c
}

func configJSON(t *testing.T, c *DeviceConfiguration) string {
	t.Helper()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestKeepStoredSecrets(t *testing.T) {
	stored := `{"wifi":{"ssid":"home","password":"w1","ap":{"ssid":"shelly","pass":"a1"}},` +
		`"mqtt":{"user":"iot","pass":"m1"},"auth":{"enable":true,"user":"admin","pass":"p1"}}`
	cases := []struct {
		name, next, want string
	}{
		{
			name: "omitted secrets are kept",
			next: `{"wifi":{"ssid":"home","ap":{"ssid":"shelly"}},"mqtt":{"user":"iot2"},"auth":{"enable":true,"user":"admin"}}`,
			want: `{"wifi":{"ssid":"home","password":"w1","ap":{"ssid":"shelly","pass":"a1"}},` +
				`"mqtt":{"user":"iot2","pass":"m1"},"auth":{"enable":true,"user":"admin","pass":"p1"}}`,
		},
		{
			name: "an omitted section keeps its secret",
			next: `{"led":{"power_indication":true}}`,
			want: `{"wifi":{"password":"w1","ap":{"pass":"a1"}},"mqtt":{"pass":"m1"},"auth":{"pass":"p1"},` +
				`"led":{"power_indication":true}}`,
		},
		{
			name: "an empty string clears the secret",
			next: `{"wifi":{"password":"","ap":{"pass":""}},"mqtt":{"pass":""},"auth":{"pass":""}}`,
			want: `{"wifi":{"ap":{}},"mqtt":{},"auth":{}}`,
		},
		{
			name: "a new secret replaces the stored one",
			next: `{"mqtt":{"pass":"m2"}}`,
			want: `{"wifi":{"password":"w1","ap":{"pass":"a1"}},"mqtt":{"pass":"m2"},"auth":{"pass":"p1"}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next := mustConfig(t, tc.next)
			KeepStoredSecrets(next, mustConfig(t, stored))
			if got := configJSON(t, next); got != configJSON(t, mustConfig(t, tc.want)) {
				t.Errorf("got %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestKeepStoredSecretsNothingStored(t *testing.T) {
	next := mustConfig(t, `{"mqtt":{"user":"iot","pass":""}}`)
	KeepStoredSecrets(next, nil)
	if got := configJSON(t, next); got != `{"mqtt":{"user":"iot"}}` {
		t.Errorf("got %s", got)
	}
	KeepStoredSecrets(nil, mustConfig(t, `{"mqtt":{"pass":"m1"}}`))
}

func TestKeepStoredSecretsDoesNotAliasStored(t *testing.T) {
	stored := mustConfig(t, `{"mqtt":{"pass":"m1"}}`)
	next := mustConfig(t, `{}`)
	KeepStoredSecrets(next, stored)
	*next.MQTT.Password = "changed"
	if *stored.MQTT.Password != "m1" {
		t.Errorf("stored secret changed through next: %q", *stored.MQTT.Password)
	}
}

func TestStoredSecretPaths(t *testing.T) {
	c := mustConfig(t, `{"wifi":{"password":"w1","ap":{"pass":""}},"mqtt":{"pass":"m1"},"auth":{"user":"admin"}}`)
	if got, want := StoredSecretPaths(c), []string{"wifi.password", "mqtt.pass"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := StoredSecretPaths(nil); len(got) != 0 || got == nil {
		t.Errorf("nil config: got %#v, want an empty list", got)
	}
}
