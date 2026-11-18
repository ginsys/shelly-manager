package gen2

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A switch's auto_on and auto_off read back as a whole number, and as no value (the zero) for a
// fraction or a number no int holds exactly, so a stored 7 never stands for a live 7.5.
func TestClient_GetConfig_SwitchAutoOnOffWholeNumbersOnly(t *testing.T) {
	if ln, err := net.Listen("tcp4", "127.0.0.1:0"); err != nil {
		t.Skipf("Skipping due to restricted socket permissions: %v", err)
	} else {
		_ = ln.Close()
	}

	cases := []struct {
		name   string
		value  interface{} // nil means the key is absent
		absent bool
		want   int
	}{
		{name: "fractional", value: 7.5, want: 0},
		{name: "fractional below one", value: 0.5, want: 0},
		{name: "whole", value: 300.0, want: 300},
		{name: "whole zero", value: 0.0, want: 0},
		{name: "absent", absent: true, want: 0},
		{name: "out of range high", value: 1e300, want: 0},
		{name: "out of range low", value: -1e300, want: 0},
		{name: "one past the exact integers", value: 9007199254740992.0, want: 0},
	}

	for _, tc := range cases {
		for _, key := range []string{"auto_on", "auto_off"} {
			t.Run(tc.name+"/"+key, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var req RPCRequest
					_ = json.NewDecoder(r.Body).Decode(&req)
					sw := map[string]interface{}{"name": "Switch 0"}
					if !tc.absent {
						sw[key] = tc.value
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"id":     req.ID,
						"result": map[string]interface{}{"switch:0": sw},
					})
				}))
				defer server.Close()

				config, err := NewClient(server.URL[len("http://"):]).GetConfig(context.Background())
				assertNoError(t, err)
				assertEqual(t, 1, len(config.Switches))
				got := config.Switches[0].AutoOn
				other := config.Switches[0].AutoOff
				if key == "auto_off" {
					got, other = other, got
				}
				assertEqual(t, tc.want, got)
				assertEqual(t, 0, other)
			})
		}
	}
}
