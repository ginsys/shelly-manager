package gen2

import "context"

// CallConfigSetter calls a Gen2 <Component>.SetConfig method with the given parameters and
// returns the restart_required field of the answer: true when the device says a reboot is needed
// before the new configuration runs. An answer without the field, or a null answer, means false.
// The per-component setters of this client (SetSysConfig, SetMQTTConfig and the rest) drop the
// answer, so the configuration apply uses this call instead.
func (c *Client) CallConfigSetter(ctx context.Context, method string, params map[string]interface{}) (bool, error) {
	var answer struct {
		RestartRequired bool `json:"restart_required"`
	}
	if err := c.rpcCall(ctx, method, params, &answer); err != nil {
		return false, err
	}
	return answer.RestartRequired, nil
}
