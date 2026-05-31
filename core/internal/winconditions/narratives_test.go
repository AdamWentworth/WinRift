package winconditions

import "testing"

func TestBuildNarrativeCoversEveryConditionRatingAndPrimaryMode(t *testing.T) {
	for _, condition := range Axes {
		for _, opponentCondition := range Axes {
			for _, rating := range RatingLabels {
				for _, opponentRating := range RatingLabels {
					for _, primary := range []bool{false, true} {
						for _, opponentPrimary := range []bool{false, true} {
							script := BuildNarrative(NarrativeMetric{
								Condition:         condition.Label,
								Rating:            rating,
								OpponentCondition: opponentCondition.Label,
								OpponentRating:    opponentRating,
								Primary:           primary,
								OpponentPrimary:   opponentPrimary,
								Wins:              6,
								Games:             10,
								WinRate:           60,
								Confidence:        31.27,
								MeetsMinGames:     true,
								Buckets: []NarrativeBucket{
									{Bucket: "0-20", Wins: 1, Games: 2, WinRate: 50, MeetsMinGames: false},
									{Bucket: "20-25", Wins: 2, Games: 3, WinRate: 66.67, MeetsMinGames: false},
								},
							})
							if script.ID == "" || script.Headline == "" || script.Overview == "" || script.Matchup == "" || script.RatingRead == "" || script.ModeRead == "" || script.TimingRead == "" || script.SampleRead == "" || script.PlayerRead == "" {
								t.Fatalf("incomplete script for %s %s primary=%t vs %s %s opponent_primary=%t: %+v", condition.Label, rating, primary, opponentCondition.Label, opponentRating, opponentPrimary, script)
							}
							if len(script.Facts) == 0 {
								t.Fatalf("script facts missing for %s", script.ID)
							}
						}
					}
				}
			}
		}
	}
}

func TestAllNarrativeScriptsBuildsFullCorpus(t *testing.T) {
	got := AllNarrativeScripts()
	want := len(Axes) * len(Axes) * len(RatingLabels) * len(RatingLabels) * 4
	if len(got) != want {
		t.Fatalf("AllNarrativeScripts() = %d, want %d", len(got), want)
	}
	seen := make(map[string]bool, len(got))
	for _, script := range got {
		if seen[script.ID] {
			t.Fatalf("duplicate script id %q", script.ID)
		}
		seen[script.ID] = true
	}
}

func TestBuildNarrativeTimingHandlesNoBuckets(t *testing.T) {
	script := BuildNarrative(NarrativeMetric{
		Condition:         "Pick",
		Rating:            "B",
		OpponentCondition: "Siege",
		OpponentRating:    "C+",
	})
	if script.TimingRead == "" || script.SampleRead == "" || script.PlayerRead == "" {
		t.Fatalf("script should explain missing data: %+v", script)
	}
}

func TestBuildNarrativeWarnsWhenWeakPlanHasInflatedWinrate(t *testing.T) {
	script := BuildNarrative(NarrativeMetric{
		Condition:         "SplitPush",
		Rating:            "C-",
		PlanRole:          "weak-angle",
		PlanLabel:         "Weak angle",
		OpponentCondition: "Control",
		OpponentRating:    "B",
		Primary:           false,
		OpponentPrimary:   true,
		Wins:              37,
		Games:             60,
		WinRate:           61.67,
		Confidence:        48,
		MeetsMinGames:     true,
	})
	if script.CautionRead == "" {
		t.Fatalf("expected caution read for weak inflated plan: %+v", script)
	}
}

func TestBuildNarrativeDoesNotWarnForPrimaryPlan(t *testing.T) {
	script := BuildNarrative(NarrativeMetric{
		Condition:         "TeamFight",
		Rating:            "A",
		PlanRole:          "primary",
		PlanLabel:         "Primary",
		OpponentCondition: "Control",
		OpponentRating:    "B",
		Primary:           true,
		OpponentPrimary:   true,
		Wins:              37,
		Games:             60,
		WinRate:           61.67,
		Confidence:        48,
		MeetsMinGames:     true,
	})
	if script.CautionRead != "" {
		t.Fatalf("unexpected caution read for primary plan: %q", script.CautionRead)
	}
}
