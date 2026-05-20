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

func TestApplyCombatAverages(t *testing.T) {
	row := ChampionGuideSummary{Games: 4}
	applyCombatAverages(&row, 20, 10, 30)

	if row.AvgKills != 5 || row.AvgDeaths != 2.5 || row.AvgAssists != 7.5 || row.KDA != 5 {
		t.Fatalf("unexpected combat averages: %+v", row)
	}
}
