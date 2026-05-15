package main

import (
	"context"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"winrift/services/core/internal/analytics"
	"winrift/services/core/internal/clickhouse"
	"winrift/services/core/internal/collector"
	"winrift/services/core/internal/config"
	"winrift/services/core/internal/riot"
)

func main() {
	cfg := config.Load()
	riot.ClearAuthFailureMarker(cfg)
	riot.StartAuthFailureMonitor(cfg, "winrift worker")
	riotClient := riot.NewClient(cfg)
	repo, err := clickhouse.NewRepository(cfg)
	if err != nil {
		log.Fatalf("clickhouse: %v", err)
	}
	if cfg.CollectorPruneOldPatches {
		pruned, err := repo.DeletePatchesOutsideWindow(context.Background(), cfg.CollectorCurrentPatch, cfg.CollectorPatchRetention)
		if err != nil {
			log.Fatalf("collector patch retention prune failed: %v", err)
		}
		log.Printf(
			"collector patch retention prune current_patch=%s retained_patches=%s pruned_patches=%s",
			cfg.CollectorCurrentPatch,
			strings.Join(analytics.PatchWindow(cfg.CollectorCurrentPatch, cfg.CollectorPatchRetention), ","),
			strings.Join(pruned, ","),
		)
	}

	matchCollector := collector.New(riotClient, repo)
	platforms := collectorPlatforms(cfg)
	platformCountsByRegion := countPlatformsByRegion(platforms)
	log.Printf(
		"collector platforms=%s current_patch=%s patch_retention=%d idle_sleep=%s region_request_budget=%d rate_limit=%d/%s reserve=%d manual_match_cap=%d rank_lane_cap=%d",
		strings.Join(platforms, ","),
		cfg.CollectorCurrentPatch,
		cfg.CollectorPatchRetention,
		cfg.CollectorIdleSleep,
		cfg.CollectorUsableRequestsPerRegion(),
		cfg.CollectorRateLimitRequests,
		cfg.CollectorRateLimitWindow,
		cfg.CollectorRateLimitReserve,
		cfg.CollectorMaxRequests,
		cfg.RankEnrichmentMaxRequests,
	)
	seedRequestsByRegion, err := seedFrontier(context.Background(), cfg, riotClient, repo, platforms)
	if err != nil {
		if isRiotAuthError(err) {
			log.Fatalf("collector stopping: Riot API key is missing, expired, or not authorized")
		}
		log.Printf("seed frontier: %v", err)
	}
	ledger := newRegionRequestLedger(cfg)
	recordSeedRequests(ledger, seedRequestsByRegion)
	regions := configuredRegions(platforms)
	for {
		ctx := context.Background()
		rateLimitedRegions := map[string]bool{}
		regionPlatformsRemaining := cloneCounts(platformCountsByRegion)
		requestsThisSweep := 0
		platformsWithWork := 0
		for _, platform := range platforms {
			region, err := riot.RegionForPlatform(platform)
			if err != nil {
				log.Printf("collector platform skipped platform=%s err=%v", platform, err)
				continue
			}
			remainingPlatforms := regionPlatformsRemaining[region]
			if remainingPlatforms <= 0 {
				remainingPlatforms = 1
			}
			regionPlatformsRemaining[region]--
			if rateLimitedRegions[region] {
				log.Printf("collector platform skipped platform=%s region=%s reason=region_rate_limited_this_pass", platform, region)
				continue
			}
			available := ledger.Available(region, time.Now())
			if available <= 0 {
				log.Printf("collector platform deferred platform=%s region=%s reason=region_budget_wait wait=%s", platform, region, ledger.Wait(region, time.Now()).Round(time.Second))
				continue
			}
			budget := allocatePlatformBudget(cfg, available, remainingPlatforms)
			log.Printf(
				"collector platform budget platform=%s region=%s total_requests=%d match_requests=%d rank_lane_requests=%d region_available=%d region_platforms_remaining=%d",
				platform,
				region,
				budget.TotalRequests,
				budget.MatchRequests,
				budget.RankRequests,
				available,
				remainingPlatforms,
			)
			result := runPlatformPass(ctx, cfg, matchCollector, repo, platform, budget)
			if result.AuthFailed {
				log.Fatalf("collector stopping: Riot API key is missing, expired, or not authorized")
			}
			rateLimitHit := false
			if result.RateLimited {
				rateLimitedRegions[region] = true
				rateLimitHit = true
			}
			rankResult := collector.Result{}
			if !result.AuthFailed && !result.RateLimited && budget.RankRequests > 0 {
				rankResult = runRankPass(ctx, cfg, matchCollector, platform, budget.RankRequests)
				if rankResult.AuthFailed {
					log.Fatalf("collector stopping: Riot API key is missing, expired, or not authorized")
				}
				if rankResult.RateLimited {
					rateLimitedRegions[region] = true
					rateLimitHit = true
				}
			}
			requestsUsed := result.RequestsUsed + rankResult.RankRequestsUsed
			ledger.Record(region, requestsUsed, time.Now())
			if rateLimitHit {
				ledger.MarkFull(region, time.Now())
			}
			requestsThisSweep += requestsUsed
			if requestsUsed > 0 || result.MatchIDsSeen > 0 || result.MatchesInserted > 0 || result.MatchesSkipped > 0 || rankResult.RankSnapshotsInserted > 0 {
				platformsWithWork++
			}
		}

		sleepFor := nextSweepSleep(cfg, ledger, regions, requestsThisSweep)
		log.Printf(
			"collector sweep complete platforms=%d active_platforms=%d requests=%d sleep=%s",
			len(platforms),
			platformsWithWork,
			requestsThisSweep,
			sleepFor.Round(time.Second),
		)
		if sleepFor > 0 {
			time.Sleep(sleepFor)
		}
	}
}

