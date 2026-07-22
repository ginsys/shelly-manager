package database

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// Pre-migration fixups.
//
// GORM's AutoMigrate cannot add a NOT NULL column to a populated table: SQLite
// refuses the ALTER outright and PostgreSQL rejects the NULLs it would create.
// Every fixup below therefore reshapes a legacy schema into one AutoMigrate can
// handle, and each must be idempotent — a fully migrated database runs them as a
// no-op.
//
// Atomicity is bounded, and deliberately so:
//
//   - preflight failure: nothing is written at all;
//   - data-phase failure: SQLite/PostgreSQL roll the transaction back, while
//     MySQL commits DDL implicitly and may keep a nullable column — recoverable
//     on the next run;
//   - constraint-phase failure: each ALTER is applied on its own (SQLite rebuilds
//     the table inside its own transaction; on PostgreSQL each DDL statement is
//     atomic), so an earlier constraint may already be committed;
//   - a later AutoMigrate failure leaves a completed fixup in place.
//
// In every case the next startup must be able to finish the job.
//
// Note: three structs currently map onto config_templates (see issue #280); this
// file deliberately repairs the table rather than reconciling the models.

const configTemplatesTable = "config_templates"

// runPreMigrationFixups makes a legacy schema safe for the AutoMigrate that
// follows. It runs before any model is migrated, on every provider.
func runPreMigrationFixups(db *gorm.DB, logger *logging.Logger) error {
	if db == nil {
		return fmt.Errorf("pre-migration fixups: nil database handle")
	}
	if logger == nil {
		logger = logging.GetDefault()
	}

	_, err := fixupConfigTemplateScope(db, logger)
	return err
}

// configTemplateFixupResult records what the fixup actually did. Schema actions
// and row counts are kept separate so a partially completed run stays legible —
// and so tests can assert which half of the work happened.
type configTemplateFixupResult struct {
	ScopeColumnAdded          bool
	ConfigColumnAdded         bool
	RowsBackfilledGlobal      int
	RowsBackfilledDeviceType  int
	ScopeConstraintTightened  bool
	ConfigConstraintTightened bool
}

func (r configTemplateFixupResult) changedAnything() bool {
	return r.ScopeColumnAdded || r.ConfigColumnAdded ||
		r.RowsBackfilledGlobal > 0 || r.RowsBackfilledDeviceType > 0 ||
		r.ScopeConstraintTightened || r.ConfigConstraintTightened
}

// configTemplateRow is the projection the preflight reads. Columns absent from a
// legacy schema are projected as NULL so one scan shape serves every dialect.
type configTemplateRow struct {
	ID           uint
	Name         *string
	Scope        *string
	DeviceType   *string
	ConfigIsNull bool
}

func (r configTemplateRow) scope() string      { return derefString(r.Scope) }
func (r configTemplateRow) deviceType() string { return derefString(r.DeviceType) }

func (r configTemplateRow) label() string {
	if name := derefString(r.Name); name != "" {
		return fmt.Sprintf("id=%d name=%q", r.ID, name)
	}
	return fmt.Sprintf("id=%d", r.ID)
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// configTemplateOffence is one row the migration refuses to interpret.
type configTemplateOffence struct {
	row    configTemplateRow
	reason string
}

// configTemplatePreflightError reports every unresolvable row at once. Operators
// get the full list and a remedy rather than one row per restart.
type configTemplatePreflightError struct {
	offences []configTemplateOffence
}

func (e *configTemplatePreflightError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "config_templates cannot be migrated: %d template(s) need manual resolution before startup", len(e.offences))
	for _, o := range e.offences {
		fmt.Fprintf(&b, "\n  - %s: %s", o.row.label(), o.reason)
	}
	b.WriteString("\n\nResolve each row, then restart. A template's scope must be 'global', 'group' or " +
		"'device_type'; a 'device_type' template needs a concrete device_type, and the legacy wildcard " +
		"is exactly 'all'. Set device_type to 'all' for a fleet-wide template or to the device type it " +
		"was written for, give any NULL config a value, or delete the row. " +
		"See docs/guides/database-upgrade.md.")
	return b.String()
}

