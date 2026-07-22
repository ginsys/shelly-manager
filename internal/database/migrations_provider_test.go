package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/database/provider"
)

// The pre-migration fixup runs unconditionally on every provider, so the legacy
// schema repair is exercised against real PostgreSQL and MySQL servers too — not
// only SQLite. Both are env-gated in the house style and skip when unavailable;
// CI supplies them via service containers.
//
// Fixtures are dialect-specific on purpose: the SQLite DDL from #275 cannot be
// reused verbatim (AUTOINCREMENT, type names, identifier quoting differ). What
// is preserved is the semantics — no scope column, and a nullable config.

const (
	pgLegacyDDL = `CREATE TABLE config_templates (
		id serial PRIMARY KEY,
		name text NOT NULL,
		description text,
		device_type text,
		generation integer,
		config text,
		variables text,
		is_default boolean,
		created_at timestamptz,
		updated_at timestamptz
	)`

	mysqlLegacyDDL = "CREATE TABLE config_templates (" +
		"id int AUTO_INCREMENT PRIMARY KEY," +
		"name varchar(191) NOT NULL," +
		"description text," +
		"device_type varchar(191)," +
		"generation int," +
		"config text," +
		"variables text," +
		"is_default tinyint(1)," +
		"created_at datetime," +
		"updated_at datetime)"
)

// providerFixture is one live database to run the matrix against.
type providerFixture struct {
	name      string
	config    provider.DatabaseConfig
	legacyDDL string
	// nullable reports whether a column permits NULL, read from the server's
	// own catalog rather than from GORM.
	nullable func(t *testing.T, db *gorm.DB, column string) bool
	// uniqueIndexed reports whether a unique index covers the name column.
	uniqueIndexed func(t *testing.T, db *gorm.DB) bool
}

func TestConfigTemplateScopeMigrationProviders(t *testing.T) {
	// Each provider is set up *inside* its own subtest so that skipping one
	// (server not configured) still runs the other.
	fixtures := []struct {
		name  string
		setup func(t *testing.T) providerFixture
	}{
		{name: "postgresql", setup: setupPostgreSQLFixture},
		{name: "mysql", setup: setupMySQLFixture},
	}

	for _, spec := range fixtures {
		t.Run(spec.name, func(t *testing.T) {
			fixture := spec.setup(t) // skips this provider when unavailable
			t.Run("legacy schema is backfilled and tightened", func(t *testing.T) {
				db := freshProviderDB(t, fixture)
				execAll(t, db, fixture.legacyDDL,
					insertLegacyTemplate("wildcard", "all", `{}`),
					insertLegacyTemplate("concrete", "SHSW-1", `{}`),
					insertLegacyTemplate("cased", "All", `{}`),
				)

				manager := startProviderManager(t, fixture)
				assertProviderMigrated(t, fixture, manager, map[string]string{
					"wildcard": "global",
					"concrete": "device_type",
					"cased":    "device_type",
				})
			})

			t.Run("ambiguous rows abort without writing", func(t *testing.T) {
				db := freshProviderDB(t, fixture)
				execAll(t, db, fixture.legacyDDL,
					insertLegacyTemplate("fine", "all", `{}`),
					insertLegacyTemplate("ambiguous", "", `{}`),
				)

				_, err := newProviderManager(t, fixture)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "ambiguous")

				raw := openProviderDB(t, fixture)
				assert.NotContains(t, providerColumns(t, raw), "scope",
					"aborted migration must not add columns")
			})

			t.Run("partial state completes on the next run", func(t *testing.T) {
				// The shape a MySQL crash leaves behind: DDL committed
				// implicitly, some rows already backfilled, constraint not yet
				// applied. Every provider must recover from it identically.
				db := freshProviderDB(t, fixture)
				execAll(t, db, fixture.legacyDDL,
					insertLegacyTemplate("already-scoped", "SHSW-1", `{}`),
					insertLegacyTemplate("still-empty", "all", `{}`),
					insertLegacyTemplate("explicitly-grouped", "SHPLG-S", `{}`),
				)
				require.NoError(t, db.Exec("ALTER TABLE config_templates ADD COLUMN scope text").Error)
				require.NoError(t, db.Exec(
					"UPDATE config_templates SET scope = 'device_type' WHERE name = 'already-scoped'").Error)
				require.NoError(t, db.Exec(
					"UPDATE config_templates SET scope = 'group' WHERE name = 'explicitly-grouped'").Error)
				require.True(t, fixture.nullable(t, db, "scope"))

				manager := startProviderManager(t, fixture)
				assertProviderMigrated(t, fixture, manager, map[string]string{
					"already-scoped":     "device_type",
					"still-empty":        "global",
					"explicitly-grouped": "group", // an explicit scope is never reinterpreted
				})
			})
		})
	}
}

