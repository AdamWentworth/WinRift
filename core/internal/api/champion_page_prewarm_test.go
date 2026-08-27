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