// fixupConfigTemplateScope repairs databases created before config_templates
// gained its scope column (issue #275).
//
// The backfill preserves or narrows a template's applicability and never widens
// it: 'all' was the only wildcard the legacy matcher understood (exact and
// case-sensitive, see configuration.Service.ApplyTemplate), so an empty
// device_type matched nothing and must not silently become a global template.
func fixupConfigTemplateScope(db *gorm.DB, logger *logging.Logger) (configTemplateFixupResult, error) {
	var result configTemplateFixupResult

	migrator := db.Migrator()
	if !migrator.HasTable(configTemplatesTable) {
		// Fresh database: CREATE TABLE carries the constraints itself.
		return result, nil
	}

	hasScope := migrator.HasColumn(&ConfigTemplate{}, "scope")
	hasDeviceType := migrator.HasColumn(&ConfigTemplate{}, "device_type")
	hasConfig := migrator.HasColumn(&ConfigTemplate{}, "config")

	rows, err := loadConfigTemplateRows(db, hasScope, hasDeviceType, hasConfig)
	if err != nil {
		return result, err
	}

	needGlobal, needDeviceType, err := preflightConfigTemplates(rows, hasDeviceType, hasConfig)
	if err != nil {
		return result, err
	}

	result.ScopeColumnAdded = !hasScope
	result.ConfigColumnAdded = !hasConfig && len(rows) == 0
	result.RowsBackfilledGlobal = len(needGlobal)
	result.RowsBackfilledDeviceType = len(needDeviceType)

	if backfillErr := backfillConfigTemplateScopes(db, backfillPlan{
		addScopeColumn:  result.ScopeColumnAdded,
		addConfigColumn: result.ConfigColumnAdded,
		hasDeviceType:   hasDeviceType,
		globalIDs:       needGlobal,
		deviceIDs:       needDeviceType,
	}); backfillErr != nil {
		return result, backfillErr
	}

	// Constraint phase — outside any transaction of ours. GORM's SQLite
	// AlterColumn rebuilds the table and toggles PRAGMA foreign_keys around its
	// own transaction; nesting that inside ours would silently drop the PRAGMA.
	result.ScopeConstraintTightened, err = tightenConfigTemplateColumn(db, "scope")
	if err != nil {
		return result, err
	}
	result.ConfigConstraintTightened, err = tightenConfigTemplateColumn(db, "config")
	if err != nil {
		return result, err
	}

	if result.changedAnything() {
		logger.WithFields(map[string]any{
			"scope_column_added":          result.ScopeColumnAdded,
			"config_column_added":         result.ConfigColumnAdded,
			"rows_backfilled_global":      result.RowsBackfilledGlobal,
			"rows_backfilled_device_type": result.RowsBackfilledDeviceType,
			"scope_constraint_tightened":  result.ScopeConstraintTightened,
			"config_constraint_tightened": result.ConfigConstraintTightened,
			"table":                       configTemplatesTable,
			"component":                   "database",
		}).Info("Repaired legacy config_templates schema")
	}

	return result, nil
}

// loadConfigTemplateRows reads the table with a projection built from the
// columns that actually exist. Selecting a missing column would fail before any
// row could be classified.
func loadConfigTemplateRows(db *gorm.DB, hasScope, hasDeviceType, hasConfig bool) ([]configTemplateRow, error) {
	scopeExpr := "NULL"
	if hasScope {
		scopeExpr = "scope"
	}
	deviceTypeExpr := "NULL"
	if hasDeviceType {
		deviceTypeExpr = "device_type"
	}
	// Test the column for NULL in SQL rather than reading the payload back.
	configExpr := "0"
	if hasConfig {
		configExpr = "CASE WHEN config IS NULL THEN 1 ELSE 0 END"
	}

	query := fmt.Sprintf(
		"SELECT id, name, %s AS scope, %s AS device_type, %s AS config_is_null FROM %s ORDER BY id",
		scopeExpr, deviceTypeExpr, configExpr, configTemplatesTable,
	)

	var rows []configTemplateRow
	if err := db.Raw(query).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to read %s for migration: %w", configTemplatesTable, err)
	}
	return rows, nil
}

