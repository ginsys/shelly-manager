package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A config that does not load shows one fixed sentence and the file's path, and nothing the decoders say
// (ginsys/shelly-manager#872). Both decoders put the value they refused into their error text, so a secret of the
// wrong type in the file, or in a SHELLY_ variable, would reach the log of either program. The sentence is a literal
// here, never read from the code under test.

const (
	loadSeed = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

	notValidSentence = "the configuration is not valid (details withheld, they may quote a secret): check the file's syntax with a YAML linter, compare each setting's type with the sample config (a linter finds a syntax error, never a wrong type), and check that each SHELLY_ environment variable holds a value of the setting's type"
)

// loadSeedPieces are the 16-character pieces of loadSeed: none may appear in anything printed.
func loadSeedPieces() []string {
	return []string{loadSeed[:16], loadSeed[16:32], loadSeed[32:48], loadSeed[48:]}
}

func writeLoadConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "load-failure.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// loadFailureShapes are the secret in layouts that do not load: a pattern on one look of the value would miss most
// of them. Each is the whole of a setting's value.
func loadFailureShapes() map[string]string {
	g := loadSeedPieces()
	return map[string]string{
		"tagged":   "!!int " + loadSeed,
		"hyphens":  "!!int " + strings.Join(g, "-"),
		"spaces":   "!!int " + strings.Join(g, " "),
		"a map":    "{a: " + loadSeed + "}",
		"a list":   "[" + strings.Join(g, ", ") + "]",
		"a number": "!!float " + strings.Join(g, "_"),
		// The decoder quotes the value, so a value that reads like a location must not become one.
		"a line word": `!!int "line ` + strings.Join(g, "-") + `"`,
		// A syntax error, which the decoder reports with a line number of its own.
		"unclosed": "[" + loadSeed,
	}
}

func TestLoadFailure_OneSentenceAndThePath(t *testing.T) {
	loaders := map[string]struct {
		load func(string) (*Config, error)
		head string // the setting the secret sits in
	}{
		"manager": {Load, "security:\n  admin_api_key: "},
		"agent": {func(p string) (*Config, error) { return LoadWithName(p, "shelly-provisioner") },
			"provisioning:\n  agent_seed: "},
	}
	for who, l := range loaders {
		for name, value := range loadFailureShapes() {
			path := writeLoadConfig(t, "logging:\n  level: info\n"+l.head+value+"\n")
			cfg, err := l.load(path)
			require.Error(t, err, "%s, %s", who, name)
			assert.Nil(t, cfg)
			// Exactly the path and the sentence: nothing else, so nothing of the decoder's text.
			assert.Equal(t, path+": "+notValidSentence, err.Error(), "%s, %s", who, name)
			for _, piece := range loadSeedPieces() {
				assert.NotContains(t, err.Error(), piece, "%s, %s", who, name)
			}
			// A file that does not parse never gives the OS's own error, which is the one text let through.
			var pathErr *fs.PathError
			assert.False(t, errors.As(err, &pathErr), "%s, %s", who, name)
		}
	}
}

// A file that loads can still fail on a SHELLY_ variable of the wrong type: the sentence covers both, and the
// variable's value is as withheld as the file's.
func TestLoadFailure_ABadEnvironmentValue(t *testing.T) {
	path := writeLoadConfig(t, "server:\n  host: 127.0.0.1\n")
	t.Setenv("SHELLY_SERVER_PORT", "port"+loadSeed)
	_, err := Load(path)
	require.Error(t, err)
	assert.Equal(t, path+": "+notValidSentence, err.Error())
	for _, piece := range loadSeedPieces() {
		assert.NotContains(t, err.Error(), piece)
	}
}

// The path is the operator's own: the --config argument, or the file the search found, read from the loader and
// never from the decoder's text.
func TestLoadFailure_NamesTheFileTheSearchFound(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "load-failure-872.yaml"), []byte("a: !!int "+loadSeed+"\n"), 0o600))
	_, err := LoadWithName("", "load-failure-872")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load-failure-872.yaml")
	assert.True(t, strings.HasSuffix(err.Error(), ": "+notValidSentence), err.Error())
	for _, piece := range loadSeedPieces() {
		assert.NotContains(t, err.Error(), piece)
	}
}

// A file that is missing or cannot be read is the operator's own path and the OS's own cause, printed as before.
func TestLoadFailure_MissingOrUnreadableFileIsStillNamed(t *testing.T) {
	t.Run("a missing named file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "absent.yaml")
		_, err := Load(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), path)
		assert.ErrorIs(t, err, fs.ErrNotExist)
		assert.NotContains(t, err.Error(), "details withheld")
	})
	t.Run("a file with no read permission", func(t *testing.T) {
		if os.Geteuid() == 0 || runtime.GOOS == "windows" {
			t.Skip("a privileged user reads any file")
		}
		path := writeLoadConfig(t, "server:\n  port: 1\n")
		require.NoError(t, os.Chmod(path, 0))
		_, err := Load(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), path)
		assert.ErrorIs(t, err, fs.ErrPermission)
		assert.NotContains(t, err.Error(), "details withheld")
	})
	t.Run("a directory given as the file", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.Mkdir(dir, 0o700))
		_, err := Load(dir)
		require.Error(t, err)
		var pathErr *fs.PathError
		assert.True(t, errors.As(err, &pathErr), "the OS's own error, a path and a cause: %v", err)
		assert.Contains(t, err.Error(), dir)
		assert.NotContains(t, err.Error(), "details withheld")
	})
	// viper's own message names only the extension of the operator's path, so it is kept (and the sentence about
	// YAML would be wrong for it).
	t.Run("a file whose extension is not a config type", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "load-failure.conf")
		require.NoError(t, os.WriteFile(path, []byte("a: !!int "+loadSeed+"\n"), 0o600))
		_, err := Load(path)
		require.Error(t, err)
		var unsupported viper.UnsupportedConfigError
		assert.True(t, errors.As(err, &unsupported), "viper's own error is kept: %v", err)
		assert.NotContains(t, err.Error(), "details withheld")
		for _, piece := range loadSeedPieces() {
			assert.NotContains(t, err.Error(), piece)
		}
	})
	t.Run("a search that finds no file", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv("HOME", t.TempDir())
		_, err := LoadWithName("", "load-failure-872-none")
		require.Error(t, err)
		var notFound viper.ConfigFileNotFoundError
		assert.True(t, errors.As(err, &notFound), "the agent's fingerprint command reads this: %v", err)
		assert.NotContains(t, err.Error(), "details withheld")
	})
}
