package gen1

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

func boolPtr(b bool) *bool { return &b }

// actionsServer records the last request and answers every call with body.
func actionsServer(t *testing.T, status int, body string) (*Client, *http.Request, *url.Values) {
	t.Helper()
	var got http.Request
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = *r.Clone(context.Background())
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				t.Errorf("ParseForm: %v", err)
			}
			form = r.PostForm
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return NewClient(strings.TrimPrefix(server.URL, "http://")), &got, &form
}

func TestGen1Client_SetActionFormSendsOneUrlsParameterPerURL(t *testing.T) {
	client, got, form := actionsServer(t, http.StatusOK, `{}`)

	err := client.SetActionForm(context.Background(), 1, "out_on_url", boolPtr(true),
		[]string{"http://192.168.1.4/on", "http://192.168.1.5/on"})
	if err != nil {
		t.Fatalf("SetActionForm: %v", err)
	}
	if got.Method != http.MethodPost || got.URL.Path != "/settings/actions" {
		t.Errorf("write went to %s %s, want POST /settings/actions", got.Method, got.URL.Path)
	}
	if v := (*form)["urls[]"]; len(v) != 2 || v[0] != "http://192.168.1.4/on" || v[1] != "http://192.168.1.5/on" {
		t.Errorf("urls[] = %q, want two separate parameters, never one comma-joined value", v)
	}
	if (*form).Get("index") != "1" || (*form).Get("name") != "out_on_url" || (*form).Get("enabled") != "true" {
		t.Errorf("form %v does not carry index, name and enabled", *form)
	}
}

// The reference writes the key as a literal urls[]=, and a firmware's form parser may compare key
// names without decoding them, so the key goes out with its brackets as they are. Go's own
// ParseForm decodes either spelling, so only the raw body can tell.
func TestGen1Client_SetActionFormSendsTheKeyLiterally(t *testing.T) {
	cases := map[string]struct {
		enabled *bool
		urls    []string
		want    string
	}{
		"a list": {
			boolPtr(true), []string{"http://192.168.1.4/on?a=b&c=d", "http://192.168.1.5/on"},
			"enabled=true&index=1&name=out_on_url&urls[]=http%3A%2F%2F192.168.1.4%2Fon%3Fa%3Db%26c%3Dd&urls[]=http%3A%2F%2F192.168.1.5%2Fon",
		},
		"a clear": {nil, []string{}, "index=1&name=out_on_url&urls[]="},
		"no list": {boolPtr(false), nil, "enabled=false&index=1&name=out_on_url"},
	}
	for name, tc := range cases {
		var raw string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read body: %v", err)
			}
			raw = string(body)
			_, _ = w.Write([]byte(`{}`))
		}))
		client := NewClient(strings.TrimPrefix(server.URL, "http://"))

		err := client.SetActionForm(context.Background(), 1, "out_on_url", tc.enabled, tc.urls)
		server.Close()

		if err != nil {
			t.Fatalf("%s: SetActionForm: %v", name, err)
		}
		if raw != tc.want {
			t.Errorf("%s: body = %q, want %q", name, raw, tc.want)
		}
	}
}

func TestGen1Client_SetActionFormClearSendsEmptyUrlsParameter(t *testing.T) {
	client, _, form := actionsServer(t, http.StatusOK, `{}`)

	if err := client.SetActionForm(context.Background(), 0, "btn_on_url", nil, []string{}); err != nil {
		t.Fatalf("SetActionForm: %v", err)
	}
	if v, ok := (*form)["urls[]"]; !ok || len(v) != 1 || v[0] != "" {
		t.Errorf("urls[] = %q (present %v), want one empty value, the device's clear", v, ok)
	}
	if _, ok := (*form)["enabled"]; ok {
		t.Errorf("enabled was sent although it was not asked for: %v", *form)
	}
}