func insertLegacyTemplate(name, deviceType, config string) string {
	return fmt.Sprintf(
		"INSERT INTO config_templates (name, device_type, config) VALUES ('%s', '%s', '%s')",
		name, deviceType, config)
}

func execAll(t *testing.T, db *gorm.DB, statements ...string) {
	t.Helper()
	for _, stmt := range statements {
		require.NoError(t, db.Exec(stmt).Error, "statement failed: %s", stmt)
	}
}

// assertProviderMigrated is the provider-side twin of assertMigratedSchema.
func assertProviderMigrated(t *testing.T, fixture providerFixture, manager *Manager, wantScopes map[string]string) {
	t.Helper()
	db := manager.GetDB()

	var rows []struct {
		Name  string
		Scope string
	}
	require.NoError(t, db.Raw("SELECT name, scope FROM config_templates ORDER BY id").Scan(&rows).Error)

	got := map[string]string{}
	for _, row := range rows {
		got[row.Name] = row.Scope
	}
	assert.Equal(t, wantScopes, got)

	assert.False(t, fixture.nullable(t, db, "scope"), "scope must be NOT NULL after migration")
	assert.False(t, fixture.nullable(t, db, "config"), "config must be NOT NULL after migration")

	nullable, ok, err := configTemplateColumnNullable(db, "scope")
	require.NoError(t, err)
	assert.True(t, ok, "driver must report nullability")
	assert.False(t, nullable)

	assert.True(t, fixture.uniqueIndexed(t, db), "unique name index missing after migration")

	// Behavioural check: uniqueness must really be enforced, not just indexed.
	inserted := ConfigTemplate{Name: "post-migration-insert", Scope: "global", Config: []byte(`{}`)}
	require.NoError(t, db.Create(&inserted).Error)
	duplicate := ConfigTemplate{Name: "post-migration-insert", Scope: "global", Config: []byte(`{}`)}
	assert.Error(t, db.Create(&duplicate).Error, "duplicate template name must be rejected")
	require.NoError(t, db.Delete(&inserted).Error)
}

// --- fixture plumbing -------------------------------------------------------

func newProviderManager(t *testing.T, fixture providerFixture) (*Manager, error) {
	t.Helper()
	manager, err := NewManagerWithLogger(fixture.config, testLogger(t))
	if manager != nil {
		t.Cleanup(func() { _ = manager.Close() })
	}
	return manager, err
}

func startProviderManager(t *testing.T, fixture providerFixture) *Manager {
	t.Helper()
	manager, err := newProviderManager(t, fixture)
	require.NoError(t, err)
	return manager
}

// openProviderDB opens the fixture database without running any migration.
func openProviderDB(t *testing.T, fixture providerFixture) *gorm.DB {
	t.Helper()

	factory := provider.NewFactory(testLogger(t))
	dbProvider, err := factory.Create(fixture.config.Provider)
	require.NoError(t, err)
	require.NoError(t, dbProvider.Connect(fixture.config))
	t.Cleanup(func() { _ = dbProvider.Close() })

	return dbProvider.GetDB()
}

// freshProviderDB drops any leftover table so each case starts from a known
// state and never depends on test ordering.
func freshProviderDB(t *testing.T, fixture providerFixture) *gorm.DB {
	t.Helper()
	db := openProviderDB(t, fixture)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS config_templates").Error)
	return db
}

func providerColumns(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	types, err := db.Migrator().ColumnTypes(&ConfigTemplate{})
	require.NoError(t, err)

	names := make([]string, 0, len(types))
	for _, columnType := range types {
		names = append(names, strings.ToLower(columnType.Name()))
	}
	return names
}