// preflightConfigTemplates classifies every row without writing anything and
// returns the IDs each backfill statement is expected to touch.
func preflightConfigTemplates(rows []configTemplateRow, hasDeviceType, hasConfig bool) (needGlobal, needDeviceType []uint, err error) {
	var offences []configTemplateOffence

	if !hasConfig && len(rows) > 0 {
		// The column is required by the model and its content is not derivable.
		for _, row := range rows {
			offences = append(offences, configTemplateOffence{
				row:    row,
				reason: "the config column is missing from this database and cannot be reconstructed",
			})
		}
		return nil, nil, &configTemplatePreflightError{offences: offences}
	}

	for _, row := range rows {
		if row.ConfigIsNull {
			// Repairing scope alone would just move the failure one step later:
			// the model requires config NOT NULL too. Never coerced to "{}" —
			// fabricating a template body is worse than refusing to guess.
			offences = append(offences, configTemplateOffence{
				row:    row,
				reason: "config is NULL, but the schema requires a value",
			})
			continue
		}

		if scope := row.scope(); scope != "" {
			// An explicit scope is authoritative; it is validated, never reinterpreted.
			if verr := configuration.ValidateTemplateScope(scope, row.deviceType()); verr != nil {
				offences = append(offences, configTemplateOffence{
					row:    row,
					reason: fmt.Sprintf("scope %q is invalid: %v", scope, verr),
				})
			}
			continue
		}

		if !hasDeviceType {
			offences = append(offences, configTemplateOffence{
				row:    row,
				reason: "scope is missing and this database has no device_type column to derive it from",
			})
			continue
		}

		switch deviceType := row.deviceType(); {
		case deviceType == "all":
			needGlobal = append(needGlobal, row.ID)
		case deviceType != "":
			needDeviceType = append(needDeviceType, row.ID)
		default:
			// Empty matched no device under the legacy matcher. Mapping it to
			// 'global' would promote an unusable template to a fleet-wide one.
			offences = append(offences, configTemplateOffence{
				row:    row,
				reason: "scope is missing and device_type is empty, so the intended scope is ambiguous",
			})
		}
	}

	if len(offences) > 0 {
		sort.SliceStable(offences, func(i, j int) bool { return offences[i].row.ID < offences[j].row.ID })
		return nil, nil, &configTemplatePreflightError{offences: offences}
	}

	return needGlobal, needDeviceType, nil
}

type backfillPlan struct {
	addScopeColumn  bool
	addConfigColumn bool
	hasDeviceType   bool
	globalIDs       []uint
	deviceIDs       []uint
}

// applyScope writes one scope to exactly the rows the preflight selected, in
// chunks so the statement stays within parameter limits. A statement that
// touches a different number of rows than planned means the table changed under
// us, and the transaction is abandoned rather than half-applied.
func applyScope(tx *gorm.DB, scope string, ids []uint) error {
	const chunkSize = 500

	for start := 0; start < len(ids); start += chunkSize {
		end := start + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]

		result := tx.Exec(
			fmt.Sprintf("UPDATE %s SET scope = ? WHERE id IN ?", configTemplatesTable),
			scope, chunk)
		if result.Error != nil {
			return fmt.Errorf("failed to backfill %q scopes: %w", scope, result.Error)
		}
		if result.RowsAffected != int64(len(chunk)) {
			return fmt.Errorf("backfill of %q scopes touched %d row(s), expected %d",
				scope, result.RowsAffected, len(chunk))
		}
	}
	return nil
}

