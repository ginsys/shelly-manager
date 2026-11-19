package provider

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	gormmysql "gorm.io/driver/mysql"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// The providers apply the pool they are given and hold no pool default of their own: the defaults live in
// the config helper, and the factory refuses an unset open or idle count before a provider is built
// (ginsys/shelly-manager#581).

// noConnector is a database/sql connector that never connects, so no server is needed.
type noConnector struct{}

func (noConnector) Connect(context.Context) (driver.Conn, error) { return nil, errors.New("no server") }
func (noConnector) Driver() driver.Driver                        { return nil }

func stubGormDB(t *testing.T, open func(*sql.DB) gorm.Dialector) (*gorm.DB, *sql.DB) {
	t.Helper()
	sqlDB := sql.OpenDB(noConnector{})
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(open(sqlDB), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("gorm.Open over a stub connection: %v", err)
	}
	return db, sqlDB
}

func TestProvidersApplyThePoolAsGiven(t *testing.T) {
	logger := logging.GetDefault()
	mysqlDB := func(c *sql.DB) gorm.Dialector {
		return gormmysql.New(gormmysql.Config{Conn: c, SkipInitializeWithVersion: true})
	}
	postgresDB := func(c *sql.DB) gorm.Dialector {
		return gormpostgres.New(gormpostgres.Config{Conn: c})
	}
	apply := map[string]func(t *testing.T, cfg DatabaseConfig) *sql.DB{
		"mysql": func(t *testing.T, cfg DatabaseConfig) *sql.DB {
			p := NewMySQLProvider(logger)
			db, sqlDB := stubGormDB(t, mysqlDB)
			p.db, p.config = db, cfg
			if err := p.configureConnectionPool(); err != nil {
				t.Fatal(err)
			}
			return sqlDB
		},
		"postgresql": func(t *testing.T, cfg DatabaseConfig) *sql.DB {
			p := NewPostgreSQLProvider(logger)
			db, sqlDB := stubGormDB(t, postgresDB)
			p.db, p.config = db, cfg
			if err := p.configureConnectionPool(); err != nil {
				t.Fatal(err)
			}
			return sqlDB
		},
	}

	for name, run := range apply {
		t.Run(name+" a zero pool stays zero", func(t *testing.T) {
			sqlDB := run(t, DatabaseConfig{})
			if got := sqlDB.Stats().MaxOpenConnections; got != 0 {
				t.Errorf("MaxOpenConnections = %d, want 0 (the provider holds no default)", got)
			}
		})
		t.Run(name+" a set pool is applied", func(t *testing.T) {
			sqlDB := run(t, DatabaseConfig{MaxOpenConns: 7, MaxIdleConns: 3, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: time.Second})
			if got := sqlDB.Stats().MaxOpenConnections; got != 7 {
				t.Errorf("MaxOpenConnections = %d, want 7", got)
			}
		})
	}
}
