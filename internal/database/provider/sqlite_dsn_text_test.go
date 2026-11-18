package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A SQLite DSN is a file path with an optional query, and the query can hold a
// key (SQLCipher) or any driver parameter. Nothing after the "?" is printed.

func TestSQLiteDSNText_StringAndTargetPrintNoQuery(t *testing.T) {
	cfg := DatabaseConfig{Provider: "sqlite", DSN: "file:data/shelly.db?_pragma_key=" + leakWord + "&mode=rwc"}

	for how, got := range map[string]string{
		"String()":    cfg.String(),
		"%v value":    fmt.Sprintf("%v", cfg),
		"%+v value":   fmt.Sprintf("%+v", cfg),
		"%v pointer":  fmt.Sprintf("%v", &cfg),
		"%+v pointer": fmt.Sprintf("%+v", &cfg),
	} {
		assert.NotContains(t, got, leakWord, how)
		assert.Contains(t, got, "file:data/shelly.db", how+": the path stays")
	}
}

func TestSQLiteDSNText_ConnectLogPrintsNoQuery(t *testing.T) {
	logger, readLog := fileLogger(t)
	p := NewSQLiteProvider(logger)
	dsn := filepath.Join(t.TempDir(), "shelly.db") + "?_busy_timeout=5000&_pragma_key=" + leakWord

	require.NoError(t, p.Connect(DatabaseConfig{Provider: "sqlite", DSN: dsn, LogLevel: "info"}))
	t.Cleanup(func() { _ = p.Close() })

	logged := readLog()
	assert.Contains(t, logged, "Connected to SQLite database", "the log must hold the connect line, or this proves nothing")
	assert.NotContains(t, logged, leakWord)
}

func TestSQLiteDSNText_DirectoryComesFromThePathPart(t *testing.T) {
	// The directory of the database is the directory of the path, never of the
	// query: a "/" in a query value must not make a directory or print the query.
	root := t.TempDir()
	p := NewSQLiteProvider(nil)

	err := p.prepareDatabasePath(filepath.Join(root, "shelly.db") + "?k=a/" + leakWord)

	require.NoError(t, err)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	assert.Empty(t, entries, "no directory is made from the query")
}
