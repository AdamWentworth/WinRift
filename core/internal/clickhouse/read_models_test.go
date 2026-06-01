package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

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

func TestReplacePatchReadModelRowsInsertsBeforeDeletingOldRows(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}
	cutoff := time.Date(2026, 5, 31, 14, 0, 0, 0, time.UTC)

	mock.ExpectExec("INSERT INTO champion_role_analytics").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("ALTER TABLE champion_role_analytics DELETE WHERE patch = \\? AND queue_id = \\? AND compiled_at < \\? SETTINGS mutations_sync = 2").
		WithArgs("16.10", uint16(420), cutoff).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.replacePatchReadModelRows(context.Background(), "champion_role_analytics", "16.10", 420, cutoff, func() error {
		_, err := db.Exec("INSERT INTO champion_role_analytics")
		return err
	})
	if err != nil {
		t.Fatalf("replace read model rows: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestReplacePatchReadModelRowsKeepsOldRowsWhenRefreshFails(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}
	refreshErr := errors.New("refresh failed")

	err := repo.replacePatchReadModelRows(context.Background(), "champion_role_analytics", "16.10", 420, time.Now(), func() error {
		return refreshErr
	})
	if !errors.Is(err, refreshErr) {
		t.Fatalf("replace read model rows error = %v; want %v", err, refreshErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRefreshChampionRoleAnalyticsUsesParticipants(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectExec("(?s)INSERT INTO champion_role_analytics.*FROM participants FINAL").
		WithArgs("16.11", uint16(420)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.refreshChampionRoleAnalytics(context.Background(), "16.11", 420); err != nil {
		t.Fatalf("refresh champion role analytics: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestChampionRoleRatesUseSummaryReadModelWhenPresent(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM champion_role_analytics FINAL").
		WithArgs(uint16(420), "16.10").
		WillReturnRows(sqlmock.NewRows([]string{"champion_id", "role", "games", "total_games", "pick_rate"}).
			AddRow(266, "TOP", 70, 100, 0.7).
			AddRow(266, "MIDDLE", 30, 100, 0.3))

	rows, err := repo.ChampionRoleRatesForPatch(context.Background(), []uint16{266}, 420, "16.10")
	if err != nil {
		t.Fatalf("champion role rates: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d; want 2", len(rows))
	}
	if rows[0].ChampionID != 266 || rows[0].Role != "TOP" || rows[0].Games != 70 || rows[0].TotalGames != 100 || rows[0].PickRate != 0.7 {
		t.Fatalf("first role row = %+v", rows[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestChampionRoleRatesFallbackUsesPatchScopedParticipants(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM champion_role_analytics FINAL").
		WithArgs(uint16(420), "16.10").
		WillReturnRows(sqlmock.NewRows([]string{"champion_id", "role", "games", "total_games", "pick_rate"}))
	mock.ExpectQuery("(?s)FROM participants FINAL.*AND patch = \\?").
		WithArgs(uint16(420), "16.10", uint16(420), "16.10").
		WillReturnRows(sqlmock.NewRows([]string{"champion_id", "role", "games", "total_games", "pick_rate"}).
			AddRow(266, "TOP", 5, 5, 1.0))

	rows, err := repo.ChampionRoleRatesForPatch(context.Background(), []uint16{266}, 420, "16.10")
	if err != nil {
		t.Fatalf("champion role rates: %v", err)
	}
	if len(rows) != 1 || rows[0].Role != "TOP" || rows[0].Games != 5 {
		t.Fatalf("fallback role rows = %+v", rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRefreshBuildSignatureAnalyticsUsesParticipantSummaries(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectExec("(?s)INSERT INTO build_signature_analytics.*FROM participant_matchups AS pm FINAL").
		WithArgs("16.11", uint16(420)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.refreshBuildSignatureAnalytics(context.Background(), "16.11", 420); err != nil {
		t.Fatalf("refresh build signature analytics: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestQueryBuildsUsesBuildSignatureReadModel(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM build_signature_analytics WHERE").
		WithArgs("16.11").
		WillReturnRows(sqlmock.NewRows([]string{"count()"}).AddRow(1))
	mock.ExpectQuery("(?s)FROM \\(.*build_signature_analytics FINAL.*patch_build_metrics AS pbm FINAL").
		WithArgs("266", "16.11", 10, 40).
		WillReturnRows(sqlmock.NewRows([]string{
			"champion_id",
			"role_bucket",
			"opponent_champion_id",
			"patch_bucket",
			"rank_bucket",
			"final_items_signature",
			"core2_signature",
			"core3_signature",
			"rune_signature",
			"spell_signature",
			"wins",
			"games",
			"win_rate",
		}).AddRow(266, "TOP", 0, "16.11", "DIAMOND", "3071-3053-6333", "3071-3053", "3071-3053-6333", "rune", "4-14", 7, 10, 0.7))

	rows, err := repo.QueryBuilds(context.Background(), map[string]string{
		"champion_id": "266",
		"role":        "TOP",
		"patch":       "16.11",
	}, 10, 8)
	if err != nil {
		t.Fatalf("query builds: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d; want 1", len(rows))
	}
	if rows[0].ChampionID != 266 || rows[0].FinalItemsSignature != "3071-3053-6333" || rows[0].Games != 10 {
		t.Fatalf("build row = %+v", rows[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestQueryItemSlotsUsesSummaryReadModel(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM item_slot_analytics FINAL").
		WithArgs("DEFAULT", "266", "122", "16.10", 5).
		WillReturnRows(sqlmock.NewRows([]string{
			"champion_id",
			"role_bucket",
			"opponent_champion_id",
			"patch_bucket",
			"rank_bucket",
			"item_slot",
			"item_id",
			"wins",
			"games",
			"win_rate",
		}).AddRow(266, "TOP", 122, "16.10", "ALL", 2, 3071, 8, 12, 0.6667))

	rows, err := repo.QueryItemSlots(context.Background(), map[string]string{
		"champion_id":          "266",
		"role":                 "TOP",
		"opponent_champion_id": "122",
		"patch":                "16.10",
	}, "DEFAULT", []uint32{3071, 3053}, nil, 5, 4)
	if err != nil {
		t.Fatalf("query item slots: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d; want 1", len(rows))
	}
	row := rows[0]
	if row.ChampionID != 266 || row.OpponentChampionID != 122 || row.ItemSlot != 2 || row.ItemID != 3071 || row.Games != 12 {
		t.Fatalf("item slot row = %+v; want summary-backed item slot", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestQueryStartingItemLoadoutsUsesSummaryReadModel(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM starting_loadout_analytics FINAL").
		WithArgs("DEFAULT", "266", "16.10", 5).
		WillReturnRows(sqlmock.NewRows([]string{
			"champion_id",
			"role_bucket",
			"opponent_champion_id",
			"patch_bucket",
			"rank_bucket",
			"item_signature",
			"wins",
			"games",
			"win_rate",
		}).AddRow(266, "TOP", 0, "16.10", "ALL", "1055-2003", 10, 18, 0.5556))

	rows, err := repo.QueryStartingItemLoadouts(context.Background(), map[string]string{
		"champion_id": "266",
		"role":        "TOP",
		"patch":       "16.10",
	}, "DEFAULT", map[uint32]uint32{1055: 450, 2003: 50}, 5, 4)
	if err != nil {
		t.Fatalf("query starting loadouts: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d; want 1", len(rows))
	}
	row := rows[0]
	if row.ChampionID != 266 || row.ItemSignature != "1055-2003" || row.Games != 18 {
		t.Fatalf("starting loadout row = %+v; want summary-backed loadout", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestChampionGuideItemPathsUseBuildSignatureReadModel(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM build_signature_analytics WHERE").
		WithArgs("16.11").
		WillReturnRows(sqlmock.NewRows([]string{"count()"}).AddRow(1))
	mock.ExpectQuery("(?s)FROM \\(.*build_signature_analytics FINAL.*patch_build_metrics AS pbm FINAL").
		WithArgs("266", "TOP", "16.11", 5, 60).
		WillReturnRows(sqlmock.NewRows([]string{"core3_signature", "final_items_signature", "wins", "games", "win_rate"}).
			AddRow("3071-3053-6333", "3071-3053-6333-3065", 8, 12, 0.6667))

	rows, err := repo.queryChampionGuideItemPaths(context.Background(), map[string]string{
		"champion_id": "266",
		"role":        "TOP",
		"patch":       "16.11",
	}, 5, 12)
	if err != nil {
		t.Fatalf("query champion guide item paths: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d; want 1", len(rows))
	}
	if rows[0].Core3Signature != "3071-3053-6333" || rows[0].Games != 12 {
		t.Fatalf("item path row = %+v", rows[0])
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

func TestCachedChampionPageBundleUsesPersistentCache(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM champion_page_bundle_cache FINAL").
		WithArgs("champion-page:test").
		WillReturnRows(sqlmock.NewRows([]string{"payload_json"}).AddRow(`{"ok":true}`))

	body, ok, err := repo.CachedChampionPageBundle(context.Background(), "champion-page:test")
	if err != nil {
		t.Fatalf("cached champion page bundle: %v", err)
	}
	if !ok || string(body) != `{"ok":true}` {
		t.Fatalf("cached bundle ok=%t body=%s; want cached payload", ok, string(body))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSummonerRankLeaderboardUsesIdentityAndProfileSummaries(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("(?s)FROM summoner_rank_snapshots FINAL.*FROM riot_account_aliases FINAL.*FROM summoner_account_snapshots FINAL.*FROM summoner_identity_summary FINAL.*FROM summoner_profile_summary FINAL").
		WithArgs("NA1", rankedSoloQueueType, "NA1", "NA1", "NA1", "NA1", uint16(420), 25).
		WillReturnRows(sqlmock.NewRows([]string{
			"puuid",
			"platform",
			"game_name",
			"tag_line",
			"tier",
			"division",
			"league_points",
			"wins",
			"losses",
			"rank_bucket",
			"latest_fetched_at",
			"expires_at",
			"latest_seen_at",
			"profile_icon_id",
			"summoner_level",
			"stored_games",
			"stored_wins",
		}).AddRow("puuid-1", "NA1", "Example", "NA1", "MASTER", "I", 88, 120, 90, "MASTER+", now, now.Add(time.Hour), now, 456, 321, 42, 24))

	rows, err := repo.SummonerRankLeaderboard(context.Background(), "NA1", 25)
	if err != nil {
		t.Fatalf("summoner rank leaderboard: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d; want 1", len(rows))
	}
	row := rows[0]
	if row.GameName != "Example" || row.ProfileIconID != 456 || row.StoredGames != 42 || row.StoredWins != 24 {
		t.Fatalf("leaderboard row = %+v; want identity/profile summary data", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSummonerProfileStatsUsesSummaryReadModel(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}
	firstSeen := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	lastSeen := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("(?s)FROM summoner_profile_summary FINAL").
		WithArgs("NA1", "puuid-1", uint16(420)).
		WillReturnRows(sqlmock.NewRows([]string{"games", "wins", "kills", "deaths", "assists", "first_seen_at", "last_seen_at"}).
			AddRow(20, 12, 140, 60, 180, firstSeen, lastSeen))

	stats, err := repo.SummonerProfileStats(context.Background(), "NA1", "puuid-1", 420)
	if err != nil {
		t.Fatalf("summoner profile stats: %v", err)
	}
	if stats.Games != 20 || stats.Wins != 12 || stats.Losses != 8 || stats.KDA <= 5.3 {
		t.Fatalf("profile stats = %+v; want summary-backed stats", stats)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSummonerTopChampionsUsesSummaryReadModel(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM summoner_champion_summary FINAL").
		WithArgs("NA1", "puuid-1", uint16(420), 5).
		WillReturnRows(sqlmock.NewRows([]string{"champion_id", "games", "wins", "kills", "deaths", "assists"}).
			AddRow(266, 20, 12, 140, 60, 180))

	rows, err := repo.SummonerTopChampions(context.Background(), "NA1", "puuid-1", 420, 5)
	if err != nil {
		t.Fatalf("summoner top champions: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d; want 1", len(rows))
	}
	row := rows[0]
	if row.ChampionID != 266 || row.Games != 20 || row.Wins != 12 || row.KDA <= 5.3 {
		t.Fatalf("top champion row = %+v; want summary-backed champion performance", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSummonerTopChampionRolesUsesSummaryReadModel(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM summoner_champion_role_summary FINAL").
		WithArgs("NA1", "puuid-1", uint16(420), 30).
		WillReturnRows(sqlmock.NewRows([]string{"champion_id", "role", "games", "wins", "kills", "deaths", "assists"}).
			AddRow(266, "TOP", 18, 11, 120, 55, 170))

	rows, err := repo.SummonerTopChampionRoles(context.Background(), "NA1", "puuid-1", 420, 30)
	if err != nil {
		t.Fatalf("summoner top champion roles: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d; want 1", len(rows))
	}
	row := rows[0]
	if row.ChampionID != 266 || row.Role != "TOP" || row.Games != 18 || row.Wins != 11 {
		t.Fatalf("top champion role row = %+v; want role summary-backed champion performance", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestQueryWinConditionMetricsUsesPatchSummaryReadModel(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM patch_win_condition_metrics FINAL").
		WithArgs(uint16(420), "ALL", "ALL", "16.10").
		WillReturnRows(sqlmock.NewRows([]string{
			"patch",
			"platform",
			"queue_id",
			"rank_bucket",
			"team_condition",
			"team_rating",
			"opponent_condition",
			"opponent_rating",
			"team_primary",
			"game_length_bucket",
			"wins",
			"games",
			"win_rate_percent",
			"confidence_percent",
		}).AddRow("16.10", "ALL", 420, "ALL", "SplitPush", "B", "Control", "B+", 1, "25-30", 32, 58, 55.17, 61.2))

	rows, err := repo.QueryWinConditionMetrics(context.Background(), WinConditionMetricFilters{
		Patch:      "16.10",
		Platform:   "ALL",
		QueueID:    420,
		RankBucket: "ALL",
	})
	if err != nil {
		t.Fatalf("query win condition metrics: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d; want 1", len(rows))
	}
	row := rows[0]
	if row.TeamCondition != "SplitPush" || row.OpponentCondition != "Control" || row.Games != 58 || row.Confidence != 61.2 {
		t.Fatalf("win condition metric = %+v; want patch summary metric", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestQueryWinConditionValidationBuildsCorpusReport(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)SELECT.*uniqExact\\(match_id\\).*FROM match_team_win_conditions FINAL").
		WillReturnRows(sqlmock.NewRows([]string{"resolved_patch", "teams", "matches"}).AddRow("16.10", 200, 100))
	mock.ExpectQuery("(?s)SELECT.*axis.*rating.*avg_score.*UNION ALL").
		WillReturnRows(sqlmock.NewRows([]string{"axis", "rating", "games", "wins", "avg_score", "min_score", "max_score"}).
			AddRow("Pick", "B", 120, 66, 14.2, 13, 16).
			AddRow("TeamFight", "C", 80, 36, 9.5, 8, 11))
	mock.ExpectQuery("(?s)arrayJoin.*delta_bucket").
		WillReturnRows(sqlmock.NewRows([]string{"axis", "delta_bucket", "games", "wins", "avg_delta"}).
			AddRow("Pick", "1..3", 70, 40, 2.1).
			AddRow("Pick", "-3..-1", 65, 28, -2.0))
	mock.ExpectQuery("(?s)primary_condition.*opponent_primary_condition").
		WillReturnRows(sqlmock.NewRows([]string{"primary_condition", "primary_rating", "opponent_primary_condition", "opponent_primary_rating", "games", "wins"}).
			AddRow("Pick", "B", "TeamFight", "C", 300, 150).
			AddRow("TeamFight", "A", "Pick", "B", 100, 70))
	mock.ExpectQuery("(?s)primary_margin.*margin_bucket").
		WillReturnRows(sqlmock.NewRows([]string{"margin_bucket", "games", "wins", "avg_margin"}).
			AddRow("2-3", 90, 48, 2.4))
	mock.ExpectQuery("(?s)t\\.champion_ids AS team_champion_ids").
		WillReturnRows(sqlmock.NewRows([]string{
			"primary_condition",
			"primary_rating",
			"opponent_primary_condition",
			"opponent_primary_rating",
			"win",
			"team_champion_ids",
			"opponent_champion_ids",
		}))
	mock.ExpectQuery("(?s)FROM patch_win_condition_metrics FINAL").
		WillReturnRows(sqlmock.NewRows([]string{
			"patch",
			"platform",
			"queue_id",
			"rank_bucket",
			"team_condition",
			"team_rating",
			"opponent_condition",
			"opponent_rating",
			"team_primary",
			"game_length_bucket",
			"wins",
			"games",
			"win_rate_percent",
			"confidence_percent",
		}).AddRow("16.10", "ALL", 420, "ALL", "SplitPush", "C", "Control", "B", 21, "ALL", 35, 60, 58.33, 45.12))

	report, err := repo.QueryWinConditionValidation(context.Background(), WinConditionValidationFilters{
		Patch:             "16.10",
		MinGames:          50,
		WeakSignalWinRate: 55,
		Limit:             10,
	})
	if err != nil {
		t.Fatalf("query win condition validation: %v", err)
	}
	if report.Patch != "16.10" || report.Matches != 100 || report.Teams != 200 {
		t.Fatalf("report scope = %+v; want patch 16.10, 100 matches, 200 teams", report)
	}
	if len(report.RatingOutcomes) != 2 || report.RatingOutcomes[0].Axis != "Pick" || report.RatingOutcomes[0].WinRate != 55 {
		t.Fatalf("rating outcomes = %+v; want Pick 55%% first", report.RatingOutcomes)
	}
	if len(report.ScoreDeltaOutcomes) != 2 {
		t.Fatalf("score delta outcomes = %d; want 2", len(report.ScoreDeltaOutcomes))
	}
	if len(report.PrimaryMatchups) != 2 || report.PrimaryMatchups[0].Condition != "TeamFight" || report.PrimaryMatchups[0].Direction != "favorable" || report.PrimaryMatchups[0].Signal <= 0 {
		t.Fatalf("primary matchups = %+v; want strongest Wilson signal first", report.PrimaryMatchups)
	}
	if len(report.WeakSignalWarnings) != 1 || report.WeakSignalWarnings[0].Condition != "SplitPush" {
		t.Fatalf("weak warnings = %+v; want SplitPush warning", report.WeakSignalWarnings)
	}
	if len(report.SynergyResiduals) != 0 {
		t.Fatalf("synergy residuals = %+v; want none from empty mock rows", report.SynergyResiduals)
	}
	if len(report.Findings) == 0 {
		t.Fatal("expected generated validation findings")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestWinConditionSynergyHelpers(t *testing.T) {
	pairs := winConditionChampionPairs([]uint16{22, 0, 11, 11, 33})
	want := [][2]uint16{{11, 22}, {11, 33}, {22, 33}}
	if len(pairs) != len(want) {
		t.Fatalf("pairs = %+v; want %+v", pairs, want)
	}
	for index := range want {
		if pairs[index] != want[index] {
			t.Fatalf("pairs[%d] = %+v; want %+v", index, pairs[index], want[index])
		}
	}

	direction, signal := winConditionResidualSignal(64, 80, 60)
	if direction != "overperforming" || signal != 4 {
		t.Fatalf("over signal = %s %.2f; want overperforming 4", direction, signal)
	}
	direction, signal = winConditionResidualSignal(35, 45, 50)
	if direction != "underperforming" || signal != 5 {
		t.Fatalf("under signal = %s %.2f; want underperforming 5", direction, signal)
	}
	direction, signal = winConditionResidualSignal(45, 55, 50)
	if direction != "mixed" || signal != 0 {
		t.Fatalf("mixed signal = %s %.2f; want mixed 0", direction, signal)
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

func TestCommonChampionGuideMatchupsUseSummaryReadModel(t *testing.T) {
	db, mock, cleanup := newMockRepository(t)
	defer cleanup()
	repo := &Repository{db: db}

	mock.ExpectQuery("(?s)FROM champion_matchup_analytics FINAL").
		WithArgs(sqlmock.AnyArg(), "266", "TOP", "16.11", 15, 3).
		WillReturnRows(sqlmock.NewRows([]string{"opponent_champion_id", "wins", "games", "win_rate"}).
			AddRow(122, 8, 15, 0.5333).
			AddRow(78, 7, 14, 0.5))

	rows, err := repo.QueryCommonChampionGuideMatchups(context.Background(), map[string]string{
		"champion_id": "266",
		"role":        "TOP",
		"patch":       "16.11",
		"rank_bucket": "ALL",
	}, 15, 3)
	if err != nil {
		t.Fatalf("common champion guide matchups: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d; want 2", len(rows))
	}
	if rows[0].OpponentChampionID != 122 || rows[0].Games != 15 || rows[1].OpponentChampionID != 78 {
		t.Fatalf("common matchup rows = %+v", rows)
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
