package api

import (
	"testing"

	"winrift/services/core/internal/clickhouse"
	"winrift/services/core/internal/winconditions"
)

func TestCompiledWinConditionMatchupsRespectPrimaryFlag(t *testing.T) {
	team := winconditions.TeamProfile{
		PrimaryCondition: "Pick",
		PrimaryRating:    "B",
		Axes: []winconditions.AxisScore{
			{Key: "pick", Label: "Pick", Score: 14, Rating: "B"},
			{Key: "control", Label: "Control", Score: 10, Rating: "C+"},
		},
	}
	opponent := winconditions.TeamProfile{
		PrimaryCondition: "Siege",
		PrimaryRating:    "B",
		Axes: []winconditions.AxisScore{
			{Key: "siege", Label: "Siege", Score: 14, Rating: "B"},
		},
	}
	rows := []clickhouse.WinConditionMetricRow{
		{TeamCondition: "Pick", TeamRating: "B", OpponentCondition: "Siege", OpponentRating: "B", TeamPrimary: 11, GameLengthBucket: "ALL", Wins: 3, Games: 5, WinRate: 61.23, Confidence: 24.56},
		{TeamCondition: "Pick", TeamRating: "B", OpponentCondition: "Siege", OpponentRating: "B", TeamPrimary: 21, GameLengthBucket: "ALL", Wins: 10, Games: 20, WinRate: 50, Confidence: 29.93},
		{TeamCondition: "Control", TeamRating: "C+", OpponentCondition: "Siege", OpponentRating: "B", TeamPrimary: 21, GameLengthBucket: "ALL", Wins: 4, Games: 8, WinRate: 44.44, Confidence: 19.31},
		{TeamCondition: "Control", TeamRating: "C+", OpponentCondition: "Siege", OpponentRating: "B", TeamPrimary: 11, GameLengthBucket: "ALL", Wins: 99, Games: 100, WinRate: 99, Confidence: 94.55},
	}

	got := buildCompiledWinConditionMatchups(rows, team, opponent, 5)

	if len(got) != 2 {
		t.Fatalf("matchups = %d, want 2", len(got))
	}
	if got[0].Condition != "Pick" || !got[0].Primary || got[0].Games != 5 || got[0].Wins != 3 || got[0].WinRate != 61.23 {
		t.Fatalf("primary matchup = %+v, want primary Pick with stored 3/5 rate", got[0])
	}
	if got[1].Condition != "Control" || got[1].Primary || got[1].Games != 8 || got[1].Wins != 4 || got[1].WinRate != 44.44 {
		t.Fatalf("alternative matchup = %+v, want any-primary Control with stored 4/8 rate", got[1])
	}
}
