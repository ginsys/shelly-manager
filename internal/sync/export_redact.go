package sync

import "github.com/ginsys/shelly-manager/internal/redact"

// maskExportSecrets replaces every saved secret in the records an export hands to a plugin with
// RedactedValue: the device settings, each device configuration, and the templates with their
// variables. A request that sets exclude_sensitive to false asks for a full export and keeps them,
// the same key the SMA export reads. Any other value, or none, masks, so a mistyped value fails
// closed. The SMA export redacts again by its own wider key rule, which does nothing to a value
// already masked.
func maskExportSecrets(data *ExportData, config map[string]interface{}) {
	if exclude, ok := config["exclude_sensitive"].(bool); ok && !exclude {
		return
	}
	for _, device := range data.Devices {
		redact.Mask(device.Settings, RedactedValue)
	}
	for _, item := range data.Configurations {
		redact.Mask(item.Config, RedactedValue)
	}
	for _, template := range data.Templates {
		redact.Mask(template.Config, RedactedValue)
		redact.Mask(template.Variables, RedactedValue)
	}
}