func TestGen1Client_SetActionFormLeavesURLsAloneWhenNil(t *testing.T) {
	client, _, form := actionsServer(t, http.StatusOK, `{}`)

	if err := client.SetActionForm(context.Background(), 0, "btn_on_url", boolPtr(false), nil); err != nil {
		t.Fatalf("SetActionForm: %v", err)
	}
	if _, ok := (*form)["urls[]"]; ok {
		t.Errorf("urls[] was sent although no URL list was asked for: %v", *form)
	}
	if (*form).Get("enabled") != "false" {
		t.Errorf("enabled = %q, want false", (*form).Get("enabled"))
	}
}

func TestGen1Client_SetActionFormEncodesURLSpecialCharacters(t *testing.T) {
	client, _, form := actionsServer(t, http.StatusOK, `{}`)

	want := "http://h/a?x=1&y=2%203#f"
	if err := client.SetActionForm(context.Background(), 0, "out_on_url", nil, []string{want}); err != nil {
		t.Fatalf("SetActionForm: %v", err)
	}
	if v := (*form)["urls[]"]; len(v) != 1 || v[0] != want {
		t.Errorf("urls[] = %q, want the URL unchanged after form decoding", v)
	}
}

func TestGen1Client_GetActionsParsesTheDeviceAnswer(t *testing.T) {
	client, got, _ := actionsServer(t, http.StatusOK, `{"actions":{"btn_on_url":[{"index":0,"urls":["http://a/1","http://a/2"],"enabled":true}],"out_on_url":[{"index":0,"urls":[],"enabled":false},{"index":1,"urls":["http://b"],"enabled":true}]}}`)

	live, err := client.GetActions(context.Background())
	if err != nil {
		t.Fatalf("GetActions: %v", err)
	}
	if got.Method != http.MethodGet || got.URL.Path != "/settings/actions" {
		t.Errorf("read went to %s %s, want GET /settings/actions", got.Method, got.URL.Path)
	}
	btn := live["btn_on_url"]
	if len(btn) != 1 || btn[0].Index != 0 || !btn[0].Enabled || len(btn[0].URLs) != 2 || btn[0].URLs[1] != "http://a/2" {
		t.Errorf("btn_on_url = %+v, does not carry the device answer", btn)
	}
	out := live["out_on_url"]
	if len(out) != 2 || out[1].Index != 1 || out[0].Enabled || len(out[0].URLs) != 0 {
		t.Errorf("out_on_url = %+v, does not carry the device answer", out)
	}
}

func TestGen1Client_ActionsRoutesFailWithAuthAndStatusErrors(t *testing.T) {
	client, _, _ := actionsServer(t, http.StatusUnauthorized, ``)
	if err := client.SetActionForm(context.Background(), 0, "btn_on_url", nil, []string{"http://a"}); !errors.Is(err, shelly.ErrAuthRequired) {
		t.Errorf("write on 401 = %v, want ErrAuthRequired", err)
	}
	if _, err := client.GetActions(context.Background()); !errors.Is(err, shelly.ErrAuthRequired) {
		t.Errorf("read on 401 = %v, want ErrAuthRequired", err)
	}

	client, _, _ = actionsServer(t, http.StatusBadRequest, ``)
	var derr *shelly.DeviceError
	if err := client.SetActionForm(context.Background(), 0, "btn_on_url", nil, []string{"http://a"}); !errors.As(err, &derr) || derr.StatusCode != http.StatusBadRequest {
		t.Errorf("write on 400 = %v, want a DeviceError with status 400", err)
	}
	if _, err := client.GetActions(context.Background()); !errors.As(err, &derr) || derr.StatusCode != http.StatusBadRequest {
		t.Errorf("read on 400 = %v, want a DeviceError with status 400", err)
	}
}

func TestGen1Client_PostFormStillJoinsListsWithAComma(t *testing.T) {
	client, _, form := actionsServer(t, http.StatusOK, `{}`)

	err := client.postForm(context.Background(), "http://"+client.ip+"/settings/relay/0", map[string]interface{}{
		"schedule_rules": []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("postForm: %v", err)
	}
	if got := (*form).Get("schedule_rules"); got != "a,b" {
		t.Errorf("schedule_rules = %q, want the comma-joined form every other route takes", got)
	}
}