func runPlatformPass(ctx context.Context, cfg config.Config, matchCollector collector.Collector, repo *clickhouse.Repository, platform string, budget platformRequestBudget) collector.Result {
	entries, err := repo.FetchDueFrontier(ctx, platform, cfg.CollectorFrontierBatchSize)
	if err != nil {
		log.Printf("frontier fetch platform=%s: %v", platform, err)
		return collector.Result{Errors: []string{err.Error()}}
	}
	if len(entries) == 0 {
		log.Printf("collector idle platform=%s: no due frontier rows", platform)
		return collector.Result{}
	}
	if budget.MatchRequests <= 0 {
		log.Printf("collector request budget exhausted before platform=%s frontier_rows=%d", platform, len(entries))
		return collector.Result{BudgetExhausted: true}
	}
	log.Printf(
		"collector platform pass start platform=%s current_patch=%s patch_retention=%d frontier_rows=%d match_request_budget=%d theoretical_max_matches=%d",
		platform,
		cfg.CollectorCurrentPatch,
		cfg.CollectorPatchRetention,
		len(entries),
		budget.MatchRequests,
		maxMatchesForBudget(len(entries), cfg.CollectorDefaultMatchCount, budget.MatchRequests),
	)
	requestsLeft := budget.MatchRequests
	var passResult collector.Result
	for _, entry := range entries {
		if requestsLeft <= 0 {
			log.Printf("collector request budget exhausted before platform=%s puuid=%s", platform, shortValue(entry.PUUID))
			break
		}
		result := matchCollector.CollectFromPUUIDWithOptions(ctx, entry.PUUID, entry.Platform, collector.CollectOptions{
			MatchCount:          cfg.CollectorDefaultMatchCount,
			MaxRequests:         requestsLeft,
			DiscoveryDelay:      cfg.CollectorDiscoveryDelay,
			DiscoveredPriority:  0,
			ApplyCachedRanks:    cfg.RankEnrichmentEnabled,
			RankSnapshotTTL:     cfg.RankSnapshotTTL,
			CurrentPatch:        cfg.CollectorCurrentPatch,
			PatchRetentionCount: cfg.CollectorPatchRetention,
		})
		passResult.MatchIDsSeen += result.MatchIDsSeen
		passResult.MatchesInserted += result.MatchesInserted
		passResult.MatchesSkipped += result.MatchesSkipped
		passResult.FrontierAdded += result.FrontierAdded
		passResult.RequestsUsed += result.RequestsUsed
		passResult.Errors = append(passResult.Errors, result.Errors...)
		passResult.AuthFailed = passResult.AuthFailed || result.AuthFailed
		passResult.RateLimited = passResult.RateLimited || result.RateLimited
		passResult.BudgetExhausted = passResult.BudgetExhausted || result.BudgetExhausted
		passResult.PatchBoundaryReached = passResult.PatchBoundaryReached || result.PatchBoundaryReached
		requestsLeft -= result.RequestsUsed
		if requestsLeft < 0 {
			requestsLeft = 0
		}
		status := frontierStatus(result)
		nextCheckAt := nextFrontierCheck(cfg, result)
		if err := repo.MarkFrontierChecked(ctx, entry, result.MatchIDsSeen, result.MatchesInserted, result.MatchesSkipped, len(result.Errors), result.RequestsUsed, status, nextCheckAt); err != nil {
			log.Printf("frontier update platform=%s puuid=%s: %v", entry.Platform, shortValue(entry.PUUID), err)
		}
		log.Printf(
			"collector platform=%s puuid=%s seen=%d inserted=%d skipped=%d frontier_added=%d requests=%d patch_boundary=%t errors=%d status=%s",
			entry.Platform,
			shortValue(entry.PUUID),
			result.MatchIDsSeen,
			result.MatchesInserted,
			result.MatchesSkipped,
			result.FrontierAdded,
			result.RequestsUsed,
			result.PatchBoundaryReached,
			len(result.Errors),
			status,
		)
		if result.AuthFailed || result.RateLimited || result.BudgetExhausted {
			break
		}
	}
	log.Printf(
		"collector platform pass complete platform=%s seen=%d inserted=%d skipped=%d frontier_added=%d requests=%d errors=%d patch_boundary=%t budget_exhausted=%t auth_failed=%t rate_limited=%t",
		platform,
		passResult.MatchIDsSeen,
		passResult.MatchesInserted,
		passResult.MatchesSkipped,
		passResult.FrontierAdded,
		passResult.RequestsUsed,
		len(passResult.Errors),
		passResult.PatchBoundaryReached,
		passResult.BudgetExhausted,
		passResult.AuthFailed,
		passResult.RateLimited,
	)
	return passResult
}

