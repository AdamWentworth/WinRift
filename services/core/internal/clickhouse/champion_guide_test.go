package clickhouse

import "testing"

func TestRoleScopesSeparateBuildAdviceFromChampionRankings(t *testing.T) {
	buildScope := analyticsRoleScope("TOP")
	if buildScope.whereSQL != "role IN ('TOP', 'MIDDLE')" {
		t.Fatalf("build scope = %q; wanted solo-lane merge", buildScope.whereSQL)
	}

	rankingScope := strictAnalyticsRoleScope("TOP")
	if rankingScope.whereSQL != "role = ?" || len(rankingScope.args) != 1 || rankingScope.args[0] != "TOP" {
		t.Fatalf("ranking scope = %+v; wanted strict top lane", rankingScope)
	}
}

func TestAnalyticsOpponentBucketAggregatesChampionWideRows(t *testing.T) {
	overallBucket := analyticsOpponentBucketExpr(map[string]string{"opponent_champion_id": ""})
	if overallBucket != "toUInt16(0)" {
		t.Fatalf("overall opponent bucket = %q; wanted champion-wide zero bucket", overallBucket)
	}

	matchupBucket := analyticsOpponentBucketExpr(map[string]string{"opponent_champion_id": "245"})
	if matchupBucket != "opponent_champion_id" {
		t.Fatalf("matchup opponent bucket = %q; wanted exact opponent column", matchupBucket)
	}
}

func TestItemCostExpressionSQLIsDeterministic(t *testing.T) {
	costs := map[uint32]uint32{
		2003: 50,
		1056: 400,
		1001: 300,
	}
	got := itemCostExpressionSQL("tie.item_id", costs)
	want := "multiIf(tie.item_id = 1001, toUInt32(300), tie.item_id = 1056, toUInt32(400), tie.item_id = 2003, toUInt32(50), toUInt32(0))"
	if got != want {
		t.Fatalf("item cost expression = %q; want %q", got, want)
	}
}

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

func TestApplyChampionTierScoresShrinksHotSmallSamples(t *testing.T) {
	rows := applyChampionTierScores([]ChampionGuideSummary{
		{
			ChampionID: 1,
			Role:       "TOP",
			Wins:       785,
			Games:      1532,
			WinRate:    0.5124,
			Confidence: 0.487,
			PickRate:   0.033,
			BanRate:    0.18,
			KDA:        2.4,
		},
		{
			ChampionID: 2,
			Role:       "TOP",
			Wins:       39,
			Games:      60,
			WinRate:    0.65,
			Confidence: 0.524,
			PickRate:   0.0013,
			BanRate:    0.038,
			KDA:        2.4,
		},
	})

	if rows[0].TierScore <= rows[1].TierScore {
		t.Fatalf("stable score = %.2f, hot small score = %.2f; wanted broad stable row higher", rows[0].TierScore, rows[1].TierScore)
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

func TestApplyChampionTierScoresRewardsDurabilityWithoutDeaths(t *testing.T) {
	rows := applyChampionTierScores([]ChampionGuideSummary{
		{
			ChampionID:                 1,
			Role:                       "TOP",
			Wins:                       52,
			Games:                      100,
			WinRate:                    0.52,
			Confidence:                 0.42,
			PickRate:                   0.08,
			KDA:                        2.5,
			KillParticipation:          0.48,
			AvgDamageDealtToChampions:  18000,
			AvgGoldEarned:              12500,
			AvgCS:                      190,
			AvgDamageTaken:             36000,
			AvgDamageSelfMitigated:     20000,
			AvgTotalTimeSpentDead:      160,
			AvgTimePlayed:              1800,
			AvgDamageDealtToObjectives: 5000,
			AvgDamageDealtToStructures: 4500,
			AvgVisionScore:             24,
		},
		{
			ChampionID:                 2,
			Role:                       "TOP",
			Wins:                       52,
			Games:                      100,
			WinRate:                    0.52,
			Confidence:                 0.42,
			PickRate:                   0.08,
			KDA:                        2.5,
			KillParticipation:          0.48,
			AvgDamageDealtToChampions:  18000,
			AvgGoldEarned:              12500,
			AvgCS:                      190,
			AvgDamageTaken:             12000,
			AvgDamageSelfMitigated:     3000,
			AvgTotalTimeSpentDead:      160,
			AvgTimePlayed:              1800,
			AvgDamageDealtToObjectives: 5000,
			AvgDamageDealtToStructures: 4500,
			AvgVisionScore:             24,
		},
	})

	if rows[0].SurvivabilityScore <= rows[1].SurvivabilityScore {
		t.Fatalf("survivability scores = %.2f and %.2f; wanted damage-soaking row higher", rows[0].SurvivabilityScore, rows[1].SurvivabilityScore)
	}
	if rows[0].ImpactScore <= rows[1].ImpactScore {
		t.Fatalf("impact scores = %.2f and %.2f; wanted damage-soaking row higher", rows[0].ImpactScore, rows[1].ImpactScore)
	}
}

func TestApplyCombatAverages(t *testing.T) {
	row := ChampionGuideSummary{Games: 4}
	applyCombatAverages(&row, 20, 10, 30)

	if row.AvgKills != 5 || row.AvgDeaths != 2.5 || row.AvgAssists != 7.5 || row.KDA != 5 {
		t.Fatalf("unexpected combat averages: %+v", row)
	}
}
