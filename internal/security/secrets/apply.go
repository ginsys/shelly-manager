package secrets

import (
	"github.com/ginsys/shelly-manager/internal/config"
)

// ApplyToConfig overrides sensitive config fields using env/file-based secrets.
// This preserves existing config values unless a corresponding env or *_FILE
// variable is provided.
//
// Supported keys (env or env_FILE):
// - SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD
// - SHELLY_OPNSENSE_API_KEY
// - SHELLY_OPNSENSE_API_SECRET
// - SHELLY_SECURITY_ADMIN_API_KEY
// - SHELLY_API_KEY (used by provisioner agent config)
//
// Note: Viper already supports direct env overrides (SHELLY_*). This function
// adds the common *_FILE convention and centralizes sensitive-field handling.
func ApplyToConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}

	// Notifications (SMTP)
	cfg.Notifications.Email.SMTPPassword = OverrideIfPresent(
		cfg.Notifications.Email.SMTPPassword,
		"SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD",
	)

	// OPNSense API credentials
	cfg.OPNSense.APIKey = OverrideIfPresent(
		cfg.OPNSense.APIKey,
		"SHELLY_OPNSENSE_API_KEY",
	)
	cfg.OPNSense.APISecret = OverrideIfPresent(
		cfg.OPNSense.APISecret,
		"SHELLY_OPNSENSE_API_SECRET",
	)

	// Admin API key (protects sensitive endpoints and WS)
	cfg.Security.AdminAPIKey = OverrideIfPresent(
		cfg.Security.AdminAPIKey,
		"SHELLY_SECURITY_ADMIN_API_KEY",
	)

	// Provisioner/Agent API key (when running provisioner binary)
	cfg.API.Key = OverrideIfPresent(
		cfg.API.Key,
		"SHELLY_API_KEY",
	)
}
