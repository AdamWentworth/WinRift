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
	opponent := winconditions.TeamProfile{PrimaryCondition: "Siege", PrimaryRating: "B"}
	rows := []clickhouse.WinConditionMetricRow{
		{TeamCondition: "Pick", TeamRating: "B", OpponentCondition: "Siege", OpponentRating: "B", TeamPrimary: true, GameLengthBucket: "20-25", Wins: 3, Games: 5},
		{TeamCondition: "Pick", TeamRating: "B", OpponentCondition: "Siege", OpponentRating: "B", TeamPrimary: false, GameLengthBucket: "20-25", Wins: 10, Games: 20},
		{TeamCondition: "Control", TeamRating: "C+", OpponentCondition: "Siege", OpponentRating: "B", TeamPrimary: false, GameLengthBucket: "25-30", Wins: 4, Games: 8},
	}

	got := buildCompiledWinConditionMatchups(rows, team, opponent, 5)

	if len(got) != 2 {
		t.Fatalf("matchups = %d, want 2", len(got))
	}
	if got[0].Condition != "Pick" || !got[0].Primary || got[0].Games != 5 || got[0].Wins != 3 {
		t.Fatalf("primary matchup = %+v, want primary Pick with 3/5", got[0])
	}
	if got[1].Condition != "Control" || got[1].Primary || got[1].Games != 8 || got[1].Wins != 4 {
		t.Fatalf("alternative matchup = %+v, want non-primary Control with 4/8", got[1])
	}
}
