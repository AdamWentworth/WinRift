package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRefreshItemSlotAnalyticsBatchesPlatformsBeforeAllAggregation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("(?s)SELECT DISTINCT platform FROM raw_matches.*ORDER BY platform").
		WithArgs("16.12", uint16(420)).
		WillReturnRows(sqlmock.NewRows([]string{"platform"}).AddRow("EUW1"))
	mock.ExpectExec("(?s)ALTER TABLE item_slot_analytics DELETE.*startsWith\\(platform, '__ROLLOVER_'\\)").
		WillReturnResult(sqlmock.NewResult(0, 1))
	matchRows := sqlmock.NewRows([]string{"match_id"})
	for i := 0; i < itemAnalyticsMatchBatchSize+1; i++ {
		matchRows.AddRow(fmt.Sprintf("EUW1_%04d", i))
	}
	mock.ExpectQuery("(?s)SELECT DISTINCT match_id.*FROM raw_matches.*PREWHERE patch = \\? AND queue_id = \\?.*WHERE platform = \\?.*ORDER BY match_id").
		WithArgs("16.12", uint16(420), "EUW1").
		WillReturnRows(matchRows)
	batchInsert := "(?s)INSERT INTO item_slot_analytics.*WITH target_match_ids AS.*SELECT arrayJoin\\(\\[.*pm.match_id IN \\(SELECT match_id FROM target_match_ids\\).*tie.match_id IN \\(SELECT match_id FROM target_match_ids\\).*SETTINGS.*join_algorithm = 'grace_hash'.*max_bytes_before_external_group_by = 268435456"
	mock.ExpectExec(batchInsert).WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec(batchInsert).WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec("(?s)INSERT INTO item_slot_analytics.*\\? AS platform.*FROM item_slot_analytics.*platform IN \\(\\?, \\?\\)").
		WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec("(?s)ALTER TABLE item_slot_analytics DELETE.*compiled_at = \\?.*platform IN \\(\\?, \\?\\)").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("(?s)INSERT INTO item_slot_analytics.*'ALL' AS platform.*FROM item_slot_analytics.*platform IN \\(\\?\\)").
		WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec("ALTER TABLE item_slot_analytics DELETE WHERE").
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &Repository{db: db}
	err = repo.RefreshItemSlotAnalytics(context.Background(), "16.12", 420, []ItemSlotAnalyticsContext{{
		Key:             "standard",
		ItemIDs:         []uint32{1001, 2001},
		StartingItemIDs: []uint32{1001},
	}})
	if err != nil {
		t.Fatalf("refresh item slots: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRetryAnalyticsMemoryPressureStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempts := 0
	err := retryAnalyticsMemoryPressureWithDelay(ctx, "test batch", time.Hour, func() error {
		attempts++
		return errors.New("code: 241, memory limit exceeded by OvercommitTracker")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRetryAnalyticsMemoryPressureDoesNotHidePermanentErrors(t *testing.T) {
	want := errors.New("unknown table")
	attempts := 0
	err := retryAnalyticsMemoryPressureWithDelay(context.Background(), "test batch", 0, func() error {
		attempts++
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRetryAnalyticsMemoryPressureRetriesTransientFailure(t *testing.T) {
	attempts := 0
	err := retryAnalyticsMemoryPressureWithDelay(context.Background(), "test batch", 0, func() error {
		attempts++
		if attempts == 1 {
			return errors.New("code: 241, memory limit exceeded by OvercommitTracker")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry transient failure: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestRetryAnalyticsMemoryPressureRecognizesNestedMutationFailure(t *testing.T) {
	attempts := 0
	err := retryAnalyticsMemoryPressureWithDelay(context.Background(), "cleanup batch", 0, func() error {
		attempts++
		if attempts == 1 {
			return errors.New("code: 341, mutation failed reason: Code: 241. memory limit exceeded")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry nested mutation failure: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestRefreshStartingLoadoutAnalyticsBatchesPlatformsBeforeAllAggregation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("(?s)SELECT DISTINCT platform FROM raw_matches.*ORDER BY platform").
		WithArgs("16.12", uint16(420)).
		WillReturnRows(sqlmock.NewRows([]string{"platform"}).AddRow("EUW1"))
	mock.ExpectExec("(?s)ALTER TABLE starting_loadout_analytics DELETE.*startsWith\\(platform, '__ROLLOVER_'\\)").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)SELECT DISTINCT match_id.*FROM raw_matches.*PREWHERE patch = \\? AND queue_id = \\?.*WHERE platform = \\?.*ORDER BY match_id").
		WithArgs("16.12", uint16(420), "EUW1").
		WillReturnRows(sqlmock.NewRows([]string{"match_id"}).AddRow("EUW1_0001"))
	batchInsert := "(?s)INSERT INTO starting_loadout_analytics.*WITH target_match_ids AS.*SELECT arrayJoin\\(\\[.*pm.match_id IN \\(SELECT match_id FROM target_match_ids\\).*tie.match_id IN \\(SELECT match_id FROM target_match_ids\\).*SETTINGS.*join_algorithm = 'grace_hash'.*max_bytes_before_external_group_by = 268435456"
	mock.ExpectExec(batchInsert).WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec("(?s)INSERT INTO starting_loadout_analytics.*\\? AS platform.*FROM starting_loadout_analytics.*platform IN \\(\\?\\)").
		WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec("(?s)ALTER TABLE starting_loadout_analytics DELETE.*compiled_at = \\?.*platform IN \\(\\?\\)").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("(?s)INSERT INTO starting_loadout_analytics.*'ALL' AS platform.*FROM starting_loadout_analytics.*platform IN \\(\\?\\)").
		WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec("ALTER TABLE starting_loadout_analytics DELETE WHERE").
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &Repository{db: db}
	err = repo.RefreshStartingLoadoutAnalytics(context.Background(), "16.12", 420, []StartingLoadoutAnalyticsContext{{
		Key:              "standard",
		OpeningItemCosts: map[uint32]uint32{1001: 300, 2003: 50},
	}})
	if err != nil {
		t.Fatalf("refresh starting loadouts: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
