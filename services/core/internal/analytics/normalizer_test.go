package analytics

import (
	"os"
	"testing"
)

const legacyMatchFixture = "../../../../legacy/data/match_data.json"

func TestNormalizeLegacyMatchFixture(t *testing.T) {
	raw, err := os.ReadFile(legacyMatchFixture)
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := NormalizeMatch(raw, nil, "NA1", "UNKNOWN")
	if err != nil {
		t.Fatal(err)
	}
	if normalized.RawMatch.MatchID == "" {
		t.Fatal("expected match id")
	}
	if len(normalized.Participants) != 10 {
		t.Fatalf("participants = %d, want 10", len(normalized.Participants))
	}
	if len(normalized.Matchups) != 10 {
		t.Fatalf("matchups = %d, want 10", len(normalized.Matchups))
	}
	first := normalized.Participants[0]
	if first.ChampionID != 26 || first.Role != "TOP" {
		t.Fatalf("unexpected first participant: %+v", first)
	}
	if first.FinalItemsSignature != "1056-6656-3157-3158-1052" {
		t.Fatalf("final items = %q", first.FinalItemsSignature)
	}
	if first.SpellSignature != "4-12" {
		t.Fatalf("spell signature = %q", first.SpellSignature)
	}
}

func TestShouldIngestFixture(t *testing.T) {
	raw, err := os.ReadFile(legacyMatchFixture)
	if err != nil {
		t.Fatal(err)
	}
	if !ShouldIngest(raw) {
		t.Fatal("expected ranked SR fixture to ingest")
	}
}

func TestNormalizeTimelineEvents(t *testing.T) {
	raw, err := os.ReadFile(legacyMatchFixture)
	if err != nil {
		t.Fatal(err)
	}
	timeline := []byte(`{
		"info": {
			"frames": [
				{
					"timestamp": 60000,
					"participantFrames": {
						"1": {
							"participantId": 1,
							"level": 3,
							"xp": 720,
							"currentGold": 320,
							"totalGold": 1320,
							"minionsKilled": 15,
							"jungleMinionsKilled": 0,
							"position": {"x": 1200, "y": 1300},
							"damageStats": {
								"totalDamageDoneToChampions": 410,
								"totalDamageTaken": 390
							}
						}
					},
					"events": [
						{"type": "ITEM_PURCHASED", "timestamp": 15000, "participantId": 1, "itemId": 1056},
						{"type": "ITEM_PURCHASED", "timestamp": 420000, "participantId": 1, "itemId": 6656},
						{"type": "ITEM_PURCHASED", "timestamp": 800000, "participantId": 1, "itemId": 3157},
						{"type": "ITEM_UNDO", "timestamp": 850000, "participantId": 1, "beforeId": 1052, "afterId": 0},
						{"type": "CHAMPION_KILL", "timestamp": 900000, "killerId": 1, "victimId": 6, "assistingParticipantIds": [2,3], "bounty": 300, "shutdownBounty": 150, "position": {"x": 5000, "y": 5000}},
						{"type": "ELITE_MONSTER_KILL", "timestamp": 950000, "killerId": 2, "teamId": 100, "monsterType": "DRAGON", "monsterSubType": "FIRE_DRAGON", "position": {"x": 9866, "y": 4414}},
						{"type": "BUILDING_KILL", "timestamp": 1000000, "killerId": 4, "teamId": 100, "buildingType": "TOWER_BUILDING", "towerType": "OUTER_TURRET", "laneType": "MID_LANE", "position": {"x": 5846, "y": 6396}}
					]
				}
			]
		}
	}`)
	normalized, err := NormalizeMatch(raw, timeline, "NA1", "UNKNOWN")
	if err != nil {
		t.Fatal(err)
	}
	if len(normalized.TimelineParticipantFrames) != 1 {
		t.Fatalf("frames = %d, want 1", len(normalized.TimelineParticipantFrames))
	}
	if len(normalized.TimelineItemEvents) != 4 {
		t.Fatalf("item events = %d, want 4", len(normalized.TimelineItemEvents))
	}
	if len(normalized.TimelineCombatEvents) != 1 {
		t.Fatalf("combat events = %d, want 1", len(normalized.TimelineCombatEvents))
	}
	if len(normalized.TimelineObjectiveEvents) != 2 {
		t.Fatalf("objective events = %d, want 2", len(normalized.TimelineObjectiveEvents))
	}
	if normalized.Participants[0].Core3Signature != "1056-6656-3157" {
		t.Fatalf("core3 = %q", normalized.Participants[0].Core3Signature)
	}
	frame := normalized.TimelineParticipantFrames[0]
	if frame.TotalGold != 1320 || frame.TotalDamageDoneToChampions != 410 {
		t.Fatalf("unexpected frame: %+v", frame)
	}
	combat := normalized.TimelineCombatEvents[0]
	if combat.AssistingParticipantIDs != "2-3" || combat.ShutdownBounty != 150 {
		t.Fatalf("unexpected combat event: %+v", combat)
	}
	objective := normalized.TimelineObjectiveEvents[0]
	if objective.MonsterType != "DRAGON" || objective.MonsterSubType != "FIRE_DRAGON" {
		t.Fatalf("unexpected objective event: %+v", objective)
	}
}

func TestWilson(t *testing.T) {
	if WilsonLowerBound(8, 10, 1.96) <= WilsonLowerBound(1, 2, 1.96) {
		t.Fatal("expected larger sample to have better confidence")
	}
}
