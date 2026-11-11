package api

import "runtime/debug"

// buildRevision returns the version control revision this binary was built from, or "" when the
// binary carries none. `go build` inside a git checkout stamps it; `-buildvcs=false`, a build
// outside a repository and a `go run` build leave it out.
func buildRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return revisionFromSettings(info.Settings)
}

// revisionFromSettings reads the vcs.revision value from build settings.
func revisionFromSettings(settings []debug.BuildSetting) string {
	for _, s := range settings {
		if s.Key == "vcs.revision" {
			return s.Value
		}
	}
	return ""
}
