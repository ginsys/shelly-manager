package service

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestKeepStoredLogin(t *testing.T) {
	const (
		loginA   = `"auth_enabled":true,"auth_user":"old","auth_pass":"oldpw"`
		loginB   = `"auth_enabled":true,"auth_user":"bee","auth_pass":"beepw"`
		noLogin  = `"auth_enabled":false`
		nested   = `"auth":{"username":"bee","password":"beepw"}`
		unparsed = `{not json`
	)
	tests := []struct {
		name                    string
		incoming, basis, stored string
		want                    string // a settings document, or "" when the incoming text stays
	}{
		{"request keeps login A, B is stored now", `{` + loginA + `,"note":"new"}`, `{` + loginA + `,"note":"old"}`, `{` + loginB + `,"note":"old"}`,
			`{` + loginB + `,"note":"new"}`},
		{"request changes the login on purpose", `{"auth_enabled":true,"auth_user":"carol","auth_pass":"cpw"}`, `{` + loginA + `}`, `{` + loginB + `}`, ""},
		{"login was cleared meanwhile", `{` + loginA + `,"note":"new"}`, `{` + loginA + `}`, `{` + noLogin + `}`,
			`{` + noLogin + `,"note":"new"}`},
		{"login was added meanwhile", `{` + noLogin + `,"note":"new"}`, `{` + noLogin + `}`, `{` + loginB + `,` + nested + `}`,
			`{` + loginB + `,` + nested + `,"note":"new"}`},
		{"request adds the auth_enabled default of the validation", `{"auth":{"username":"old","password":"oldpw"},"auth_enabled":false}`,
			`{"auth":{"username":"old","password":"oldpw"}}`, `{` + loginB + `,` + nested + `}`,
			`{` + loginB + `,` + nested + `}`},
		{"stored settings do not parse", `{` + loginA + `}`, `{` + loginA + `}`, unparsed, ""},
		{"basis does not parse", `{` + loginA + `}`, unparsed, `{` + loginB + `}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := keepStoredLogin(tt.incoming, tt.basis, tt.stored)
			if err != nil {
				t.Fatal(err)
			}
			want := tt.want
			if want == "" {
				want = tt.incoming
			}
			var gotDoc, wantDoc map[string]any
			if err := json.Unmarshal([]byte(got), &gotDoc); err != nil {
				t.Fatalf("result is not JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(want), &wantDoc); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gotDoc, wantDoc) {
				t.Errorf("got %s, want %s", got, want)
			}
		})
	}
}

func TestKeepStoredLogin_IncomingThatDoesNotParse(t *testing.T) {
	if _, err := keepStoredLogin(`{not json`, `{}`, `{}`); err == nil {
		t.Error("an incoming text that does not parse must be an error")
	}
}
