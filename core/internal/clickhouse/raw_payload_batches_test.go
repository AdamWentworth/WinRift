package clickhouse

import (
	"context"
	"regexp"
	"strconv"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestForEachMissingRawPayloadMatchBatchChunksCandidateIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	missingRows := sqlmock.NewRows([]string{"match_id"})
	for i := 0; i < rawPayloadBackfillBatchSize+1; i++ {
		missingRows.AddRow("NA1_" + leftPadNumber(i, 4))
	}
	mock.ExpectQuery("(?s)SELECT match_id.*FROM raw_matches FINAL.*match_id NOT IN.*FROM participant_performance FINAL.*ORDER BY match_id").
		WithArgs("16.12", "NA1", uint16(420), "16.12", "NA1", uint16(420)).
		WillReturnRows(missingRows)

	var batchSizes []int
	repo := &Repository{db: db}
	err = repo.forEachMissingRawPayloadMatchBatch(context.Background(), "raw_matches", "participant_performance", "16.12", "NA1", 420, func(matchIDs []string) error {
		batchSizes = append(batchSizes, len(matchIDs))
		return nil
	})
	if err != nil {
		t.Fatalf("iterate batches: %v", err)
	}
	if len(batchSizes) != 2 || batchSizes[0] != rawPayloadBackfillBatchSize || batchSizes[1] != 1 {
		t.Fatalf("batch sizes = %v, want [%d 1]", batchSizes, rawPayloadBackfillBatchSize)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestForEachMissingRawPayloadMatchBatchRejectsUnknownTables(t *testing.T) {
	repo := &Repository{}
	err := repo.forEachMissingRawPayloadMatchBatch(context.Background(), "raw_secrets", "participant_performance", "16.12", "NA1", 420, func([]string) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected unsupported source table error")
	}
	err = repo.forEachMissingRawPayloadMatchBatch(context.Background(), "raw_matches", "private_secrets", "16.12", "NA1", 420, func([]string) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected unsupported destination table error")
	}
}

func TestBackfillParticipantPerformanceUsesBoundedRawMatchBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("(?s)SELECT DISTINCT platform FROM raw_matches.*ORDER BY platform").
		WithArgs("16.12", uint16(420)).
		WillReturnRows(sqlmock.NewRows([]string{"platform"}).AddRow("NA1"))
	mock.ExpectQuery("(?s)SELECT match_id.*FROM raw_matches FINAL.*match_id NOT IN.*FROM participant_performance FINAL.*ORDER BY match_id").
		WithArgs("16.12", "NA1", uint16(420), "16.12", "NA1", uint16(420)).
		WillReturnRows(sqlmock.NewRows([]string{"match_id"}).AddRow("NA1_1").AddRow("NA1_2"))
	mock.ExpectExec("(?s)INSERT INTO participant_performance.*FROM participants FINAL.*match_id IN \\(\\?, \\?\\)").
		WithArgs("16.12", "NA1", uint16(420), "NA1_1", "NA1_2").
		WillReturnResult(sqlmock.NewResult(0, 20))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count() FROM participant_performance FINAL WHERE patch = ? AND queue_id = ?")).
		WithArgs("16.12", uint16(420)).
		WillReturnRows(sqlmock.NewRows([]string{"count()"}).AddRow(20))

	repo := &Repository{db: db}
	result, err := repo.BackfillParticipantPerformance(context.Background(), "16.12", 420)
	if err != nil {
		t.Fatalf("backfill performance: %v", err)
	}
	if result.Rows != 20 {
		t.Fatalf("rows = %d, want 20", result.Rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestChampionGuideRawPayloadBackfillsUseBoundedFinalBatches(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("(?s)INSERT INTO timeline_skill_events.*FROM raw_timelines AS rt FINAL.*match_id IN \\(\\?, \\?\\)").
		WithArgs("16.12", "NA1", uint16(420), "NA1_1", "NA1_2").
		WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec("(?s)INSERT INTO champion_bans.*FROM raw_matches AS rm FINAL.*match_id IN \\(\\?, \\?\\)").
		WithArgs("16.12", "NA1", uint16(420), "NA1_1", "NA1_2").
		WillReturnResult(sqlmock.NewResult(0, 20))

	repo := &Repository{db: db}
	matchIDs := []string{"NA1_1", "NA1_2"}
	if err := repo.backfillChampionGuideSkillEventsBatch(context.Background(), "16.12", "NA1", 420, matchIDs); err != nil {
		t.Fatalf("backfill skill events: %v", err)
	}
	if err := repo.backfillChampionBansBatch(context.Background(), "16.12", "NA1", 420, matchIDs); err != nil {
		t.Fatalf("backfill bans: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestBackfillChampionGuideEventsOnlyProcessesMissingMatches(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("(?s)SELECT DISTINCT platform FROM raw_matches.*ORDER BY platform").
		WithArgs("16.12", uint16(420)).
		WillReturnRows(sqlmock.NewRows([]string{"platform"}).AddRow("NA1"))
	mock.ExpectQuery("(?s)SELECT match_id.*FROM raw_timelines FINAL.*match_id NOT IN.*FROM timeline_skill_events FINAL.*ORDER BY match_id").
		WithArgs("16.12", "NA1", uint16(420), "16.12", "NA1", uint16(420)).
		WillReturnRows(sqlmock.NewRows([]string{"match_id"}))
	mock.ExpectQuery("(?s)SELECT match_id.*FROM raw_matches FINAL.*match_id NOT IN.*FROM champion_bans FINAL.*ORDER BY match_id").
		WithArgs("16.12", "NA1", uint16(420), "16.12", "NA1", uint16(420)).
		WillReturnRows(sqlmock.NewRows([]string{"match_id"}))
	mock.ExpectQuery("(?s)SELECT.*SELECT count\\(\\) FROM raw_matches.*SELECT count\\(\\) FROM timeline_skill_events.*SELECT count\\(\\) FROM champion_bans").
		WithArgs("16.12", uint16(420), "16.12", uint16(420), "16.12", uint16(420)).
		WillReturnRows(sqlmock.NewRows([]string{"matches", "skill_events", "bans"}).AddRow(100, 1500, 900))

	repo := &Repository{db: db}
	result, err := repo.BackfillChampionGuideEvents(context.Background(), "16.12", 420)
	if err != nil {
		t.Fatalf("backfill champion guide events: %v", err)
	}
	if result.Matches != 100 || result.SkillEvents != 1500 || result.Bans != 900 {
		t.Fatalf("result = %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func leftPadNumber(value int, width int) string {
	digits := "0000000000" + strconv.Itoa(value)
	return digits[len(digits)-width:]
}
