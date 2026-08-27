package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"winrift/core/internal/clickhouse"
	"winrift/core/internal/config"
	"winrift/core/internal/riot"
	"winrift/core/internal/runstate"
)

func TestIsRiotAuthError(t *testing.T) {
	if !isRiotAuthError(riot.APIError{StatusCode: http.StatusUnauthorized}) {
		t.Fatal("expected 401 to be an auth error")
	}
	if !isRiotAuthError(errors.Join(errors.New("wrapped"), riot.APIError{StatusCode: http.StatusForbidden})) {
		t.Fatal("expected wrapped 403 to be an auth error")
	}
	if !isRiotAuthError(riot.APIError{StatusCode: http.StatusServiceUnavailable, Message: "RIOT_API_KEY is not configured"}) {
		t.Fatal("expected missing key to be an auth error")
	}
	if isRiotAuthError(riot.APIError{StatusCode: http.StatusTooManyRequests}) {
		t.Fatal("did not expect 429 to be an auth error")
	}
}

func TestIsRiotRateLimitError(t *testing.T) {
	if !isRiotRateLimitError(riot.APIError{StatusCode: http.StatusTooManyRequests}) {
		t.Fatal("expected 429 to be a rate-limit error")
	}
	if isRiotRateLimitError(riot.APIError{StatusCode: http.StatusForbidden}) {
		t.Fatal("did not expect 403 to be a rate-limit error")
	}
}

func TestAllocatePlatformBudgetSharesAvailableBudget(t *testing.T) {
	cfg := config.Config{
		CollectorInterval:          120 * time.Second,
		CollectorRateLimitRequests: 100,
		CollectorRateLimitWindow:   120 * time.Second,
		CollectorRateLimitReserve:  10,
		RankEnrichmentEnabled:      true,
		RankEnrichmentMaxRequests:  5,
	}

	first := allocatePlatformBudget(cfg, 90, 4)
	if first.TotalRequests != 23 || first.MatchRequests != 18 || first.RankRequests != 5 {
		t.Fatalf("first budget = %+v, want total=23 match=18 rank=5", first)
	}

	second := allocatePlatformBudget(cfg, 67, 3)
	if second.TotalRequests != 23 || second.MatchRequests != 18 || second.RankRequests != 5 {
		t.Fatalf("second budget = %+v, want total=23 match=18 rank=5", second)
	}

	third := allocatePlatformBudget(cfg, 44, 2)
	if third.TotalRequests != 22 || third.MatchRequests != 17 || third.RankRequests != 5 {
		t.Fatalf("third budget = %+v, want total=22 match=17 rank=5", third)
	}
}

func TestAllocatePlatformBudgetReservesAliasLane(t *testing.T) {
	cfg := config.Config{
		CollectorInterval:             120 * time.Second,
		CollectorRateLimitRequests:    100,
		CollectorRateLimitWindow:      120 * time.Second,
		CollectorRateLimitReserve:     10,
		RankEnrichmentEnabled:         true,
		RankEnrichmentMaxRequests:     5,
		AccountAliasEnrichmentEnabled: true,
		AccountAliasMaxRequests:       3,
	}

	budget := allocatePlatformBudget(cfg, 90, 4)
	if budget.TotalRequests != 23 || budget.MatchRequests != 15 || budget.RankRequests != 5 || budget.AliasRequests != 3 {
		t.Fatalf("budget = %+v, want total=23 match=15 rank=5 alias=3", budget)
	}
}

func TestRegionRequestLedgerWaitsForRollingWindow(t *testing.T) {
	cfg := config.Config{
		CollectorInterval:          120 * time.Second,
		CollectorRateLimitRequests: 100,
		CollectorRateLimitWindow:   120 * time.Second,
		CollectorRateLimitReserve:  10,
	}
	ledger := newRegionRequestLedger(cfg)
	start := time.Date(2026, 5, 15, 5, 0, 0, 0, time.UTC)
	ledger.Record("AMERICAS", 90, start)

	if got := ledger.Available("AMERICAS", start.Add(30*time.Second)); got != 0 {
		t.Fatalf("available = %d, want 0", got)
	}
	if got := ledger.Wait("AMERICAS", start.Add(30*time.Second)); got != 90*time.Second {
		t.Fatalf("wait after 30s = %s, want 90s", got)
	}
	if got := ledger.Wait("AMERICAS", start.Add(60*time.Second)); got != 60*time.Second {
		t.Fatalf("wait after 60s = %s, want 60s", got)
	}
	if got := ledger.Available("AMERICAS", start.Add(121*time.Second)); got != 90 {
		t.Fatalf("available after window = %d, want 90", got)
	}
}

