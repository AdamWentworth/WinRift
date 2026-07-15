package clickhouse

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRefreshChampionSkillAnalyticsUsesSpillableJoinSettings(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("(?s)INSERT INTO champion_skill_analytics.*SETTINGS.*join_algorithm = 'grace_hash'.*max_bytes_before_external_group_by = 268435456.*max_bytes_before_external_sort = 268435456").
		WithArgs("16.12", uint16(420), "16.12", uint16(420)).
		WillReturnResult(sqlmock.NewResult(0, 10))

	repo := &Repository{db: db}
	if err := repo.refreshChampionSkillAnalytics(context.Background(), "16.12", 420); err != nil {
		t.Fatalf("refresh champion skill analytics: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRefreshChampionGuideSummaryAnalyticsUsesSpillableJoinSettings(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("(?s)INSERT INTO champion_guide_summary_analytics.*FROM participant_performance FINAL.*WHERE patch = \\? AND queue_id = \\?.*FROM team_kill_summary FINAL.*WHERE patch = \\? AND queue_id = \\?.*SETTINGS.*join_algorithm = 'grace_hash'.*max_bytes_before_external_group_by = 268435456.*max_bytes_before_external_sort = 268435456").
		WithArgs(
			"16.12", uint16(420),
			"16.12", uint16(420),
			"16.12", uint16(420),
		).
		WillReturnResult(sqlmock.NewResult(0, 10))

	repo := &Repository{db: db}
	if err := repo.refreshChampionGuideSummaryAnalytics(context.Background(), "16.12", 420); err != nil {
		t.Fatalf("refresh champion guide summary analytics: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
