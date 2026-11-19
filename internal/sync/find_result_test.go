package sync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

func findResultEngine(t *testing.T) (*SyncEngine, *database.Manager) {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	return NewSyncEngine(db, logger), db
}

// The result the engine holds in memory wins over the history record, because it holds more.
func TestFindExportResult_MemoryWinsOverHistory(t *testing.T) {
	engine, db := findResultEngine(t)
	require.NoError(t, db.GetDB().Create(&database.ExportHistory{
		ExportID: "exp-1", PluginName: "from-history", Format: "txt", Success: true,
	}).Error)
	engine.exportResults["exp-1"] = &ExportResult{ExportID: "exp-1", PluginName: "from-memory", Checksum: "sha256:abc"}

	res, err := engine.FindExportResult(context.Background(), "exp-1")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, "from-memory", res.PluginName)
	require.Equal(t, "sha256:abc", res.Checksum)
}

// With nothing in memory the record is described field by field, and what it lacks stays empty.
func TestFindExportResult_DescribesTheHistoryRecord(t *testing.T) {
	engine, db := findResultEngine(t)
	createdAt := time.Date(2026, 11, 1, 9, 30, 0, 0, time.UTC)
	require.NoError(t, db.GetDB().Create(&database.ExportHistory{
		ExportID: "exp-2", PluginName: "sma", Format: "sma", Success: false, RecordCount: 7, FileSize: 99,
		FilePath: "/data/exp-2.sma", DurationMs: 1234, ErrorMessage: "disk full", CreatedAt: createdAt,
	}).Error)

	res, err := engine.FindExportResult(context.Background(), "exp-2")
	require.NoError(t, err)
	require.Equal(t, &ExportResult{
		Success: false, ExportID: "exp-2", PluginName: "sma", Format: "sma", OutputPath: "/data/exp-2.sma",
		RecordCount: 7, FileSize: 99, Duration: 1234 * time.Millisecond, Errors: []string{"disk full"},
		CreatedAt: createdAt,
	}, res)
}

// A record with no error message has no error list, not a list with an empty entry.
func TestFindExportResult_NoErrorMessageMeansNoErrors(t *testing.T) {
	engine, db := findResultEngine(t)
	require.NoError(t, db.GetDB().Create(&database.ExportHistory{ExportID: "exp-3", Success: true}).Error)

	res, err := engine.FindExportResult(context.Background(), "exp-3")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Nil(t, res.Errors)
}

func TestFindExportResult_UnknownIDIsNilWithoutError(t *testing.T) {
	engine, _ := findResultEngine(t)

	res, err := engine.FindExportResult(context.Background(), "nope")
	require.NoError(t, err)
	require.Nil(t, res)
}

// A database that cannot be read is an error, never a not-found.
func TestFindExportResult_ADatabaseErrorIsReturned(t *testing.T) {
	engine, db := findResultEngine(t)
	require.NoError(t, db.GetDB().Migrator().DropTable(&database.ExportHistory{}))

	res, err := engine.FindExportResult(context.Background(), "exp-4")
	require.Error(t, err)
	require.Nil(t, res)
}

func TestFindImportResult_MemoryWinsOverHistory(t *testing.T) {
	engine, db := findResultEngine(t)
	require.NoError(t, db.GetDB().Create(&database.ImportHistory{
		ImportID: "imp-1", PluginName: "from-history", Format: "txt", Success: true,
	}).Error)
	engine.importResults["imp-1"] = &ImportResult{
		ImportID: "imp-1", PluginName: "from-memory", Changes: []ImportChange{{Type: "create", Resource: "device"}},
	}

	res, err := engine.FindImportResult(context.Background(), "imp-1")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, "from-memory", res.PluginName)
	require.Len(t, res.Changes, 1)
}

func TestFindImportResult_DescribesTheHistoryRecord(t *testing.T) {
	engine, db := findResultEngine(t)
	createdAt := time.Date(2026, 11, 1, 9, 30, 0, 0, time.UTC)
	require.NoError(t, db.GetDB().Create(&database.ImportHistory{
		ImportID: "imp-2", PluginName: "sma", Format: "sma", Success: false, RecordsImported: 4, RecordsSkipped: 2,
		DurationMs: 250, ErrorMessage: "template exists", CreatedAt: createdAt,
	}).Error)

	res, err := engine.FindImportResult(context.Background(), "imp-2")
	require.NoError(t, err)
	require.Equal(t, &ImportResult{
		Success: false, ImportID: "imp-2", PluginName: "sma", Format: "sma", RecordsImported: 4, RecordsSkipped: 2,
		Duration: 250 * time.Millisecond, Errors: []string{"template exists"}, CreatedAt: createdAt,
	}, res)
}

func TestFindImportResult_UnknownIDIsNilWithoutError(t *testing.T) {
	engine, _ := findResultEngine(t)

	res, err := engine.FindImportResult(context.Background(), "nope")
	require.NoError(t, err)
	require.Nil(t, res)
}

func TestFindImportResult_ADatabaseErrorIsReturned(t *testing.T) {
	engine, db := findResultEngine(t)
	require.NoError(t, db.GetDB().Migrator().DropTable(&database.ImportHistory{}))

	res, err := engine.FindImportResult(context.Background(), "imp-3")
	require.Error(t, err)
	require.Nil(t, res)
}