func setupPostgreSQLFixture(t *testing.T) providerFixture {
	t.Helper()

	host := os.Getenv("POSTGRES_TEST_HOST")
	if host == "" {
		t.Skip("PostgreSQL migration matrix requires POSTGRES_TEST_HOST")
	}

	port := envOrDefault("POSTGRES_TEST_PORT", "5432")
	user := envOrDefault("POSTGRES_TEST_USER", "postgres")
	password := envOrDefault("POSTGRES_TEST_PASSWORD", "postgres")
	database := envOrDefault("POSTGRES_TEST_DB", "shelly_migration_test")

	adminDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable", user, password, host, port)
	requireReachable(t, "pgx", adminDSN)

	// An isolated database per run — never share state with other suites.
	admin, err := sql.Open("pgx", adminDSN)
	require.NoError(t, err)
	defer func() { _ = admin.Close() }()
	_, _ = admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", database))
	_, err = admin.Exec(fmt.Sprintf("CREATE DATABASE %s", database))
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, err := sql.Open("pgx", adminDSN)
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Close() }()
		_, _ = cleanup.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", database))
	})

	return providerFixture{
		name:      "postgresql",
		legacyDDL: pgLegacyDDL,
		config: provider.DatabaseConfig{
			Provider:     "postgresql",
			DSN:          fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, password, host, port, database),
			MaxOpenConns: 5,
			MaxIdleConns: 2,
			LogLevel:     "error",
			Options:      map[string]string{"sslmode": "disable"},
		},
		nullable:      postgresColumnNullable,
		uniqueIndexed: postgresNameUniqueIndexed,
	}
}

func setupMySQLFixture(t *testing.T) providerFixture {
	t.Helper()

	host := os.Getenv("MYSQL_TEST_HOST")
	if host == "" {
		t.Skip("MySQL migration matrix requires MYSQL_TEST_HOST")
	}

	port := envOrDefault("MYSQL_TEST_PORT", "3306")
	user := envOrDefault("MYSQL_TEST_USER", "root")
	password := envOrDefault("MYSQL_TEST_PASSWORD", "root")
	database := envOrDefault("MYSQL_TEST_DB", "shelly_migration_test")

	adminDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/", user, password, host, port)
	requireReachable(t, "mysql", adminDSN)

	admin, err := sql.Open("mysql", adminDSN)
	require.NoError(t, err)
	defer func() { _ = admin.Close() }()
	_, _ = admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", database))
	_, err = admin.Exec(fmt.Sprintf("CREATE DATABASE %s", database))
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, err := sql.Open("mysql", adminDSN)
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Close() }()
		_, _ = cleanup.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", database))
	})

	return providerFixture{
		name:      "mysql",
		legacyDDL: mysqlLegacyDDL,
		config: provider.DatabaseConfig{
			Provider:     "mysql",
			DSN:          fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, password, host, port, database),
			MaxOpenConns: 5,
			MaxIdleConns: 2,
			LogLevel:     "error",
			Options:      map[string]string{"tls": "false"},
		},
		nullable:      mysqlColumnNullable,
		uniqueIndexed: mysqlNameUniqueIndexed,
	}
}

func requireReachable(t *testing.T, driver, dsn string) {
	t.Helper()

	db, err := sql.Open(driver, dsn)
	if err != nil {
		t.Skipf("%s not available: %v", driver, err)
	}
	defer func() { _ = db.Close() }()

	deadline := time.Now().Add(10 * time.Second)
	for {
		if err := db.Ping(); err == nil {
			return
		} else if time.Now().After(deadline) {
			t.Skipf("%s not reachable: %v", driver, err)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// PostgreSQL exposes nullability in information_schema.columns, but indexes live
// in pg_indexes / pg_catalog — it has no information_schema.statistics.
func postgresColumnNullable(t *testing.T, db *gorm.DB, column string) bool {
	t.Helper()
	var isNullable string
	require.NoError(t, db.Raw(
		"SELECT is_nullable FROM information_schema.columns WHERE table_name = 'config_templates' AND column_name = ?",
		column).Scan(&isNullable).Error)
	require.NotEmpty(t, isNullable, "column %q not found", column)
	return isNullable == "YES"
}

func postgresNameUniqueIndexed(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	var count int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*)
		FROM pg_index i
		JOIN pg_class t ON t.oid = i.indrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(i.indkey)
		WHERE t.relname = 'config_templates' AND a.attname = 'name' AND i.indisunique
	`).Scan(&count).Error)
	return count > 0
}

func mysqlColumnNullable(t *testing.T, db *gorm.DB, column string) bool {
	t.Helper()
	var isNullable string
	require.NoError(t, db.Raw(
		"SELECT is_nullable FROM information_schema.columns "+
			"WHERE table_schema = DATABASE() AND table_name = 'config_templates' AND column_name = ?",
		column).Scan(&isNullable).Error)
	require.NotEmpty(t, isNullable, "column %q not found", column)
	return strings.EqualFold(isNullable, "YES")
}

func mysqlNameUniqueIndexed(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	var count int64
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM information_schema.statistics "+
			"WHERE table_schema = DATABASE() AND table_name = 'config_templates' "+
			"AND column_name = 'name' AND non_unique = 0").Scan(&count).Error)
	return count > 0
}