func runRankPass(ctx context.Context, cfg config.Config, matchCollector collector.Collector, platform string, rankRequests int) collector.Result {
	if !cfg.RankEnrichmentEnabled || rankRequests <= 0 {
		return collector.Result{}
	}
	result := matchCollector.CollectRanksForPlatform(ctx, platform, collector.RankCollectOptions{
		MaxRequests:     rankRequests,
		CandidateLimit:  rankRequests,
		RankSnapshotTTL: cfg.RankSnapshotTTL,
	})
	log.Printf(
		"rank lane platform=%s candidates_budget=%d rank_requests=%d rank_snapshots=%d errors=%d auth_failed=%t rate_limited=%t",
		platform,
		rankRequests,
		result.RankRequestsUsed,
		result.RankSnapshotsInserted,
		len(result.Errors),
		result.AuthFailed,
		result.RateLimited,
	)
	return result
}

type platformRequestBudget struct {
	TotalRequests int
	MatchRequests int
	RankRequests  int
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
	matchRequests := total - rankRequests
	if cfg.CollectorMaxRequests > 0 && matchRequests > cfg.CollectorMaxRequests {
		matchRequests = cfg.CollectorMaxRequests
	}
	if matchRequests < 0 {
		matchRequests = 0
	}
	return platformRequestBudget{
		TotalRequests: matchRequests + rankRequests,
		MatchRequests: matchRequests,
		RankRequests:  rankRequests,
	}
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

func seedFrontier(ctx context.Context, cfg config.Config, riotClient *riot.Client, repo *clickhouse.Repository, platforms []string) (map[string]int, error) {
	requestsByRegion := map[string]int{}
	for _, puuid := range splitCSV(os.Getenv("COLLECTOR_SEED_PUUIDS")) {
		ok, err := repo.InsertFrontierSeed(ctx, clickhouse.FrontierSeed{
			PUUID:        puuid,
			Platform:     cfg.DefaultPlatform,
			Source:       "seed-env-puuid",
			SourceDetail: "COLLECTOR_SEED_PUUIDS",
			Priority:     100,
			NextCheckAt:  time.Now(),
			Force:        true,
		})
		if err != nil {
			return requestsByRegion, err
		}
		if ok {
			log.Printf("frontier seed added source=env-puuid platform=%s puuid=%s", cfg.DefaultPlatform, shortValue(puuid))
		}
	}
	for _, value := range splitCSV(os.Getenv("COLLECTOR_SEED_RIOT_IDS")) {
		gameName, tagLine, err := riot.ParseRiotID(value)
		if err != nil {
			log.Printf("invalid riot id %q: %v", value, err)
			continue
		}
		if region, err := riot.RegionForPlatform(cfg.DefaultPlatform); err == nil {
			requestsByRegion[region]++
		}
		account, err := riotClient.AccountByRiotID(ctx, gameName, tagLine, cfg.DefaultPlatform)
		if err != nil {
			if isRiotAuthError(err) {
				return requestsByRegion, err
			}
			log.Printf("resolve seed %q: %v", value, err)
			continue
		}
		if account == nil {
			continue
		}
		ok, err := repo.InsertFrontierSeed(ctx, clickhouse.FrontierSeed{
			PUUID:        account.PUUID,
			Platform:     cfg.DefaultPlatform,
			Source:       "seed-env-riot-id",
			SourceDetail: value,
			Priority:     100,
			NextCheckAt:  time.Now(),
			Force:        true,
		})
		if err != nil {
			return requestsByRegion, err
		}
		if ok {
			log.Printf("frontier seed added source=env-riot-id platform=%s riot_id=%s", cfg.DefaultPlatform, value)
		}
	}
	if cfg.CollectorAutoSeedChallenger {
		for _, platform := range platforms {
			requestsUsed, err := seedChallengerFrontier(ctx, cfg, riotClient, repo, platform)
			if region, regionErr := riot.RegionForPlatform(platform); regionErr == nil {
				requestsByRegion[region] += requestsUsed
			}
			if err != nil {
				if isRiotAuthError(err) {
					return requestsByRegion, err
				}
				log.Printf("frontier auto seed platform=%s: %v", platform, err)
			}
		}
	}
	return requestsByRegion, nil
}

func seedChallengerFrontier(ctx context.Context, cfg config.Config, riotClient *riot.Client, repo *clickhouse.Repository, platform string) (int, error) {
	limit := cfg.CollectorAutoSeedLimit
	if limit <= 0 {
		limit = 3
	}
	requestsUsed := 1
	league, err := riotClient.ChallengerLeagueByQueue(ctx, platform, "RANKED_SOLO_5x5")
	if err != nil {
		return requestsUsed, err
	}
	if league == nil {
		return requestsUsed, nil
	}
	entries := append([]riot.LeagueEntry(nil), league.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LeaguePoints > entries[j].LeaguePoints
	})
	inserted := 0
	considered := 0
	for _, entry := range entries {
		if considered >= limit {
			break
		}
		considered++
		puuid := strings.TrimSpace(entry.PUUID)
		if puuid == "" && entry.SummonerID != "" {
			requestsUsed++
			summoner, err := riotClient.SummonerByID(ctx, entry.SummonerID, platform)
			if err != nil {
				if isRiotAuthError(err) {
					return requestsUsed, err
				}
				log.Printf("frontier auto seed summoner lookup failed platform=%s summoner_id=%s err=%v", platform, shortValue(entry.SummonerID), err)
			}
			if summoner != nil {
				puuid = summoner.PUUID
			}
		}
		if puuid == "" {
			continue
		}
		ok, err := repo.InsertFrontierSeed(ctx, clickhouse.FrontierSeed{
			PUUID:        puuid,
			Platform:     platform,
			Source:       "seed-auto-challenger",
			SourceDetail: "RANKED_SOLO_5x5 challenger",
			Priority:     90,
			NextCheckAt:  time.Now(),
		})
		if err != nil {
			return requestsUsed, err
		}
		if ok {
			inserted++
		}
	}
	log.Printf("frontier auto seed complete platform=%s considered=%d inserted=%d", platform, considered, inserted)
	return requestsUsed, nil
}

