package provider

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The SQLite driver builds its open error from the DSN query it refuses, so an
// operator's value can reach the error Connect returns and the log. The error names
// the parameter key and never a value (ginsys/shelly-manager#596).

// connectSQLiteWithQuery opens a SQLite provider on a file DSN with the given query and
// returns what the log holds and the error Connect returned.
func connectSQLiteWithQuery(t *testing.T, query string) (logged string, err error) {
	t.Helper()
	logger, readLog := fileLogger(t)
	s := NewSQLiteProvider(logger)
	dsn := "file:" + filepath.Join(t.TempDir(), "open.db") + "?" + query

	err = s.Connect(DatabaseConfig{Provider: "sqlite", DSN: dsn, LogLevel: "info"})
	if err == nil {
		_ = s.Close()
	}
	return readLog(), err
}

func TestSQLiteOpenError_NamesTheParameterKeyNotItsValue(t *testing.T) {
	for name, tc := range map[string]struct{ query, key string }{
		"_txlock":      {"_txlock=" + leakWord, "_txlock"},
		"_time_format": {"_time_format=" + leakWord, "_time_format"},
		// Two values of one key: the driver quotes both.
		"vfs": {"vfs=" + leakWord + "&vfs=" + leakWord + "2", "vfs"},
		// A value that is another key's name: the key the driver names is the first one.
		"a value that is another key": {"_txlock=_time_format", "_txlock"},
	} {
		t.Run(name, func(t *testing.T) {
			logged, err := connectSQLiteWithQuery(t, tc.query)

			require.Error(t, err)
			assert.NotContains(t, err.Error(), leakWord)
			assert.NotContains(t, err.Error(), "unknown", "the driver's sentence")
			assert.NotContains(t, err.Error(), "conflicting", "the driver's sentence")
			assert.Contains(t, err.Error(), "invalid DSN parameter ("+tc.key+")")
			assert.NotContains(t, logged, leakWord)
		})
	}
}

// A refusal whose text names no key of the DSN names no key: the driver's text is never
// the source of what is printed.
func TestSQLiteOpenError_NamesNoKeyTheDriverDidNotName(t *testing.T) {
	for name, query := range map[string]string{
		"mode":                     "mode=" + leakWord,
		"a bad escape":             "_txlock=%zz" + leakWord,
		"a bad escape after a key": "_busy_timeout=5000&_txlock=%zz" + leakWord,
		"a pragma syntax error":    "_pragma=)(" + leakWord,
	} {
		t.Run(name, func(t *testing.T) {
			logged, err := connectSQLiteWithQuery(t, query)

			require.Error(t, err)
			assert.NotContains(t, err.Error(), leakWord)
			assert.NotContains(t, err.Error(), "%zz", "a slice of the value")
			assert.NotContains(t, err.Error(), "invalid DSN parameter")
			assert.NotContains(t, logged, leakWord)
		})
	}
}

func TestSealDSNParameterError_MatchesWholeKeysOnly(t *testing.T) {
	driverErr := errors.New(`unknown _time_format "x"`)

	got := sealDSNParameterError("file:a.db?_time=1&_time_format=x", driverErr)

	require.Error(t, got)
	assert.Contains(t, got.Error(), "(_time_format)")
	assert.NotContains(t, got.Error(), "(_time)", "a key that is a prefix of the named key")
	assert.NotContains(t, got.Error(), `"x"`, "the driver's text")
}

func TestSealDSNParameterError_KeepsNilAndAnUnnamedFailure(t *testing.T) {
	assert.NoError(t, sealDSNParameterError("file:a.db?_txlock=x", nil))

	got := sealDSNParameterError("file:a.db", errors.New("boom "+leakWord))

	require.Error(t, got)
	assert.NotContains(t, got.Error(), leakWord)
	assert.NotContains(t, got.Error(), "invalid DSN parameter")
}
