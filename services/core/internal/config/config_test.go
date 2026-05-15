package config

import (
	"testing"
	"time"
)

func TestCollectorUsableRequestsPerRegion(t *testing.T) {
	cfg := Config{
		CollectorInterval:          120 * time.Second,
		CollectorRateLimitRequests: 100,
		CollectorRateLimitWindow:   120 * time.Second,
		CollectorRateLimitReserve:  10,
	}
	if got := cfg.CollectorUsableRequestsPerRegion(); got != 90 {
		t.Fatalf("usable requests = %d, want 90", got)
	}

	cfg.CollectorInterval = 60 * time.Second
	if got := cfg.CollectorUsableRequestsPerRegion(); got != 40 {
		t.Fatalf("scaled usable requests = %d, want 40", got)
	}
}

func TestCollectorRankRequestBudget(t *testing.T) {
	cfg := Config{
		RankEnrichmentEnabled:     true,
		RankEnrichmentMaxRequests: 5,
	}
	if got := cfg.CollectorRankRequestBudget(23); got != 5 {
		t.Fatalf("rank budget = %d, want 5", got)
	}
	if got := cfg.CollectorRankRequestBudget(3); got != 2 {
		t.Fatalf("small rank budget = %d, want 2", got)
	}

	cfg.RankEnrichmentEnabled = false
	if got := cfg.CollectorRankRequestBudget(23); got != 0 {
		t.Fatalf("disabled rank budget = %d, want 0", got)
	}
}
