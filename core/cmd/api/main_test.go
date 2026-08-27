package main

import (
	"reflect"
	"testing"

	"winrift/core/internal/clickhouse"
)

func TestChampionPageHydrationPatchListUsesMatureFallbackThenEverySelectablePatch(t *testing.T) {
	patches := championPageHydrationPatchList("16.17", 2, []clickhouse.PatchStat{
		{Patch: "16.17", Matches: 2500},
		{Patch: "16.16", Matches: 62000},
		{Patch: "16.15", Matches: 3900},
		{Patch: "16.13", Matches: 350000},
	})
	if want := []string{"16.17", "16.16", "16.15", "16.13"}; !reflect.DeepEqual(patches, want) {
		t.Fatalf("patches = %v, want %v", patches, want)
	}
}

func TestChampionPageHydrationPatchListKeepsMatureFallbackAheadOfNewerThinPatch(t *testing.T) {
	patches := championPageHydrationPatchList("16.17", 2, []clickhouse.PatchStat{
		{Patch: "16.17", Matches: 200},
		{Patch: "16.16", Matches: 4000},
		{Patch: "16.15", Matches: 7000},
		{Patch: "16.14", Matches: 9000},
	})
	if want := []string{"16.17", "16.15", "16.16", "16.14"}; !reflect.DeepEqual(patches, want) {
		t.Fatalf("patches = %v, want %v", patches, want)
	}
}

func TestChampionPageHydrationPatchListFallsBackToRetentionWindow(t *testing.T) {
	patches := championPageHydrationPatchList("16.17", 2, nil)
	if want := []string{"16.17", "16.16"}; !reflect.DeepEqual(patches, want) {
		t.Fatalf("patches = %v, want %v", patches, want)
	}
}
