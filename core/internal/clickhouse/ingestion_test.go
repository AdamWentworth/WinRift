package clickhouse

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"winrift/core/internal/analytics"
)

func TestInsertNormalizedBatchesRowsByTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	repo := &Repository{db: db}
	for _, table := range []string{
		"raw_matches",
		"raw_timelines",
		"participants",
		"participant_performance",
		"participant_matchups",
		"timeline_participant_frames",
		"timeline_item_events",
		"timeline_skill_events",
		"timeline_combat_events",
		"timeline_objective_events",
		"champion_bans",
	} {
		mock.ExpectExec("INSERT INTO " + table).WillReturnResult(sqlmock.NewResult(0, 1))
	}

	first := participantRow(1)
	second := participantRow(2)
	normalized := analytics.NormalizedMatch{
		RawMatch:    analytics.RawMatch{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, RawJSON: "{}"},
		RawTimeline: analytics.RawTimeline{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, RawJSON: "{}"},
		Participants: []analytics.ParticipantRow{
			first,
			second,
		},
		Matchups: []analytics.MatchupRow{
			{ParticipantRow: first, OpponentParticipantID: 2, OpponentChampionID: 2, OpponentRole: "JUNGLE"},
			{ParticipantRow: second, OpponentParticipantID: 1, OpponentChampionID: 1, OpponentRole: "TOP"},
		},
		TimelineParticipantFrames: []analytics.TimelineParticipantFrameRow{
			{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, TimestampMS: 60_000, ParticipantID: 1},
			{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, TimestampMS: 60_000, ParticipantID: 2},
		},
		TimelineItemEvents: []analytics.TimelineItemEventRow{
			{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, TimestampMS: 10_000, ParticipantID: 1, EventType: "ITEM_PURCHASED", ItemID: 1055},
			{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, TimestampMS: 10_000, ParticipantID: 2, EventType: "ITEM_PURCHASED", ItemID: 1056},
		},
		TimelineSkillEvents: []analytics.TimelineSkillEventRow{
			{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, TimestampMS: 20_000, ParticipantID: 1, SkillSlot: 1, SkillOrder: 1, LevelUpType: "NORMAL"},
			{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, TimestampMS: 20_000, ParticipantID: 2, SkillSlot: 2, SkillOrder: 1, LevelUpType: "NORMAL"},
		},
		TimelineCombatEvents: []analytics.TimelineCombatEventRow{
			{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, TimestampMS: 180_000, KillerID: 1, VictimID: 2},
			{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, TimestampMS: 240_000, KillerID: 2, VictimID: 1},
		},
		TimelineObjectiveEvents: []analytics.TimelineObjectiveEventRow{
			{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, TimestampMS: 300_000, EventType: "ELITE_MONSTER_KILL", KillerID: 1, TeamID: 100},
			{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, TimestampMS: 360_000, EventType: "BUILDING_KILL", KillerID: 2, TeamID: 200},
		},
		ChampionBans: []analytics.ChampionBanRow{
			{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, TeamID: 100, ChampionID: 1, PickTurn: 1},
			{MatchID: "NA1_1", Platform: "NA1", Patch: "16.11", QueueID: analytics.RankedSoloQueueID, TeamID: 200, ChampionID: 2, PickTurn: 2},
		},
	}

	if err := repo.InsertNormalized(context.Background(), normalized); err != nil {
		t.Fatalf("insert normalized: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestInsertRowsInBatchesChunksLargeTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO test_table").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO test_table").WillReturnResult(sqlmock.NewResult(0, 1))

	err = insertRowsInBatches(context.Background(), db, "INSERT INTO test_table (id) VALUES ", "?", maxBatchInsertRows+1, func(i int) []any {
		return []any{i}
	})
	if err != nil {
		t.Fatalf("insert rows in batches: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func participantRow(id uint8) analytics.ParticipantRow {
	return analytics.ParticipantRow{
		MatchID:                        "NA1_1",
		Platform:                       "NA1",
		Patch:                          "16.11",
		QueueID:                        analytics.RankedSoloQueueID,
		ParticipantID:                  id,
		PUUID:                          "puuid",
		TeamID:                         100,
		ChampionID:                     uint16(id),
		ChampionName:                   "Champion",
		Role:                           "TOP",
		Win:                            1,
		RankBucket:                     "UNKNOWN",
		TotalDamageDealtToChampions:    1,
		PhysicalDamageDealtToChampions: 1,
		MagicDamageDealtToChampions:    1,
		TrueDamageDealtToChampions:     1,
	}
}
