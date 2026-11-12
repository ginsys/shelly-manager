package gen2

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// rpcFake answers every RPC with the given result JSON, or with an RPC error when errMessage is
// set, and keeps the last request it saw.
func rpcFake(t *testing.T, result, errMessage string) (*Client, *map[string]interface{}) {
	t.Helper()
	if ln, err := net.Listen("tcp4", "127.0.0.1:0"); err != nil {
		t.Skipf("Skipping due to restricted socket permissions: %v", err)
	} else {
		_ = ln.Close()
	}
	seen := map[string]interface{}{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = map[string]interface{}{}
		_ = json.NewDecoder(r.Body).Decode(&seen)
		w.Header().Set("Content-Type", "application/json")
		if errMessage != "" {
			_, _ = w.Write([]byte(`{"id":1,"error":{"code":-103,"message":"` + errMessage + `"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":1,"result":` + result + `}`))
	}))
	t.Cleanup(server.Close)
	return NewClient(server.URL[len("http://"):]), &seen
}

// The Gen2 setters used to drop the answer of the device, so the restart_required field of
// Sys.SetConfig and MQTT.SetConfig was never read. CallConfigSetter returns it.
func TestClient_CallConfigSetter_ReturnsRestartRequired(t *testing.T) {
	cases := []struct {
		name   string
		result string
		want   bool
	}{
		{"restart needed", `{"restart_required":true}`, true},
		{"no restart", `{"restart_required":false}`, false},
		{"field missing", `{}`, false},
		{"null answer", `null`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, seen := rpcFake(t, tc.result, "")
			got, err := client.CallConfigSetter(context.Background(), "Sys.SetConfig",
				map[string]interface{}{"config": map[string]interface{}{"device": map[string]interface{}{"name": "lamp"}}})
			assertNoError(t, err)
			assertEqual(t, tc.want, got)
			assertEqual(t, "Sys.SetConfig", (*seen)["method"].(string))
			params := (*seen)["params"].(map[string]interface{})
			device := params["config"].(map[string]interface{})["device"].(map[string]interface{})
			assertEqual(t, "lamp", device["name"].(string))
		})
	}
}

func TestClient_CallConfigSetter_ReportsADeviceError(t *testing.T) {
	client, _ := rpcFake(t, "", "invalid argument")
	restart, err := client.CallConfigSetter(context.Background(), "Sys.SetConfig", map[string]interface{}{})
	assertError(t, err)
	assertTrue(t, !restart)
}
