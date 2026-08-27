package api

import (
	"reflect"
	"testing"

	"winrift/core/internal/clickhouse"
)

func TestChampionPagePrewarmChampionIDsIncludesHistoricalOnlyChampions(t *testing.T) {
	current := []clickhouse.ChampionGuideSummary{
		{ChampionID: 62},
		{ChampionID: 266},
	}
	allPatches := []clickhouse.ChampionGuideSummary{
		{ChampionID: 103},
		{ChampionID: 62},
		{ChampionID: 84},
		{ChampionID: 0},
	}

	got := championPagePrewarmChampionIDs(current, allPatches)
	want := []uint16{62, 266, 103, 84}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("champion ids = %v, want %v", got, want)
	}
}

func TestFilterChampionPagePrewarmChampionIDsKeepsRequestedDiscoveryOrder(t *testing.T) {
	got := filterChampionPagePrewarmChampionIDs([]uint16{62, 266, 103, 84}, []uint16{84, 62, 0, 999})
	want := []uint16{62, 84}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("champion ids = %v, want %v", got, want)
	}
}

func TestChampionPageGuideGamesFindsCurrentPatchGaps(t *testing.T) {
	games, err := championPageGuideGames([]byte(`{"guide":{"summary":{"games":0}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if games != 0 {
		t.Fatalf("games = %d, want 0", games)
	}
	games, err = championPageGuideGames([]byte(`{"guide":{"summary":{"games":42}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if games != 42 {
		t.Fatalf("games = %d, want 42", games)
	}
}
