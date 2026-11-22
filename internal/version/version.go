// Package version holds the one version of the application. A release build stamps it with
// -ldflags "-X github.com/ginsys/shelly-manager/internal/version.Version=<value>".
package version

// Version is the version this binary was built as, or "dev" when the build did not stamp one.
// It is a variable, not a constant, because -X cannot set a constant.
var Version = "dev"