// backfillConfigTemplateScopes performs the data phase in one transaction.
// Columns are added nullable and without a default: that is the only ADD COLUMN
// form portable across SQLite, PostgreSQL and MySQL, and a permanent default
// would silently accept scope-less inserts forever.
func backfillConfigTemplateScopes(db *gorm.DB, plan backfillPlan) error {
	if !plan.addScopeColumn && !plan.addConfigColumn && len(plan.globalIDs) == 0 && len(plan.deviceIDs) == 0 {
		return nil // already migrated
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if plan.addScopeColumn {
			if err := tx.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN scope text", configTemplatesTable)).Error; err != nil {
				return fmt.Errorf("failed to add %s.scope: %w", configTemplatesTable, err)
			}
		}
		if plan.addConfigColumn {
			// Only reachable on an empty table; a populated one already aborted.
			if err := tx.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN config text", configTemplatesTable)).Error; err != nil {
				return fmt.Errorf("failed to add %s.config: %w", configTemplatesTable, err)
			}
		}

		// A missing device_type column means an empty table (the preflight
		// aborts otherwise), so there is nothing to backfill. AutoMigrate adds
		// the column afterwards — it is nullable, so that ALTER is safe on
		// every provider.
		if !plan.hasDeviceType {
			return nil
		}

		// Rows are addressed by the ids the preflight classified, not by
		// re-matching device_type in SQL. The wildcard test has to be exactly
		// 'all', and string comparison is collation-dependent: under MySQL's
		// default case-insensitive collation a device_type of 'All' matches
		// 'all', which would silently promote a single-device-type template to
		// a fleet-wide one. Comparing in Go keeps one authoritative rule.
		if err := applyScope(tx, configuration.ScopeGlobal, plan.globalIDs); err != nil {
			return err
		}
		if err := applyScope(tx, configuration.ScopeDeviceType, plan.deviceIDs); err != nil {
			return err
		}

		var remaining int64
		if err := tx.Raw(fmt.Sprintf(
			"SELECT COUNT(*) FROM %s WHERE scope IS NULL OR scope = ''", configTemplatesTable)).
			Scan(&remaining).Error; err != nil {
			return fmt.Errorf("failed to verify backfilled scopes: %w", err)
		}
		if remaining != 0 {
			return fmt.Errorf("config_templates still has %d row(s) without a scope after backfill", remaining)
		}

		return nil
	})
}

// tightenConfigTemplateColumn enforces NOT NULL on one column, and only when it
// is actually nullable — an unconditional AlterColumn would rebuild the whole
// SQLite table on every single startup. Reports whether it altered anything.
func tightenConfigTemplateColumn(db *gorm.DB, column string) (bool, error) {
	nullable, ok, err := configTemplateColumnNullable(db, column)
	if err != nil {
		return false, err
	}
	if !ok {
		// An unknown result is not a pass: proceeding would let AutoMigrate hit
		// the very ALTER this fixup exists to prevent.
		return false, fmt.Errorf("cannot determine whether %s.%s is nullable; refusing to migrate", configTemplatesTable, column)
	}
	if !nullable {
		return false, nil
	}

	if alterErr := db.Migrator().AlterColumn(&ConfigTemplate{}, column); alterErr != nil {
		return false, fmt.Errorf("failed to enforce NOT NULL on %s.%s: %w", configTemplatesTable, column, alterErr)
	}

	nullable, ok, err = configTemplateColumnNullable(db, column)
	if err != nil {
		return true, err
	}
	if !ok || nullable {
		return true, fmt.Errorf("%s.%s is still nullable after ALTER; this provider needs an explicit table rebuild",
			configTemplatesTable, column)
	}
	return true, nil
}

// configTemplateColumnNullable reports a column's nullability and whether the
// driver could answer at all.
func configTemplateColumnNullable(db *gorm.DB, column string) (nullable, ok bool, err error) {
	types, err := db.Migrator().ColumnTypes(&ConfigTemplate{})
	if err != nil {
		return false, false, fmt.Errorf("failed to inspect %s columns: %w", configTemplatesTable, err)
	}
	for _, columnType := range types {
		if !strings.EqualFold(columnType.Name(), column) {
			continue
		}
		nullable, ok = columnType.Nullable()
		return nullable, ok, nil
	}
	return false, false, fmt.Errorf("column %s.%s not found", configTemplatesTable, column)
}
