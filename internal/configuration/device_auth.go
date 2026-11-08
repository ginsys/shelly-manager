package configuration

import "encoding/json"

// LegacyDeviceAuth returns the device credentials that an older UpdateDeviceAuth stored as
// settings.auth.{username,password}, a shape no client builder read. It reports ok only when the
// settings hold a complete legacy pair and no flat auth_user or auth_pass value, so credentials
// saved in the current shape always win. Discovery writes empty auth_user and auth_pass keys, so
// an empty flat value does not count as set.
func LegacyDeviceAuth(settings string) (user, pass string, ok bool) {
	if settings == "" {
		return "", "", false
	}
	var flat struct {
		AuthUser string          `json:"auth_user"`
		AuthPass string          `json:"auth_pass"`
		Auth     json.RawMessage `json:"auth"`
	}
	if err := json.Unmarshal([]byte(settings), &flat); err != nil {
		return "", "", false
	}
	if flat.AuthUser != "" || flat.AuthPass != "" || len(flat.Auth) == 0 {
		return "", "", false
	}
	var legacy struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	// Discovery stores "auth" as a boolean; that is not a credential pair.
	if err := json.Unmarshal(flat.Auth, &legacy); err != nil {
		return "", "", false
	}
	if legacy.Username == "" || legacy.Password == "" {
		return "", "", false
	}
	return legacy.Username, legacy.Password, true
}
