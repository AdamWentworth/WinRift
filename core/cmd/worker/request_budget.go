package main

import (
	"context"
	"log"
	"sort"
	"time"

	"winrift/core/internal/clickhouse"
	"winrift/core/internal/config"
	"winrift/core/internal/riot"
)

type platformRequestBudget struct {
	TotalRequests int
	MatchRequests int
	RankRequests  int
	AliasRequests int
}

func allocatePlatformBudget(cfg config.Config, available, platformsRemaining int) platformRequestBudget {
	if available <= 0 || platformsRemaining <= 0 {
		return platformRequestBudget{}
	}
	total := available / platformsRemaining
	if available%platformsRemaining != 0 {
		total++
	}
	if total < 0 {
		total = 0
	}
	rankRequests := cfg.CollectorRankRequestBudget(total)
	aliasRequests := cfg.CollectorAccountAliasRequestBudget(total - rankRequests)
	matchRequests := total - rankRequests - aliasRequests
	if cfg.CollectorMaxRequests > 0 && matchRequests > cfg.CollectorMaxRequests {
		matchRequests = cfg.CollectorMaxRequests
	}
	if matchRequests < 0 {
		matchRequests = 0
	}
	return platformRequestBudget{
		TotalRequests: matchRequests + rankRequests + aliasRequests,
		MatchRequests: matchRequests,
		RankRequests:  rankRequests,
		AliasRequests: aliasRequests,
	}
}

func reserveSharedRequests(ctx context.Context, cfg config.Config, repo *clickhouse.Repository, route, source string, desired int) int {
	if desired <= 0 {
		return 0
	}
	reservation, err := repo.ReserveRiotRequests(ctx, route, source, desired, cfg.CollectorUsableRequestsPerRegion(), cfg.CollectorRateLimitWindow, time.Now())
	if err != nil {
		log.Printf("shared riot budget failed route=%s source=%s desired=%d err=%v", route, source, desired, err)
		return 0
	}
	if reservation.Granted < reservation.Desired {
		log.Printf(
			"shared riot budget limited route=%s source=%s desired=%d granted=%d wait=%s used=%d limit=%d",
			route,
			source,
			reservation.Desired,
			reservation.Granted,
			reservation.Wait.Round(time.Second),
			reservation.Used,
			reservation.Limit,
		)
	}
	return reservation.Granted
}

type regionRequestLedger struct {
	limit   int
	window  time.Duration
	entries map[string][]time.Time
}

func newRegionRequestLedger(cfg config.Config) *regionRequestLedger {
	return &regionRequestLedger{
		limit:   cfg.CollectorUsableRequestsPerRegion(),
		window:  cfg.CollectorRateLimitWindow,
		entries: map[string][]time.Time{},
	}
}

func (l *regionRequestLedger) Available(region string, now time.Time) int {
	l.prune(region, now)
	available := l.limit - len(l.entries[region])
	if available < 0 {
		return 0
	}
	return available
}

func (l *regionRequestLedger) Record(region string, requests int, now time.Time) {
	if requests <= 0 {
		return
	}
	l.prune(region, now)
	for range requests {
		l.entries[region] = append(l.entries[region], now)
	}
}

func (l *regionRequestLedger) MarkFull(region string, now time.Time) {
	l.prune(region, now)
	for len(l.entries[region]) < l.limit {
		l.entries[region] = append(l.entries[region], now)
	}
}

func (l *regionRequestLedger) Wait(region string, now time.Time) time.Duration {
	l.prune(region, now)
	if len(l.entries[region]) < l.limit {
		return 0
	}
	wait := l.entries[region][0].Add(l.window).Sub(now)
	if wait < 0 {
		return 0
	}
	return wait
}

func (l *regionRequestLedger) EarliestWait(regions []string, now time.Time) time.Duration {
	var best time.Duration
	for _, region := range regions {
		wait := l.Wait(region, now)
		if wait <= 0 {
			return 0
		}
		if best == 0 || wait < best {
			best = wait
		}
	}
	return best
}

func (l *regionRequestLedger) prune(region string, now time.Time) {
	cutoff := now.Add(-l.window)
	entries := l.entries[region]
	firstActive := 0
	for firstActive < len(entries) && !entries[firstActive].After(cutoff) {
		firstActive++
	}
	if firstActive > 0 {
		entries = append([]time.Time(nil), entries[firstActive:]...)
		l.entries[region] = entries
	}
}

func recordSeedRequests(ledger *regionRequestLedger, requestsByRegion map[string]int) {
	now := time.Now()
	for region, requestsUsed := range requestsByRegion {
		if requestsUsed <= 0 {
			continue
		}
		ledger.Record(region, requestsUsed, now)
		log.Printf("collector first sweep budget debit region=%s seed_requests=%d remaining=%d", region, requestsUsed, ledger.Available(region, now))
	}
}

func nextSweepSleep(cfg config.Config, ledger *regionRequestLedger, regions []string, requestsUsed int) time.Duration {
	if requestsUsed > 0 {
		return 0
	}
	wait := ledger.EarliestWait(regions, time.Now())
	if wait > 0 {
		return wait
	}
	return cfg.CollectorIdleSleep
}

func countPlatformsByRegion(platforms []string) map[string]int {
	counts := map[string]int{}
	for _, platform := range platforms {
		region, err := riot.RegionForPlatform(platform)
		if err != nil {
			continue
		}
		counts[region]++
	}
	return counts
}

func cloneCounts(counts map[string]int) map[string]int {
	out := make(map[string]int, len(counts))
	for key, value := range counts {
		out[key] = value
	}
	return out
}

func configuredRegions(platforms []string) []string {
	counts := countPlatformsByRegion(platforms)
	regions := make([]string, 0, len(counts))
	for region := range counts {
		regions = append(regions, region)
	}
	sort.Strings(regions)
	return regions
}

func maxMatchesForBudget(frontierRows, matchCount, matchRequests int) int {
	if frontierRows <= 0 || matchCount <= 0 || matchRequests <= frontierRows {
		return 0
	}
	byRequests := (matchRequests - frontierRows) / 2
	byIDs := frontierRows * matchCount
	if byRequests < byIDs {
		return byRequests
	}
	return byIDs
}