func TestMaxMatchesForBudget(t *testing.T) {
	if got := maxMatchesForBudget(1, 20, 18); got != 8 {
		t.Fatalf("max matches = %d, want 8", got)
	}
	if got := maxMatchesForBudget(1, 20, 40); got != 19 {
		t.Fatalf("max matches = %d, want 19", got)
	}
	if got := maxMatchesForBudget(3, 20, 90); got != 43 {
		t.Fatalf("max matches = %d, want 43", got)
	}
}

func TestCollectorPlatformsNormalizesAndDedupes(t *testing.T) {
	platforms := collectorPlatforms(config.Config{
		DefaultPlatform:    "NA1",
		CollectorPlatforms: []string{"na", "EUW", "EUW1", "bad-platform", "kr"},
	})

	want := []string{"NA1", "EUW1", "KR"}
	if len(platforms) != len(want) {
		t.Fatalf("platform count = %d, want %d: %v", len(platforms), len(want), platforms)
	}
	for i := range want {
		if platforms[i] != want[i] {
			t.Fatalf("platform[%d] = %q, want %q; all=%v", i, platforms[i], want[i], platforms)
		}
	}
}

func TestRefreshStatusRecorderWritesSuccessAndFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "refresh.json")
	recorder := newRefreshStatusRecorder(path)
	started := recorder.start("champion-guide-analytics", "16.11", 420, "test detail")
	recorder.succeed("champion-guide-analytics", started.Add(-25*time.Millisecond), map[string]int{"rows": 10})
	recorder.fail("summoner-profile-analytics", started.Add(-10*time.Millisecond), errors.New("boom"))

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var status runstate.WorkerRefreshStatus
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatal(err)
	}
	if len(status.Refreshes) != 2 {
		t.Fatalf("refresh count = %d, want 2", len(status.Refreshes))
	}
	byName := map[string]runstate.RefreshStatus{}
	for _, refresh := range status.Refreshes {
		byName[refresh.Name] = refresh
	}
	if byName["champion-guide-analytics"].Patch != "16.11" {
		t.Fatalf("patch = %q, want 16.11", byName["champion-guide-analytics"].Patch)
	}
	if byName["champion-guide-analytics"].Rows["rows"] != 10 {
		t.Fatalf("rows = %d, want 10", byName["champion-guide-analytics"].Rows["rows"])
	}
	if byName["summoner-profile-analytics"].LastError != "boom" {
		t.Fatalf("last error = %q, want boom", byName["summoner-profile-analytics"].LastError)
	}
}

func TestChampionPagePrewarmPatchListIncludesSelectableArchivedPatches(t *testing.T) {
	patches := championPagePrewarmPatchList(
		[]string{"16.17", "16.16"},
		[]clickhouse.PatchStat{
			{Patch: "16.17"},
			{Patch: "16.16"},
			{Patch: "16.15"},
			{Patch: "16.13"},
		},
	)
	want := []string{"16.17", "16.16", "16.15", "16.13"}
	if len(patches) != len(want) {
		t.Fatalf("patches = %v, want %v", patches, want)
	}
	for index := range want {
		if patches[index] != want[index] {
			t.Fatalf("patches = %v, want %v", patches, want)
		}
	}
}

func TestChampionPageStartupPrewarmPatchListUsesMatureFallback(t *testing.T) {
	patches := championPageStartupPrewarmPatchList(
		"16.17",
		2,
		[]clickhouse.PatchStat{
			{Patch: "16.15", Matches: 7000},
			{Patch: "16.17", Matches: 200},
			{Patch: "16.16", Matches: 4000},
			{Patch: "16.14", Matches: 9000},
		},
	)
	want := []string{"16.17", "16.15"}
	if len(patches) != len(want) {
		t.Fatalf("patches = %v, want %v", patches, want)
	}
	for index := range want {
		if patches[index] != want[index] {
			t.Fatalf("patches = %v, want %v", patches, want)
		}
	}
}

func TestChampionPageStartupPrewarmPatchListFallsBackToRetentionWindow(t *testing.T) {
	patches := championPageStartupPrewarmPatchList("16.17", 2, nil)
	want := []string{"16.17", "16.16"}
	if len(patches) != len(want) || patches[0] != want[0] || patches[1] != want[1] {
		t.Fatalf("patches = %v, want %v", patches, want)
	}
}