func isRiotAuthError(err error) bool {
	return riot.IsAuthFailure(err)
}

func frontierStatus(result collector.Result) string {
	if result.AuthFailed {
		return "blocked"
	}
	if len(result.Errors) > 0 {
		return "error"
	}
	return "active"
}

func nextFrontierCheck(cfg config.Config, result collector.Result) time.Time {
	now := time.Now()
	if result.AuthFailed {
		return now.Add(24 * time.Hour)
	}
	if result.RateLimited || result.BudgetExhausted {
		return now.Add(cfg.CollectorInterval)
	}
	if len(result.Errors) > 0 {
		return now.Add(2 * cfg.CollectorInterval)
	}
	return now.Add(cfg.CollectorRecheckInterval)
}

func splitCSV(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func collectorPlatforms(cfg config.Config) []string {
	values := cfg.CollectorPlatforms
	if len(values) == 0 {
		values = []string{cfg.DefaultPlatform}
	}
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		platform := riot.NormalizePlatform(value)
		if platform == "" || seen[platform] {
			continue
		}
		if _, err := riot.RegionForPlatform(platform); err != nil {
			log.Printf("collector platform ignored platform=%s err=%v", platform, err)
			continue
		}
		seen[platform] = true
		out = append(out, platform)
	}
	if len(out) == 0 {
		out = []string{riot.NormalizePlatform(cfg.DefaultPlatform)}
	}
	return out
}

func shortValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 12 {
		return value
	}
	return value[:8] + "..." + value[len(value)-4:]
}
