package clickhouse

import "testing"

func TestApplyChampionTierScoresRewardsStableWinningSamples(t *testing.T) {
	rows := applyChampionTierScores([]ChampionGuideSummary{
		{
			ChampionID: 1,
			Wins:       54,
			Games:      100,
			WinRate:    0.54,
			Confidence: 0.44,
			PickRate:   0.12,
			BanRate:    0.02,
			KDA:        3.0,
		},
		{
			ChampionID: 2,
			Wins:       1,
			Games:      1,
			WinRate:    1,
			Confidence: 0.21,
			PickRate:   0.01,
			BanRate:    0,
			KDA:        4.0,
		},
	})

	if rows[0].TierScore <= rows[1].TierScore {
		t.Fatalf("stable sample score = %.2f, tiny sample score = %.2f; wanted stable sample higher", rows[0].TierScore, rows[1].TierScore)
	}
}

func TestApplyChampionTierScoresAddsImpactSignal(t *testing.T) {
	rows := applyChampionTierScores([]ChampionGuideSummary{
		{ChampionID: 1, Wins: 55, Games: 100, WinRate: 0.55, Confidence: 0.45, PickRate: 0.1, KDA: 4.0},
		{ChampionID: 2, Wins: 55, Games: 100, WinRate: 0.55, Confidence: 0.45, PickRate: 0.1, KDA: 1.5},
	})

	if rows[0].ImpactScore <= rows[1].ImpactScore {
		t.Fatalf("impact scores = %.2f and %.2f; wanted higher KDA row to have higher impact", rows[0].ImpactScore, rows[1].ImpactScore)
	}
	if rows[0].TierScore <= rows[1].TierScore {
		t.Fatalf("tier scores = %.2f and %.2f; wanted impact to break otherwise similar rows", rows[0].TierScore, rows[1].TierScore)
	}
}

func TestApplyChampionTierScoresUsesPerformanceSignals(t *testing.T) {
	rows := applyChampionTierScores([]ChampionGuideSummary{
		{
			ChampionID:                 1,
			Role:                       "JUNGLE",
			Wins:                       55,
			Games:                      100,
			WinRate:                    0.55,
			Confidence:                 0.45,
			PickRate:                   0.1,
			KDA:                        3.0,
			KillParticipation:          0.68,
			AvgDamageDealtToChampions:  24000,
			AvgGoldEarned:              14500,
			AvgCS:                      205,
			AvgDamageDealtToObjectives: 38000,
			AvgDamageDealtToStructures: 5500,
			AvgObjectiveTakedowns:      2.4,
			AvgVisionScore:             32,
			AvgDamageTaken:             28000,
			AvgDamageSelfMitigated:     16000,
			AvgTotalTimeSpentDead:      220,
			AvgTimePlayed:              1900,
		},
		{
			ChampionID:                 2,
			Role:                       "JUNGLE",
			Wins:                       55,
			Games:                      100,
			WinRate:                    0.55,
			Confidence:                 0.45,
			PickRate:                   0.1,
			KDA:                        2.2,
			KillParticipation:          0.48,
			AvgDamageDealtToChampions:  14000,
			AvgGoldEarned:              11500,
			AvgCS:                      160,
			AvgDamageDealtToObjectives: 16000,
			AvgDamageDealtToStructures: 2200,
			AvgObjectiveTakedowns:      0.8,
			AvgVisionScore:             20,
			AvgDamageTaken:             28000,
			AvgDamageSelfMitigated:     9000,
			AvgTotalTimeSpentDead:      330,
			AvgTimePlayed:              1900,
		},
	})

	if rows[0].ObjectiveScore <= rows[1].ObjectiveScore {
		t.Fatalf("objective scores = %.2f and %.2f; wanted objective-heavy jungler higher", rows[0].ObjectiveScore, rows[1].ObjectiveScore)
	}
	if rows[0].ImpactScore <= rows[1].ImpactScore {
		t.Fatalf("impact scores = %.2f and %.2f; wanted richer performance row higher", rows[0].ImpactScore, rows[1].ImpactScore)
	}
}

func TestApplyChampionTierScoresWeightsSupportUtility(t *testing.T) {
	rows := applyChampionTierScores([]ChampionGuideSummary{
		{
			ChampionID:                1,
			Role:                      "UTILITY",
			Wins:                      53,
			Games:                     100,
			WinRate:                   0.53,
			Confidence:                0.43,
			PickRate:                  0.08,
			KDA:                       3.0,
			KillParticipation:         0.62,
			AvgVisionScore:            82,
			AvgTeamUtility:            9000,
			AvgTimeCCingOthers:        40,
			AvgDamageDealtToChampions: 9000,
		},
		{
			ChampionID:                2,
			Role:                      "UTILITY",
			Wins:                      53,
			Games:                     100,
			WinRate:                   0.53,
			Confidence:                0.43,
			PickRate:                  0.08,
			KDA:                       3.0,
			KillParticipation:         0.62,
			AvgVisionScore:            35,
			AvgTeamUtility:            2000,
			AvgTimeCCingOthers:        15,
			AvgDamageDealtToChampions: 15000,
		},
	})

	if rows[0].ImpactScore <= rows[1].ImpactScore {
		t.Fatalf("impact scores = %.2f and %.2f; wanted support utility row higher", rows[0].ImpactScore, rows[1].ImpactScore)
	}
}

func TestApplyCombatAverages(t *testing.T) {
	row := ChampionGuideSummary{Games: 4}
	applyCombatAverages(&row, 20, 10, 30)

	if row.AvgKills != 5 || row.AvgDeaths != 2.5 || row.AvgAssists != 7.5 || row.KDA != 5 {
		t.Fatalf("unexpected combat averages: %+v", row)
	}
}
