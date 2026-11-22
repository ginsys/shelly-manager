package provider

import (
	"strings"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// The MySQL provider reads a DSN with the driver's own parse. These tests read
// what buildDSN returns the way the driver does, so a decision made on the text
// of the DSN shows as a different parsed value.

func buildMySQLDSN(t *testing.T, dsn string, options map[string]string) string {
	t.Helper()
	built, err := NewMySQLProvider(logging.GetDefault()).buildDSN(dsn, options)
	require.NoError(t, err)
	return built
}

func parseMySQLDSN(t *testing.T, dsn string) *mysqldriver.Config {
	t.Helper()
	cfg, err := mysqldriver.ParseDSN(dsn)
	require.NoError(t, err, "the driver reads the DSN")
	return cfg
}

// driverParams is the driver's own canonical form of the parameters of a parsed
// DSN, without the user, the password, the database name and the unknown keys.
func driverParams(cfg *mysqldriver.Config) string {
	c := cfg.Clone()
	c.User, c.Passwd, c.DBName, c.Params = "", "", "", nil
	return c.FormatDSN()
}

// paramPairs lists the pairs of the parameter section of a DSN, cut after its last
// "/" and its first "?".
func paramPairs(dsn string) []string {
	_, rest, _ := strings.Cut(dsn[strings.LastIndex(dsn, "/")+1:], "?")
	return strings.Split(rest, "&")
}

func countKey(dsn, key string) int {
	n := 0
	for _, pair := range paramPairs(dsn) {
		if k, _, found := strings.Cut(pair, "="); found && k == key {
			n++
		}
	}
	return n
}

// What the provider accepts is what the driver parses. A word that looks like SQL
// in a host, a database name or a password is part of the value it sits in.
func TestMySQLBuildDSN_StructureAndText(t *testing.T) {
	accepted := map[string]string{
		"a host holding a reserved-looking word":     "user:pw@tcp(executor:3306)/db",
		"a database holding a reserved-looking word": "user:pw@tcp(h:3306)/sp_data",
		"a password with a semicolon and dashes":     "user:p;--x@tcp(h:3306)/db",
		"a database name with blanks and words":      "user:pw@tcp(h:3306)/drop table users",
		"a session setting the driver does not know": "user:pw@tcp(h:3306)/db?unlisted_setting=1",
	}
	for name, dsn := range accepted {
		t.Run(name, func(t *testing.T) {
			built := buildMySQLDSN(t, dsn, nil)
			cfg := parseMySQLDSN(t, built)
			if strings.Contains(dsn, "unlisted_setting") {
				assert.Equal(t, "1", cfg.Params["unlisted_setting"], "the setting is left for the server")
			}
		})
	}

	t.Run("a DSN the driver cannot parse names none of its text", func(t *testing.T) {
		const marker = "s3cr3tmark"
		err := NewMySQLProvider(logging.GetDefault()).Connect(DatabaseConfig{
			Provider: "mysql",
			DSN:      "user:" + marker + "@tcp(h:3306/db",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), errDSNNotParsed.Error())
		assert.NotContains(t, err.Error(), marker)
	})
}

// A timeout option is read as a duration, the way the driver reads it, and refused
// otherwise with a message that names the option and not its value. Every duration
// spelling the driver reads keeps working. The other options are escaped so that
// each stays one value.
func TestMySQLBuildDSN_OptionsAreEscaped(t *testing.T) {
	for _, key := range []string{"timeout", "readTimeout", "writeTimeout"} {
		t.Run(key+" must be a duration", func(t *testing.T) {
			const word = "sometime"
			_, err := NewMySQLProvider(logging.GetDefault()).buildDSN("user:pw@tcp(h:3306)/db", map[string]string{key: word})
			require.Error(t, err)
			assert.Contains(t, err.Error(), key)
			assert.NotContains(t, err.Error(), word)
		})
		t.Run(key+" keeps every duration spelling", func(t *testing.T) {
			for value, want := range map[string]string{"+5s": "5s", "1.5s": "1.5s", "1µs": "1µs", "2m": "2m0s"} {
				built := buildMySQLDSN(t, "user:pw@tcp(h:3306)/db", map[string]string{key: value})
				cfg := parseMySQLDSN(t, built)
				got := map[string]string{"timeout": cfg.Timeout.String(), "readTimeout": cfg.ReadTimeout.String(), "writeTimeout": cfg.WriteTimeout.String()}[key]
				assert.Equal(t, want, got, value)
			}
		})
	}
	t.Run("loc is decoded by the driver, so it is escaped", func(t *testing.T) {
		built := buildMySQLDSN(t, "user:pw@tcp(h:3306)/db", map[string]string{"loc": "Europe/Paris"})
		assert.Equal(t, "Europe/Paris", parseMySQLDSN(t, built).Loc.String())
	})
	for _, key := range []string{"charset", "collation"} {
		t.Run(key+" is one value", func(t *testing.T) {
			built := buildMySQLDSN(t, "user:pw@tcp(h:3306)/db", map[string]string{key: "latin1&wait_timeout=1"})
			cfg := parseMySQLDSN(t, built)
			assert.Empty(t, cfg.Params, "the value parses and adds no parameter")
		})
	}
}

// A charset option may name several character sets, separated by a comma, and the
// driver tries them in order. Escaping the option keeps the comma a comma.
func TestMySQLBuildDSN_CharsetListStaysAList(t *testing.T) {
	built := buildMySQLDSN(t, "user:pw@tcp(h:3306)/db", map[string]string{"charset": "utf8mb4,latin1"})
	assert.Contains(t, driverParams(parseMySQLDSN(t, built)), "charset=utf8mb4,latin1")
}

// A default is decided from the parameters of the DSN, never from a word of the
// whole text: a password that holds the name of a key does not stop the default.
func TestMySQLBuildDSN_DefaultsFromKeys(t *testing.T) {
	for _, password := range []string{"tls=true", "timeout=1s", "readTimeout=1s", "writeTimeout=1s", "charset=x", "collation=x", "a?b"} {
		t.Run("a password holding "+password, func(t *testing.T) {
			built := buildMySQLDSN(t, "user:"+password+"@tcp(h:3306)/db", map[string]string{"collation": "utf8mb4_unicode_ci"})
			cfg := parseMySQLDSN(t, built)
			assert.Equal(t, password, cfg.Passwd)
			assert.Equal(t, "db", cfg.DBName)
			assert.Equal(t, "preferred", cfg.TLSConfig)
			assert.EqualValues(t, "10s", cfg.Timeout.String())
			assert.EqualValues(t, "30s", cfg.ReadTimeout.String())
			assert.EqualValues(t, "30s", cfg.WriteTimeout.String())
			assert.Equal(t, "utf8mb4_unicode_ci", cfg.Collation)
			assert.Contains(t, driverParams(cfg), "charset=utf8mb4")
			assert.Empty(t, cfg.Params)
		})
	}

	t.Run("an explicit zero and an explicit false are kept", func(t *testing.T) {
		cfg := parseMySQLDSN(t, buildMySQLDSN(t, "user:pw@tcp(h:3306)/db?readTimeout=0&tls=false", nil))
		assert.Zero(t, cfg.ReadTimeout, "no limit stays no limit")
		assert.Equal(t, "false", cfg.TLSConfig)
		assert.Equal(t, "10s", cfg.Timeout.String(), "what is not set still gets its default")
	})

	t.Run("an empty parameter section takes the defaults", func(t *testing.T) {
		cfg := parseMySQLDSN(t, buildMySQLDSN(t, "user:pw@tcp(h:3306)/db?", nil))
		assert.Equal(t, "db", cfg.DBName)
		assert.Equal(t, "preferred", cfg.TLSConfig)
	})
}

// The provider finds the keys of a DSN the way the driver does, so an explicit
// value is the value the driver reads from the built DSN, once, and the default
// does not follow it.
func TestMySQLBuildDSN_CutterMatchesDriver(t *testing.T) {
	explicit := map[string]func(*mysqldriver.Config) string{
		"tls=true":              func(c *mysqldriver.Config) string { return c.TLSConfig },
		"timeout=3s":            func(c *mysqldriver.Config) string { return c.Timeout.String() },
		"readTimeout=4s":        func(c *mysqldriver.Config) string { return c.ReadTimeout.String() },
		"writeTimeout=5s":       func(c *mysqldriver.Config) string { return c.WriteTimeout.String() },
		"collation=utf8mb4_bin": func(c *mysqldriver.Config) string { return c.Collation },
		"charset=latin1":        func(c *mysqldriver.Config) string { return driverParams(c) },
	}
	for pair, read := range explicit {
		key, value, _ := strings.Cut(pair, "=")
		t.Run(key, func(t *testing.T) {
			built := buildMySQLDSN(t, "user:pw@tcp(h:3306)/db?"+pair, map[string]string{
				"tls": "skip-verify", "timeout": "9s", "readTimeout": "9s", "writeTimeout": "9s", "charset": "utf8", "collation": "utf8mb4_unicode_ci",
			})
			assert.Equal(t, 1, countKey(built, key), "the key occurs once")
			assert.Contains(t, read(parseMySQLDSN(t, built)), value)
		})
	}

	t.Run("a pair with no equals sign is not a key", func(t *testing.T) {
		built := buildMySQLDSN(t, "user:pw@tcp(h:3306)/db?timeout", nil)
		assert.Equal(t, "10s", parseMySQLDSN(t, built).Timeout.String(), "the driver skips the pair, so the default applies")
	})
}
