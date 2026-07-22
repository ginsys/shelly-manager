package configuration

// Template scope vocabulary. A template applies globally, to a tagged group, or
// to a single device type.
const (
	ScopeGlobal     = "global"
	ScopeGroup      = "group"
	ScopeDeviceType = "device_type"
)

// ValidateTemplateScope is the single definition of the config_templates scope
// invariant: the scope must be one of the three known values, and a device-type
// scope must name a concrete device type.
//
// Every writer of the table calls this — ConfigurationService (the repository
// path), Service.CreateTemplate/UpdateTemplate (the legacy path) and the
// upgrade preflight in internal/database/migrations.go. Keeping them on one
// rule is what stops the API from writing rows that the next startup refuses to
// migrate; see issue #275.
func ValidateTemplateScope(scope, deviceType string) error {
	switch scope {
	case ScopeGlobal, ScopeGroup:
		return nil
	case ScopeDeviceType:
		if deviceType == "" {
			return ErrDeviceTypeRequired
		}
		return nil
	default:
		return ErrInvalidScope
	}
}
