package provider

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openGORM is the only place a provider opens GORM. GORM records every error of
// an open connection (a statement, a transaction, a migration) through one
// function, (*DB).AddError, and when Config.TranslateError is set and the
// dialector implements gorm.ErrorTranslator it passes each error through
// Translate first. So one wrapped dialector seals the sites that record through
// AddError (a statement, a transaction); the logger wrapper below seals GORM's
// own trace line. A query made with Row or Rows stores the driver's error on the
// statement without AddError (the migrator reads columns that way), so the
// providers also seal what Migrate returns.
func openGORM(dialector gorm.Dialector, config *gorm.Config) (*gorm.DB, error) {
	config.TranslateError = true
	db, err := gorm.Open(sealedDialector{Dialector: dialector}, config)
	if db == nil {
		return db, err
	}
	// Rows, Scan and the migrator's column reads run the Row callbacks, which
	// store the driver's error on the statement without calling AddError. Record
	// it again after "gorm:row", through AddError, so it is translated and sealed
	// like every other.
	if regErr := db.Callback().Row().After("gorm:row").Register("shelly:seal_row_error", sealRowError); regErr != nil {
		return nil, regErr
	}
	return db, err
}

// sealRowError records a Row callback's error again through AddError.
func sealRowError(db *gorm.DB) {
	if db.Error == nil {
		return
	}
	err := db.Error
	db.Error = nil
	_ = db.AddError(err)
}

// sealedDialector wraps a GORM dialector and seals the errors it translates.
//
// GORM finds three optional behaviours of a dialector by type assertion, and an
// embedded interface hides every one that is not part of gorm.Dialector: Apply
// (the PostgreSQL identifier length, the MySQL clock rounding), SavePoint and
// RollbackTo (nested transactions). The wrapper therefore always has all three
// and delegates each to the inner dialector when that has it.
//
// Migrator is not wrapped on purpose: the PostgreSQL migrator asserts its own
// dialector type, which holds only while Migrator() stays the inner one's.
type sealedDialector struct {
	gorm.Dialector
}

// Apply delegates to the inner dialector's configuration hook.
func (d sealedDialector) Apply(config *gorm.Config) error {
	if applier, ok := d.Dialector.(interface{ Apply(*gorm.Config) error }); ok {
		return applier.Apply(config)
	}
	return nil
}

// SavePoint delegates to the inner dialector.
func (d sealedDialector) SavePoint(tx *gorm.DB, name string) error {
	if inner, ok := d.Dialector.(gorm.SavePointerDialectorInterface); ok {
		return inner.SavePoint(tx, name)
	}
	return gorm.ErrUnsupportedDriver
}

// RollbackTo delegates to the inner dialector.
func (d sealedDialector) RollbackTo(tx *gorm.DB, name string) error {
	if inner, ok := d.Dialector.(gorm.SavePointerDialectorInterface); ok {
		return inner.RollbackTo(tx, name)
	}
	return gorm.ErrUnsupportedDriver
}

// Translate lets the inner dialector map the constraint errors GORM's callers
// test for (gorm.ErrDuplicatedKey, gorm.ErrForeignKeyViolated), then seals
// whatever is left.
func (d sealedDialector) Translate(err error) error {
	if translator, ok := d.Dialector.(gorm.ErrorTranslator); ok {
		err = translator.Translate(err)
	}
	return sealOpenError(err)
}

// sealedLogger seals the error GORM's trace line prints. Translate covers the
// errors GORM records through AddError, but a query made with Row or Rows
// stores the driver's error on the statement directly, and the trace line then
// quotes it (the socket path, the address, the failing identifiers).
type sealedLogger struct {
	logger.Interface
}

// sealLogger wraps a GORM logger so that no trace line carries a driver error.
func sealLogger(inner logger.Interface) logger.Interface {
	return sealedLogger{Interface: inner}
}

// LogMode keeps the wrapper when GORM derives a logger at another level.
func (l sealedLogger) LogMode(level logger.LogLevel) logger.Interface {
	return sealedLogger{Interface: l.Interface.LogMode(level)}
}

// ParamsFilter delegates to the wrapped logger. GORM finds it by type assertion
// on its logger, and the embedded interface hides it: without this method the
// bound values (a device login) come back into the SQL trace
// (ginsys/shelly-manager#489).
func (l sealedLogger) ParamsFilter(ctx context.Context, sql string, params ...interface{}) (string, []interface{}) {
	if filter, ok := l.Interface.(gorm.ParamsFilter); ok {
		return filter.ParamsFilter(ctx, sql, params...)
	}
	return sql, params
}

// Trace seals err before the wrapped logger prints it.
func (l sealedLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.Interface.Trace(ctx, begin, fc, sealOpenError(err))
}

// passThrough lists the errors of fixed text that carry nothing of the
// configuration or the data and that callers test by identity.
var passThrough = []error{
	gorm.ErrRecordNotFound, gorm.ErrInvalidTransaction, gorm.ErrNotImplemented,
	gorm.ErrMissingWhereClause, gorm.ErrUnsupportedRelation, gorm.ErrPrimaryKeyRequired,
	gorm.ErrModelValueRequired, gorm.ErrModelAccessibleFieldsRequired, gorm.ErrSubQueryRequired,
	gorm.ErrInvalidData, gorm.ErrUnsupportedDriver, gorm.ErrRegistered, gorm.ErrInvalidField,
	gorm.ErrEmptySlice, gorm.ErrDryRunModeUnsupported, gorm.ErrInvalidDB, gorm.ErrInvalidValue,
	gorm.ErrInvalidValueOfLength, gorm.ErrPreloadNotAllowed, gorm.ErrDuplicatedKey,
	gorm.ErrForeignKeyViolated, gorm.ErrCheckConstraintViolated,
}

// passThroughExact are sentinels of the standard library that GORM and
// database/sql callers compare by equality.
var passThroughExact = []error{
	context.Canceled, context.DeadlineExceeded, driver.ErrBadConn,
	sql.ErrNoRows, sql.ErrTxDone, sql.ErrConnDone,
}

// SealOperationError is sealOpenError for the code that uses the pool directly,
// outside GORM (sql.DB.Conn, QueryRowContext), where no dialector sees the error.
func SealOperationError(err error) error { return sealOpenError(err) }

// sealOpenError returns err unchanged when it is nil, an already sealed
// failure or one of the fixed sentinels above, and its classification
// otherwise. It seals by default: an error type nobody listed is reported as
// unclassified, never printed.
func sealOpenError(err error) error {
	if err == nil {
		return nil
	}
	var failure *connectFailure
	if errors.As(err, &failure) {
		return err
	}
	// The dialector's Translate joins the sentinel it chose with the driver's own
	// error ("duplicated key not allowed: ...: <driver text> (SQLSTATE 23505)"),
	// so the sentinel is returned bare: errors.Is still answers, and the driver
	// text is dropped.
	for _, sentinel := range passThrough {
		if errors.Is(err, sentinel) {
			return sentinel
		}
	}
	for _, sentinel := range passThroughExact {
		if err == sentinel {
			return err
		}
	}
	return classifyOperationError(err)
}
