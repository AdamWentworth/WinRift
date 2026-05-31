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

func TestQueryChampionGuideSummaryUsesSummaryReadModelWhenPresent(t *testing.T) {
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

	summary, err := repo.queryChampionGuideSummary(context.Background(), map[string]string{
		"champion_id": "266",
		"role":        "TOP",
		"patch":       "16.10",
		"rank_bucket": "ALL",
	}, 5)
	if err != nil {
		t.Fatalf("query champion guide summary: %v", err)
	}
	if summary.ChampionID != 266 || summary.Role != "TOP" || summary.Games != 100 || summary.Wins != 55 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.RoleRank != 1 || summary.RoleRankTotal != 1 {
		t.Fatalf("summary rank = %d/%d; want 1/1", summary.RoleRank, summary.RoleRankTotal)
	}
	if summary.KillParticipation != 0.62 {
		t.Fatalf("kill participation = %.2f; want 0.62 from summary read model", summary.KillParticipation)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRefreshTeamKillSummaryUsesParticipants(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectExec("(?s)INSERT INTO team_kill_summary.*FROM participants FINAL").
		WithArgs("16.11", uint16(420)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.refreshTeamKillSummary(context.Background(), "16.11", 420); err != nil {
		t.Fatalf("refresh team kill summary: %v", err)
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

func TestSummonerRecentMatchesUsesSummaryReadModel(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM summoner_recent_match_summary FINAL").
		WithArgs("NA1", "puuid-1", uint16(420), 20).
		WillReturnRows(sqlmock.NewRows([]string{
			"match_id",
			"platform",
			"patch",
			"queue_id",
			"champion_id",
			"role",
			"win",
			"kills",
			"deaths",
			"assists",
			"game_start_timestamp",
			"duration_seconds",
		}).AddRow("NA1_1", "NA1", "16.11", 420, 266, "TOP", 1, 7, 2, 9, 1770000000000, 1830))

	rows, err := repo.SummonerRecentMatches(context.Background(), "NA1", "puuid-1", 420, 20)
	if err != nil {
		t.Fatalf("summoner recent matches: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d; want 1", len(rows))
	}
	if rows[0].MatchID != "NA1_1" || rows[0].ChampionID != 266 || !rows[0].Win {
		t.Fatalf("recent row = %+v", rows[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSummonerBuildsUsesSummaryReadModel(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM summoner_build_summary FINAL").
		WithArgs("NA1", "puuid-1", uint16(420), 12).
		WillReturnRows(sqlmock.NewRows([]string{
			"champion_id",
			"role",
			"final_items_signature",
			"core2_signature",
			"core3_signature",
			"rune_signature",
			"spell_signature",
			"games",
			"wins",
			"kills",
			"deaths",
			"assists",
		}).AddRow(266, "TOP", "3071-3053-6333", "3071-3053", "3071-3053-6333", "rune", "4-14", 11, 7, 60, 33, 80))

	rows, err := repo.SummonerBuilds(context.Background(), "NA1", "puuid-1", 420, 12)
	if err != nil {
		t.Fatalf("summoner builds: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d; want 1", len(rows))
	}
	row := rows[0]
	if row.ChampionID != 266 || row.Games != 11 || row.Wins != 7 || row.Losses != 4 {
		t.Fatalf("build row = %+v", row)
	}
	if row.WinRate <= 0.63 || row.WinRate >= 0.64 {
		t.Fatalf("win rate = %.4f; want around 0.636", row.WinRate)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestChampionGuideMatchupsUseSummaryReadModel(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM champion_matchup_analytics FINAL").
		WithArgs(sqlmock.AnyArg(), "266", "TOP", "16.11", 5, 12).
		WillReturnRows(sqlmock.NewRows([]string{"opponent_champion_id", "wins", "games", "win_rate"}).
			AddRow(122, 4, 5, 0.8))

	rows, err := repo.queryChampionGuideMatchups(context.Background(), map[string]string{
		"champion_id": "266",
		"role":        "TOP",
		"patch":       "16.11",
		"rank_bucket": "ALL",
	}, 5, 12, false)
	if err != nil {
		t.Fatalf("champion guide matchups: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d; want 1", len(rows))
	}
	if rows[0].OpponentChampionID != 122 || rows[0].Wins != 4 || rows[0].Games != 5 {
		t.Fatalf("matchup row = %+v", rows[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestChampionGuideSignaturesUseSummaryReadModel(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM champion_signature_analytics FINAL").
		WithArgs(sqlmock.AnyArg(), "266", "rune", "TOP", "16.11", 5, 48).
		WillReturnRows(sqlmock.NewRows([]string{"signature", "wins", "games", "win_rate"}).
			AddRow("8010|8000|8400|5005|5008|5011", 8, 10, 0.8))

	rows, err := repo.queryChampionGuideSignatures(context.Background(), map[string]string{
		"champion_id": "266",
		"role":        "TOP",
		"patch":       "16.11",
		"rank_bucket": "ALL",
	}, "rune_signature", 5, 12)
	if err != nil {
		t.Fatalf("champion guide signatures: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d; want 1", len(rows))
	}
	if rows[0].Signature != "8010|8000|8400|5005|5008|5011" || rows[0].Wins != 8 || rows[0].Games != 10 {
		t.Fatalf("signature row = %+v", rows[0])
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
