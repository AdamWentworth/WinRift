package clickhouse

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestQueryChampionGuideIndexUsesSummaryReadModelsWhenPresent(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM champion_guide_scope_analytics FINAL").
		WithArgs(sqlmock.AnyArg(), "TOP", "ALL", "16.10").
		WillReturnRows(sqlmock.NewRows([]string{"participant_samples", "match_count"}).AddRow(1000, 100))

	mock.ExpectQuery("(?s)FROM champion_guide_summary_analytics FINAL").
		WithArgs(sqlmock.AnyArg(), "TOP", "16.10", 50).
		WillReturnRows(sqlmock.NewRows(championGuideSummaryColumns()).
			AddRow(266, 55, 100, 0.55, 600, 300, 700, 12000, 190, 24000, 28000, 18000, 6000, 2500, 22, 18, 900, 1.2, 0.3, 230, 1800, 0.62))

	mock.ExpectQuery("(?s)FROM champion_ban_analytics FINAL").
		WithArgs("16.10").
		WillReturnRows(sqlmock.NewRows([]string{"champion_id", "bans", "games"}).AddRow(266, 20, 1000))

	index, err := repo.QueryChampionGuideIndex(context.Background(), map[string]string{
		"role":        "TOP",
		"patch":       "16.10",
		"rank_bucket": "ALL",
	}, 50, 10)
	if err != nil {
		t.Fatalf("query champion guide index: %v", err)
	}
	if index.MatchCount != 100 || index.ParticipantSamples != 1000 {
		t.Fatalf("scope counts = matches %d participants %d; want 100 and 1000", index.MatchCount, index.ParticipantSamples)
	}
	if len(index.Results) != 1 {
		t.Fatalf("results = %d; want 1", len(index.Results))
	}
	row := index.Results[0]
	if row.ChampionID != 266 || row.Role != "TOP" || row.PatchBucket != "16.10" || row.RankBucket != "ALL" {
		t.Fatalf("summary row scope = %+v", row)
	}
	if row.PickRate != 0.1 {
		t.Fatalf("pick rate = %.4f; want 0.1 from summary participant samples", row.PickRate)
	}
	if row.Bans != 20 || row.BanRate != 0.02 {
		t.Fatalf("ban row = bans %d rate %.4f; want 20 and 0.02", row.Bans, row.BanRate)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPatchStatsIncludesCompiledSnapshotsAfterRawPrune(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM raw_matches FINAL.*FROM participants FINAL.*FROM patch_snapshots FINAL").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"patch", "matches", "participant_samples", "raw_matches", "compiled_matches"}).
			AddRow("16.11", 42, 420, 42, 0).
			AddRow("16.9", 500, 5000, 0, 500))

	stats, err := repo.PatchStats(context.Background(), 0)
	if err != nil {
		t.Fatalf("patch stats: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("stats = %d; want 2", len(stats))
	}
	oldPatch := stats[1]
	if oldPatch.Patch != "16.9" || oldPatch.RawMatches != 0 || oldPatch.CompiledMatches != 500 || oldPatch.Matches != 500 {
		t.Fatalf("old patch stats = %+v; want compiled 16.9 data with raw pruned", oldPatch)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func newMockRepository(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	return db, mock, func() {
		_ = db.Close()
	}
}

func championGuideSummaryColumns() []string {
	return []string{
		"champion_id",
		"wins",
		"games",
		"win_rate",
		"total_kills",
		"total_deaths",
		"total_assists",
		"avg_gold_earned",
		"avg_cs",
		"avg_damage_dealt_to_champions",
		"avg_damage_taken",
		"avg_damage_self_mitigated",
		"avg_damage_dealt_to_objectives",
		"avg_damage_dealt_to_structures",
		"avg_vision_score",
		"avg_time_ccing_others",
		"avg_team_utility",
		"avg_structure_takedowns",
		"avg_objective_takedowns",
		"avg_total_time_spent_dead",
		"avg_time_played",
		"kill_participation",
	}
}
