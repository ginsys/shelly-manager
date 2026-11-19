package secrets

import (
	"fmt"
	"os"
	"strings"
)

// EnvProvider reads secrets from environment variables using a conventional
// "NAME" or "NAME_FILE" pattern (the latter treated as a filesystem path).
type EnvProvider struct{}

// NewEnvProvider creates a new EnvProvider.
func NewEnvProvider() *EnvProvider { return &EnvProvider{} }

// Get returns the value for ENV "key" if it is set and not empty; otherwise, if "key_FILE" names a
// readable file, returns the file contents trimmed of trailing newlines. An empty value counts as
// absent, for the variable and for the file content alike (viper treats an empty env value the same
// way), so a Compose file that always sets the plain variable, empty by default, still reads the file,
// and an unfilled key file never blanks a key set in the config file.
//
// A "key_FILE" that is set to a path the process cannot read is an error naming the variable and the
// path: a typo in a mount path must not leave the key empty, which opens the guarded routes. A plain
// value that wins is returned before the file is looked at, so it hides an unreadable file.
func (p *EnvProvider) Get(key string) (string, bool, error) {
	// Direct value takes precedence
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v, true, nil
	}
	// "_FILE" indirection (Docker/K8s secrets convention)
	if path, ok := os.LookupEnv(key + "_FILE"); ok && path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", false, fmt.Errorf("%s_FILE: %w", key, err)
		}
		// Trim the trailing newlines commonly present in secret files
		s := string(b)
		s = strings.TrimRight(s, "\n")
		if s == "" {
			return "", false, nil
		}
		return s, true, nil
	}
	return "", false, nil
}

// Helper: GetEnvOrFile fetches a secret using the common convention.
// It is a convenience wrapper for one-off lookups without constructing a provider.
func GetEnvOrFile(key string) (string, bool, error) {
	return NewEnvProvider().Get(key)
}

// Helper: OverrideIfPresent returns the secret for envKey if present, otherwise original. It fails
// when the secret's _FILE variable names a file that cannot be read.
func OverrideIfPresent(original string, envKey string) (string, error) {
	v, ok, err := GetEnvOrFile(envKey)
	if err != nil {
		return original, err
	}
	if ok {
		return v, nil
	}
	return original, nil
}

// DebugString redacts secret values for logs.
func DebugString(name, value string) string {
	if value == "" {
		return fmt.Sprintf("%s=<empty>", name)
	}
	// Show only prefix to confirm selection while redacting majority
	if len(value) <= 4 {
		return fmt.Sprintf("%s=****", name)
	}
	return fmt.Sprintf("%s=%s****", name, value[:4])
}
