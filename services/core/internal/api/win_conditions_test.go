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
			{Key: "pick", Label: "Pick", Score: 14, Rating: "B", PlanRole: "primary", PlanLabel: "Primary"},
			{Key: "control", Label: "Control", Score: 10, Rating: "C+", DeltaFromPrimary: 4, PlanRole: "secondary", PlanLabel: "Secondary"},
		},
	}
	opponent := winconditions.TeamProfile{
		PrimaryCondition: "Siege",
		PrimaryRating:    "B",
		Axes: []winconditions.AxisScore{
			{Key: "siege", Label: "Siege", Score: 14, Rating: "B", PlanRole: "primary", PlanLabel: "Primary"},
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
	if got[0].PlanRole != "primary" || got[0].OpponentPlanRole != "primary" {
		t.Fatalf("primary matchup plan roles = %s/%s, want primary/primary", got[0].PlanRole, got[0].OpponentPlanRole)
	}
	if got[1].Condition != "Control" || got[1].Primary || got[1].Games != 8 || got[1].Wins != 4 || got[1].WinRate != 44.44 {
		t.Fatalf("alternative matchup = %+v, want any-primary Control with stored 4/8 rate", got[1])
	}
	if got[1].PlanRole != "secondary" || got[1].DeltaFromPrimary != 4 {
		t.Fatalf("alternative matchup plan = %s delta %d, want secondary delta 4", got[1].PlanRole, got[1].DeltaFromPrimary)
	}
}

func TestWinConditionEvidenceSeparatesSampleSizeFromWinRate(t *testing.T) {
	largeSample := winConditionEvidence(5300, 10000)
	if largeSample.Direction != "favorable" {
		t.Fatalf("large sample direction = %q, want favorable", largeSample.Direction)
	}
	if largeSample.Score < 85 {
		t.Fatalf("large sample score = %.2f, want very strong evidence", largeSample.Score)
	}

	tinyHotSample := winConditionEvidence(13, 19)
	if tinyHotSample.Score >= largeSample.Score {
		t.Fatalf("tiny hot sample score = %.2f should be below large sample %.2f", tinyHotSample.Score, largeSample.Score)
	}
	if tinyHotSample.Level == "Strong" || tinyHotSample.Level == "Very strong" {
		t.Fatalf("tiny hot sample level = %q, want not strong", tinyHotSample.Level)
	}
}

func TestItemSlotFallbackScopesExpandWithoutDuplicates(t *testing.T) {
	filters := map[string]string{
		"champion_id":          "64",
		"role":                 "",
		"opponent_champion_id": "154",
		"patch":                "16.10",
		"rank_bucket":          "",
	}

	got := itemSlotFallbackScopes(filters)
	wantKeys := []string{
		"exact_patch_matchup",
		"all_patch_matchup",
		"patch_champion",
		"all_champion",
	}
	if len(got) != len(wantKeys) {
		t.Fatalf("scopes = %d, want %d: %+v", len(got), len(wantKeys), got)
	}
	for index, want := range wantKeys {
		if got[index].Key != want {
			t.Fatalf("scope[%d] = %q, want %q", index, got[index].Key, want)
		}
	}
	if got[0].Fallback {
		t.Fatalf("first scope should be exact, got fallback")
	}
	for _, scope := range got[1:] {
		if !scope.Fallback {
			t.Fatalf("scope %q should be marked fallback", scope.Key)
		}
	}
	if got[1].Filters["patch"] != "" || got[1].Filters["opponent_champion_id"] != "154" {
		t.Fatalf("all-patch matchup filters = %+v, want opponent retained and patch dropped", got[1].Filters)
	}
	if got[2].Filters["patch"] != "16.10" || got[2].Filters["opponent_champion_id"] != "" {
		t.Fatalf("patch champion filters = %+v, want patch retained and opponent dropped", got[2].Filters)
	}
	if got[3].Filters["patch"] != "" || got[3].Filters["opponent_champion_id"] != "" {
		t.Fatalf("all champion filters = %+v, want patch and opponent dropped", got[3].Filters)
	}
}

func TestItemSlotFallbackScopesCanSuppressChampionWideFallback(t *testing.T) {
	filters := map[string]string{
		"champion_id":          "421",
		"role":                 "JUNGLE",
		"opponent_champion_id": "950",
		"patch":                "16.10",
		"rank_bucket":          "",
	}

	got := itemSlotFallbackScopesWithOptions(filters, true)
	wantKeys := []string{"exact_patch_matchup", "all_patch_matchup"}
	if len(got) != len(wantKeys) {
		t.Fatalf("scopes = %d, want %d: %+v", len(got), len(wantKeys), got)
	}
	for index, want := range wantKeys {
		if got[index].Key != want {
			t.Fatalf("scope[%d] = %q, want %q", index, got[index].Key, want)
		}
	}
	if got[1].Filters["opponent_champion_id"] != "950" {
		t.Fatalf("fallback filters = %+v; wanted exact opponent retained", got[1].Filters)
	}
}

func TestItemSlotFallbackCompleteRequiresEverySlotToReachLimit(t *testing.T) {
	covered := map[uint8]int{1: 2, 2: 2, 3: 2, 4: 2, 5: 2}
	if itemSlotFallbackComplete(covered, 2) {
		t.Fatal("fallback should not be complete without slot 6")
	}
	covered[6] = 1
	if itemSlotFallbackComplete(covered, 2) {
		t.Fatal("fallback should not be complete until slot 6 reaches the requested option limit")
	}
	covered[6] = 2
	if !itemSlotFallbackComplete(covered, 2) {
		t.Fatal("fallback should be complete when all six slots reach the requested option limit")
	}
}

func TestBuildAdviceSampleQualityLabels(t *testing.T) {
	cases := []struct {
		games int
		key   string
		label string
	}{
		{0, "none", "No sample"},
		{3, "tiny", "Tiny sample"},
		{10, "early", "Early sample"},
		{30, "moderate", "Moderate sample"},
		{100, "strong", "Strong sample"},
	}
	for _, tc := range cases {
		if got := buildAdviceSampleQuality(tc.games); got != tc.key {
			t.Fatalf("quality(%d) = %q, want %q", tc.games, got, tc.key)
		}
		if got := buildAdviceSampleQualityLabel(tc.games); got != tc.label {
			t.Fatalf("quality label(%d) = %q, want %q", tc.games, got, tc.label)
		}
	}
}

func TestBuildAdviceNotesCallOutAllFallbackItemSlots(t *testing.T) {
	matchupSlots := []scopedItemSlotRow{
		{Row: clickhouse.ItemSlotRow{ItemSlot: 1, OpponentChampionID: 0, Games: 12}, Scope: itemSlotScope{Key: "patch_champion", Fallback: true}},
		{Row: clickhouse.ItemSlotRow{ItemSlot: 2, OpponentChampionID: 0, Games: 20}, Scope: itemSlotScope{Key: "patch_champion", Fallback: true}},
	}

	notes := buildAdviceNotes(true, matchupSlots, []scopedItemSlotRow{{Row: clickhouse.ItemSlotRow{ItemSlot: 1, Games: 50}}})
	if len(notes) == 0 || notes[0] != "No exact matchup item slots met the threshold yet; showing champion-wide slot signals as a baseline." {
		t.Fatalf("notes = %+v; wanted all-fallback warning", notes)
	}
}

func TestBuildAdviceNotesCallOutAllPatchMatchupFallback(t *testing.T) {
	matchupSlots := []scopedItemSlotRow{
		{Row: clickhouse.ItemSlotRow{ItemSlot: 1, OpponentChampionID: 950, Games: 12}, Scope: itemSlotScope{Key: "all_patch_matchup", Fallback: true}},
	}

	notes := buildAdviceNotes(true, matchupSlots, []scopedItemSlotRow{{Row: clickhouse.ItemSlotRow{ItemSlot: 1, Games: 50}}})
	if len(notes) == 0 || notes[0] != "No current-patch matchup item slots met the threshold yet; showing exact-matchup rows from broader patch scope." {
		t.Fatalf("notes = %+v; wanted exact-matchup broader-patch warning", notes)
	}
}

func TestBuildAdviceNotesSummarizeMixedAllPatchFallback(t *testing.T) {
	matchupSlots := []scopedItemSlotRow{
		{Row: clickhouse.ItemSlotRow{ItemSlot: 1, OpponentChampionID: 950, Games: 12}, Scope: itemSlotScope{Key: "exact_patch_matchup", Fallback: false}},
		{Row: clickhouse.ItemSlotRow{ItemSlot: 2, OpponentChampionID: 950, Games: 12}, Scope: itemSlotScope{Key: "all_patch_matchup", Fallback: true}},
	}

	notes := buildAdviceNotes(true, matchupSlots, []scopedItemSlotRow{{Row: clickhouse.ItemSlotRow{ItemSlot: 1, Games: 50}}})
	if len(notes) == 0 || notes[0] != "Some slots use exact-matchup data from other stored patches." {
		t.Fatalf("notes = %+v; wanted all-patch fallback note", notes)
	}
}

func TestBuildAdviceItemSlotDiagnosticsSummarizeScopes(t *testing.T) {
	rows := []scopedItemSlotRow{
		{
			Row: clickhouse.ItemSlotRow{ItemSlot: 1, ItemID: 3053, OpponentChampionID: 950, Games: 15, WinRate: 0.6},
			Scope: itemSlotScope{
				Key:      "exact_patch_matchup",
				Label:    "Current patch exact matchup",
				Fallback: false,
			},
		},
		{
			Row: clickhouse.ItemSlotRow{ItemSlot: 1, ItemID: 3071, OpponentChampionID: 950, Games: 12, WinRate: 0.55},
			Scope: itemSlotScope{
				Key:      "exact_patch_matchup",
				Label:    "Current patch exact matchup",
				Fallback: false,
			},
		},
		{
			Row: clickhouse.ItemSlotRow{ItemSlot: 2, ItemID: 3006, OpponentChampionID: 950, Games: 20, WinRate: 0.5},
			Scope: itemSlotScope{
				Key:      "all_patch_matchup",
				Label:    "Exact matchup, all stored patches",
				Fallback: true,
			},
		},
		{
			Row: clickhouse.ItemSlotRow{ItemSlot: 3, ItemID: 3026, OpponentChampionID: 0, Games: 40, WinRate: 0.525},
			Scope: itemSlotScope{
				Key:      "patch_champion",
				Label:    "Current patch champion overall",
				Fallback: true,
			},
		},
	}

	got := buildAdviceItemSlotDiagnostics(rows)
	if got.SelectedSlots[0].ItemID != 3053 || got.SelectedSlots[0].CandidateCount != 2 || got.SelectedSlots[0].WinRate != 60 {
		t.Fatalf("slot 1 diagnostic = %+v; wanted first exact row with two candidates", got.SelectedSlots[0])
	}
	if !intSlicesEqual(got.MissingSlots, []int{4, 5, 6}) {
		t.Fatalf("missing slots = %+v; want [4 5 6]", got.MissingSlots)
	}
	if !intSlicesEqual(got.FallbackSlots, []int{2, 3}) {
		t.Fatalf("fallback slots = %+v; want [2 3]", got.FallbackSlots)
	}
	if !intSlicesEqual(got.CurrentPatchExactSlots, []int{1}) {
		t.Fatalf("current exact slots = %+v; want [1]", got.CurrentPatchExactSlots)
	}
	if !intSlicesEqual(got.AllPatchExactSlots, []int{2}) {
		t.Fatalf("all-patch exact slots = %+v; want [2]", got.AllPatchExactSlots)
	}
	if !intSlicesEqual(got.ChampionWideSlots, []int{3}) {
		t.Fatalf("champion-wide slots = %+v; want [3]", got.ChampionWideSlots)
	}
	if len(got.ScopeCounts) != 3 || got.ScopeCounts[0].Rows != 2 || got.ScopeCounts[1].Rows != 1 || got.ScopeCounts[2].Rows != 1 {
		t.Fatalf("scope counts = %+v; wanted 2/1/1 rows by scope", got.ScopeCounts)
	}
}

func intSlicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}
