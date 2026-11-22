package provider

import (
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The PostgreSQL provider builds the DSN from what net/url and pgx read. These
// tests read the built DSN the way they do.

func buildPostgreSQLDSN(t *testing.T, cfg DatabaseConfig, dsn string, options map[string]string) string {
	t.Helper()
	built, err := newCredentialProvider(cfg).buildDSN(dsn, options)
	require.NoError(t, err)
	return built
}

func queryOf(t *testing.T, built string) url.Values {
	t.Helper()
	u, err := url.Parse(built)
	require.NoError(t, err)
	return u.Query()
}

// A setting, a host, a database name and a password are values: a word that looks
// like SQL in one of them is part of the value.
func TestPostgreSQLBuildDSN_TextInValues(t *testing.T) {
	t.Run("default_query_exec_mode is a setting pgx reads", func(t *testing.T) {
		built := buildPostgreSQLDSN(t, DatabaseConfig{}, "postgres://user:pw@h:5432/db?default_query_exec_mode=simple_protocol", nil)
		cfg, err := pgx.ParseConfig(built)
		require.NoError(t, err)
		assert.Equal(t, pgx.QueryExecModeSimpleProtocol, cfg.DefaultQueryExecMode)
	})

	t.Run("default_query_exec_mode as an option is added once, as one value", func(t *testing.T) {
		built := buildPostgreSQLDSN(t, DatabaseConfig{}, "postgres://user:pw@h:5432/db", map[string]string{"default_query_exec_mode": "simple_protocol"})
		cfg, err := pgx.ParseConfig(built)
		require.NoError(t, err)
		assert.Equal(t, pgx.QueryExecModeSimpleProtocol, cfg.DefaultQueryExecMode)
		assert.Len(t, queryOf(t, built)["default_query_exec_mode"], 1)

		escaped := buildPostgreSQLDSN(t, DatabaseConfig{}, "postgres://user:pw@h:5432/db", map[string]string{"default_query_exec_mode": "simple_protocol&sslmode=disable"})
		q := queryOf(t, escaped)
		assert.Equal(t, "simple_protocol&sslmode=disable", q.Get("default_query_exec_mode"))
		assert.Equal(t, "require", q.Get("sslmode"), "the value added no key")
	})

	t.Run("default_query_exec_mode is accepted next to database.username", func(t *testing.T) {
		built := buildPostgreSQLDSN(t, DatabaseConfig{Username: "user", Password: "pw"}, "postgres://h:5432/db?default_query_exec_mode=simple_protocol", nil)
		assert.Equal(t, "simple_protocol", queryOf(t, built).Get("default_query_exec_mode"))
	})

	for name, dsn := range map[string]string{
		"a host holding a reserved-looking word":     "postgres://user:pw@executor:5432/db",
		"a database holding a reserved-looking word": "postgres://user:pw@h:5432/sp_data",
	} {
		t.Run(name, func(t *testing.T) {
			built := buildPostgreSQLDSN(t, DatabaseConfig{}, dsn, nil)
			_, err := pgx.ParseConfig(built)
			assert.NoError(t, err)
		})
	}
}

// One password is one password, whichever way it is configured.
func TestPostgreSQLBuildDSN_OnePasswordTwoSpellings(t *testing.T) {
	const password = "p;--x"
	inDSN := buildPostgreSQLDSN(t, DatabaseConfig{}, "postgres://user:"+password+"@h:5432/db", nil)
	inConfig := buildPostgreSQLDSN(t, DatabaseConfig{Username: "user", Password: password}, "postgres://h:5432/db", nil)
	for name, built := range map[string]string{"the DSN": inDSN, "database.password": inConfig} {
		u, err := url.Parse(built)
		require.NoError(t, err, name)
		got, _ := u.User.Password()
		assert.Equal(t, password, got, name)
	}
}

// A default is decided from the keys of the query, never from a word of the whole
// text: a password that holds the name of a key does not stop the default.
func TestPostgreSQLBuildDSN_DefaultsFromKeys(t *testing.T) {
	for _, password := range []string{"sslmode=x", "connect_timeout=1", "application_name=x"} {
		t.Run("a password holding "+password, func(t *testing.T) {
			built := buildPostgreSQLDSN(t, DatabaseConfig{}, "postgres://user:"+password+"@h:5432/db", map[string]string{"application_name": "shelly"})
			u, err := url.Parse(built)
			require.NoError(t, err)
			got, _ := u.User.Password()
			assert.Equal(t, password, got)
			q := u.Query()
			assert.Equal(t, "require", q.Get("sslmode"))
			assert.Equal(t, "10", q.Get("connect_timeout"))
			assert.Equal(t, "shelly", q.Get("application_name"))
		})
	}

	t.Run("an explicit value is kept and not repeated", func(t *testing.T) {
		built := buildPostgreSQLDSN(t, DatabaseConfig{}, "postgres://user:pw@h:5432/db?sslmode=disable&connect_timeout=3", map[string]string{"sslmode": "verify-full"})
		q := queryOf(t, built)
		assert.Equal(t, []string{"disable"}, q["sslmode"])
		assert.Equal(t, []string{"3"}, q["connect_timeout"])
	})
}

// The checks that read the structure of the DSN are unchanged.
func TestPostgreSQLBuildDSN_StructureStillRefused(t *testing.T) {
	_, err := newCredentialProvider(DatabaseConfig{}).buildDSN("mysql://user:pw@h/db", nil)
	assert.ErrorContains(t, err, "invalid PostgreSQL DSN scheme")

	_, err = newCredentialProvider(DatabaseConfig{}).buildDSN("postgres://user:pw@h/db/extra", nil)
	assert.ErrorContains(t, err, "one path segment")

	_, err = newCredentialProvider(DatabaseConfig{Username: "user"}).buildDSN("postgres://h/db?user=other", nil)
	assert.ErrorContains(t, err, "not allowed together with database.username")
}
