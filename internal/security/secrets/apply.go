package secrets

import (
	"github.com/ginsys/shelly-manager/internal/config"
)

// ApplyToConfig overrides sensitive config fields using env/file-based secrets.
// This preserves existing config values unless a corresponding env or *_FILE
// variable is provided.
//
// It returns the first error and stops: a *_FILE variable that names a file the
// process cannot read. The config may then be partly updated, so the caller must
// not run on it (an empty admin key leaves every guarded route open).
//
// Supported keys (env or env_FILE):
// - SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD
// - SHELLY_OPNSENSE_API_KEY
// - SHELLY_OPNSENSE_API_SECRET
// - SHELLY_SECURITY_ADMIN_API_KEY (protects sensitive endpoints and WS)
// - SHELLY_SECURITY_AGENT_API_KEY (the manager's key for the provisioner agent routes)
// - SHELLY_API_KEY (used by provisioner agent config)
// - SHELLY_PROVISIONING_AGENT_SEED (the provisioner agent's signing seed; the manager reads it and uses it nowhere)
//
// Note: Viper already supports direct env overrides (SHELLY_*). This function
// adds the common *_FILE convention and centralizes sensitive-field handling.
func ApplyToConfig(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}

	for _, s := range []struct {
		target *string
		key    string
	}{
		{&cfg.Notifications.Email.SMTPPassword, "SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD"},
		{&cfg.OPNSense.APIKey, "SHELLY_OPNSENSE_API_KEY"},
		{&cfg.OPNSense.APISecret, "SHELLY_OPNSENSE_API_SECRET"},
		{&cfg.Security.AdminAPIKey, "SHELLY_SECURITY_ADMIN_API_KEY"},
		{&cfg.Security.AgentAPIKey, "SHELLY_SECURITY_AGENT_API_KEY"},
		{&cfg.API.Key, "SHELLY_API_KEY"},
		{&cfg.Provisioning.AgentSeed, "SHELLY_PROVISIONING_AGENT_SEED"},
	} {
		v, err := OverrideIfPresent(*s.target, s.key)
		if err != nil {
			return err
		}
		*s.target = v
	}
	return nil
}
