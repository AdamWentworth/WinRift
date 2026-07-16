package clickhouse

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"winrift/core/internal/analytics"
)

func TestPatchMaintenanceStepRetriesMemoryPressureInPlace(t *testing.T) {
	attempts := 0
	err := retryPatchMaintenanceStepWithDelay(context.Background(), "compile BR1", 0, func() error {
		attempts++
		if attempts == 1 {
			return errors.New("code: 241, memory limit exceeded by OvercommitTracker")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry patch maintenance step: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestPatchCompileAndPruneKeepMemoryRetriesInsideSteps(t *testing.T) {
	sourceBytes, err := os.ReadFile("patch_lifecycle.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	source := string(sourceBytes)
	compileStart := strings.Index(source, "func (r *Repository) CompilePatchMetrics")
	if compileStart < 0 {
		t.Fatal("could not locate patch compile implementation")
	}
	compileEnd := strings.Index(source[compileStart:], "func (r *Repository) DeleteRawPatchData")
	if compileEnd < 0 {
		t.Fatal("could not locate patch compile implementation")
	}
	compileImplementation := source[compileStart : compileStart+compileEnd]
	if count := strings.Count(compileImplementation, "retryPatchMaintenanceStep"); count != 8 {
		t.Fatalf("compile retry steps = %d, want 8", count)
	}
	if !strings.Contains(source, `retryPatchMaintenanceStep(ctx, "raw prune table="+table`) ||
		!strings.Contains(source, `retryPatchMaintenanceStep(ctx, label`) {
		t.Fatal("raw prune mutations are not protected by in-place memory retries")
	}
}

func TestDeleteRawPatchDataForPatchDropsRawPartitionsWhenAvailable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	repo := &Repository{db: db}
	queueID := uint16(analytics.RankedSoloQueueID)
	expectPatchPlatforms(mock, "16.10", queueID, "BR1", "NA1")
	expectPatchClosed(mock, "16.10", "BR1", queueID)
	expectPatchClosed(mock, "16.10", "NA1", queueID)

	for _, table := range []string{"raw_timelines", "raw_matches"} {
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT partition_key FROM system.tables WHERE database = currentDatabase() AND name = ?`)).
			WithArgs(table).
			WillReturnRows(sqlmock.NewRows([]string{"partition_key"}).AddRow("tuple(patch, queue_id)"))
		mock.ExpectExec(regexp.QuoteMeta("ALTER TABLE " + table + " DROP PARTITION tuple('16.10', 420) SETTINGS mutations_sync = 2")).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}

	for _, table := range []string{
		"timeline_participant_frames",
		"timeline_item_events",
		"timeline_skill_events",
		"timeline_combat_events",
		"timeline_objective_events",
		"champion_bans",
	} {
		mock.ExpectExec(regexp.QuoteMeta("ALTER TABLE "+table+" DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2")).
			WithArgs("16.10", queueID).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}

	if err := repo.DeleteRawPatchDataForPatch(context.Background(), "16.10", queueID); err != nil {
		t.Fatalf("delete raw patch data: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestDeleteRawTablePatchDataFallsBackWhenRawTableIsNotPartitioned(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	repo := &Repository{db: db}
	queueID := uint16(analytics.RankedSoloQueueID)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT partition_key FROM system.tables WHERE database = currentDatabase() AND name = ?`)).
		WithArgs("raw_timelines").
		WillReturnRows(sqlmock.NewRows([]string{"partition_key"}).AddRow(""))
	mock.ExpectExec(regexp.QuoteMeta("ALTER TABLE raw_timelines DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2")).
		WithArgs("16.10", queueID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.deleteRawTablePatchData(context.Background(), "raw_timelines", "16.10", queueID); err != nil {
		t.Fatalf("delete raw table patch data: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestPatchQueuePartitionLiteralRejectsUnsafePatchValues(t *testing.T) {
	if _, err := patchQueuePartitionLiteral("16.10'); DROP TABLE raw_matches; --", analytics.RankedSoloQueueID); err == nil {
		t.Fatal("expected unsafe patch value to be rejected")
	}
}

func expectPatchPlatforms(mock sqlmock.Sqlmock, patch string, queueID uint16, platforms ...string) {
	rows := sqlmock.NewRows([]string{"platform"})
	for _, platform := range platforms {
		rows.AddRow(platform)
	}
	mock.ExpectQuery("(?s)SELECT DISTINCT platform.*FROM.*raw_matches.*patch_snapshots").
		WithArgs(
			patch, queueID,
			patch, queueID,
			patch, queueID,
			patch, queueID,
			patch, queueID,
			patch, queueID,
		).
		WillReturnRows(rows)
}

func expectPatchClosed(mock sqlmock.Sqlmock, patch, platform string, queueID uint16) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT status FROM patch_snapshots FINAL WHERE patch = ? AND platform = ? AND queue_id = ? LIMIT 1`)).
		WithArgs(patch, platform, queueID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("closed"))
}
