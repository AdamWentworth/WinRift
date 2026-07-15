package clickhouse

import (
	"context"
	"testing"

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
		WillReturnRows(sqlmock.NewRows([]string{"platform"}).AddRow("EUW1").AddRow("NA1"))
	platformInsert := "(?s)INSERT INTO item_slot_analytics.*participant_matchups AS pm FINAL.*pm.platform = \\?.*tie.platform = \\?.*SETTINGS join_algorithm = 'grace_hash'"
	mock.ExpectExec(platformInsert).WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec(platformInsert).WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec("(?s)INSERT INTO item_slot_analytics.*'ALL' AS platform.*FROM item_slot_analytics.*platform IN \\(\\?, \\?\\)").
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

func TestRefreshStartingLoadoutAnalyticsBatchesPlatformsBeforeAllAggregation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("(?s)SELECT DISTINCT platform FROM raw_matches.*ORDER BY platform").
		WithArgs("16.12", uint16(420)).
		WillReturnRows(sqlmock.NewRows([]string{"platform"}).AddRow("EUW1").AddRow("NA1"))
	platformInsert := "(?s)INSERT INTO starting_loadout_analytics.*participant_matchups AS pm FINAL.*pm.platform = \\?.*tie.platform = \\?.*SETTINGS join_algorithm = 'grace_hash'"
	mock.ExpectExec(platformInsert).WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec(platformInsert).WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec("(?s)INSERT INTO starting_loadout_analytics.*'ALL' AS platform.*FROM starting_loadout_analytics.*platform IN \\(\\?, \\?\\)").
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
