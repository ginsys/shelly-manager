package configuration

import "testing"

func TestLegacyDeviceAuth(t *testing.T) {
	cases := []struct {
		name, settings, user, pass string
		ok                         bool
	}{
		{"legacy pair", `{"gen":1,"auth":{"username":"admin","password":"pw"}}`, "admin", "pw", true},
		{"legacy pair beside empty flat keys", `{"auth_user":"","auth_pass":"","auth":{"username":"admin","password":"pw"}}`, "admin", "pw", true},
		{"flat keys win", `{"auth_user":"root","auth_pass":"x","auth":{"username":"admin","password":"pw"}}`, "", "", false},
		{"discovery boolean", `{"auth":false}`, "", "", false},
		{"incomplete legacy pair", `{"auth":{"username":"admin"}}`, "", "", false},
		{"no auth", `{"gen":2}`, "", "", false},
		{"empty", ``, "", "", false},
		{"invalid json", `{`, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user, pass, ok := LegacyDeviceAuth(tc.settings)
			if user != tc.user || pass != tc.pass || ok != tc.ok {
				t.Errorf("got (%q, %q, %v), want (%q, %q, %v)", user, pass, ok, tc.user, tc.pass, tc.ok)
			}
		})
	}
}
