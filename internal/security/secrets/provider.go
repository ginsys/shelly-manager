package secrets

import (
	"fmt"
	"os"
	"strings"
)

// Provider abstracts a secret source.
// Implementations may read from environment variables, files, or secret stores.
type Provider interface {
	// Get returns the secret value for a given key if present.
	// The boolean indicates whether a value was found.
	Get(key string) (string, bool)
}

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
func (p *EnvProvider) Get(key string) (string, bool) {
	// Direct value takes precedence
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v, true
	}
	// "_FILE" indirection (Docker/K8s secrets convention)
	if path, ok := os.LookupEnv(key + "_FILE"); ok && path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			// Surface a clear error in logs via wrapper caller, but here
			// indicate not found to allow fallbacks.
			return "", false
		}
		// Trim the trailing newlines commonly present in secret files
		s := string(b)
		s = strings.TrimRight(s, "\n")
		if s == "" {
			return "", false
		}
		return s, true
	}
	return "", false
}

// MultiProvider checks each provider in order and returns the first hit.
type MultiProvider struct{ providers []Provider }

// NewMultiProvider creates a new MultiProvider chain.
func NewMultiProvider(providers ...Provider) *MultiProvider {
	return &MultiProvider{providers: providers}
}

// Get queries each provider in order and returns the first present value.
func (m *MultiProvider) Get(key string) (string, bool) {
	for _, p := range m.providers {
		if v, ok := p.Get(key); ok {
			return v, true
		}
	}
	return "", false
}

// Helper: GetEnvOrFile fetches a secret using the common convention.
// It is a convenience wrapper for one-off lookups without constructing a provider.
func GetEnvOrFile(key string) (string, bool) {
	return NewEnvProvider().Get(key)
}

// Helper: OverrideIfPresent returns envValue if present, otherwise original.
func OverrideIfPresent(original string, envKey string) string {
	if v, ok := GetEnvOrFile(envKey); ok {
		return v
	}
	return original
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
