package main

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"winrift/services/core/internal/config"
	"winrift/services/core/internal/riot"
)

func TestIsRiotAuthError(t *testing.T) {
	if !isRiotAuthError(riot.APIError{StatusCode: http.StatusUnauthorized}) {
		t.Fatal("expected 401 to be an auth error")
	}
	if !isRiotAuthError(errors.Join(errors.New("wrapped"), riot.APIError{StatusCode: http.StatusForbidden})) {
		t.Fatal("expected wrapped 403 to be an auth error")
	}
	if isRiotAuthError(riot.APIError{StatusCode: http.StatusTooManyRequests}) {
		t.Fatal("did not expect 429 to be an auth error")
	}
}

func TestAllocatePlatformBudgetSharesRegionalBudget(t *testing.T) {
	cfg := config.Config{
		CollectorInterval:          120 * time.Second,
		CollectorRateLimitRequests: 100,
		CollectorRateLimitWindow:   120 * time.Second,
		CollectorRateLimitReserve:  10,
		RankEnrichmentEnabled:      true,
		RankEnrichmentMaxRequests:  5,
	}
	budgets := newRegionCycleBudgets(cfg, []string{"NA1", "BR1", "LA1", "LA2"})
	americas := budgets["AMERICAS"]

	first := allocatePlatformBudget(cfg, americas)
	if first.TotalRequests != 23 || first.MatchRequests != 18 || first.RankRequests != 5 {
		t.Fatalf("first budget = %+v, want total=23 match=18 rank=5", first)
	}
	recordRegionBudgetUse(americas, first.TotalRequests)

	second := allocatePlatformBudget(cfg, americas)
	if second.TotalRequests != 23 || second.MatchRequests != 18 || second.RankRequests != 5 {
		t.Fatalf("second budget = %+v, want total=23 match=18 rank=5", second)
	}
	recordRegionBudgetUse(americas, second.TotalRequests)

	third := allocatePlatformBudget(cfg, americas)
	if third.TotalRequests != 22 || third.MatchRequests != 17 || third.RankRequests != 5 {
		t.Fatalf("third budget = %+v, want total=22 match=17 rank=5", third)
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
