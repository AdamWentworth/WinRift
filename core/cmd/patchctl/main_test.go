package main

import (
	"os"
	"strings"
	"testing"
)

func TestSummonerProfilesDoesNotRequirePatchArgument(t *testing.T) {
	if patchRequired("summoner-profiles") {
		t.Fatal("summoner profile maintenance must be resumable without a patch argument")
	}
}

func TestArchiveRefreshesCombinedWinConditionsOnceAfterPlatformCompiles(t *testing.T) {
	sourceBytes, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read patchctl source: %v", err)
	}
	source := string(sourceBytes)
	start := strings.Index(source, "func archivePatch")
	if start < 0 {
		t.Fatal("could not locate archive implementation")
	}
	end := strings.Index(source[start:], "func deleteRawPatchData")
	if end < 0 {
		t.Fatal("could not locate archive implementation")
	}
	archiveImplementation := source[start : start+end]
	if count := strings.Count(archiveImplementation, "RefreshCombinedWinConditionMetrics"); count != 1 {
		t.Fatalf("combined win-condition refreshes = %d, want 1", count)
	}
	compileIndex := strings.Index(archiveImplementation, "CompilePatchMetrics")
	combinedIndex := strings.Index(archiveImplementation, "RefreshCombinedWinConditionMetrics")
	if compileIndex < 0 || combinedIndex < compileIndex {
		t.Fatal("combined win conditions must refresh after platform compilation")
	}
}
