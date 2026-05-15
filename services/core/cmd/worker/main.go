package main

import (
	"context"
	"log"
	"os"
	"sort"
	"strings"
	"time"

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

	matchCollector := collector.New(riotClient, repo)
	platforms := collectorPlatforms(cfg)
	log.Printf(
		"collector platforms=%s interval=%s region_request_budget=%d rate_limit=%d/%s reserve=%d manual_match_cap=%d rank_cap=%d",
		strings.Join(platforms, ","),
		cfg.CollectorInterval,
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
	interval := cfg.CollectorInterval
	for {
		ctx := context.Background()
		rateLimitedRegions := map[string]bool{}
		regionBudgets := newRegionCycleBudgets(cfg, platforms)
		debitRegionBudgets(regionBudgets, seedRequestsByRegion)
		seedRequestsByRegion = nil
		for _, platform := range platforms {
			region, err := riot.RegionForPlatform(platform)
			if err != nil {
				log.Printf("collector platform skipped platform=%s err=%v", platform, err)
				continue
			}
			if rateLimitedRegions[region] {
				log.Printf("collector platform skipped platform=%s region=%s reason=region_rate_limited_this_pass", platform, region)
				continue
			}
			budget := allocatePlatformBudget(cfg, regionBudgets[region])
			log.Printf(
				"collector platform budget platform=%s region=%s total_requests=%d match_requests=%d rank_requests=%d region_remaining=%d region_platforms_remaining=%d",
				platform,
				region,
				budget.TotalRequests,
				budget.MatchRequests,
				budget.RankRequests,
				regionBudgets[region].Remaining,
				regionBudgets[region].PlatformsRemaining,
			)
			result := runPlatformPass(ctx, cfg, matchCollector, repo, platform, budget)
			recordRegionBudgetUse(regionBudgets[region], result.RequestsUsed+result.RankRequestsUsed)
			if result.AuthFailed {
				log.Fatalf("collector stopping: Riot API key is missing, expired, or not authorized")
			}
			if result.RateLimited {
				rateLimitedRegions[region] = true
			}
		}

		log.Printf("collector cycle complete platforms=%d sleeping=%s", len(platforms), interval)
		time.Sleep(interval)
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
		"collector platform pass start platform=%s frontier_rows=%d match_request_budget=%d rank_request_budget=%d theoretical_max_matches=%d",
		platform,
		len(entries),
		budget.MatchRequests,
		budget.RankRequests,
		maxMatchesForBudget(len(entries), cfg.CollectorDefaultMatchCount, budget.MatchRequests),
	)
	requestsLeft := budget.MatchRequests
	rankRequestsLeft := budget.RankRequests
	var passResult collector.Result
	for _, entry := range entries {
		if requestsLeft <= 0 {
			log.Printf("collector request budget exhausted before platform=%s puuid=%s", platform, shortValue(entry.PUUID))
			break
		}
		rankEnabled := cfg.RankEnrichmentEnabled
		if cfg.RankEnrichmentEnabled && rankRequestsLeft <= 0 {
			log.Printf("collector rank enrichment budget exhausted platform=%s", platform)
			rankEnabled = false
		}
		result := matchCollector.CollectFromPUUIDWithOptions(ctx, entry.PUUID, entry.Platform, collector.CollectOptions{
			MatchCount:            cfg.CollectorDefaultMatchCount,
			MaxRequests:           requestsLeft,
			DiscoveryDelay:        cfg.CollectorDiscoveryDelay,
			DiscoveredPriority:    0,
			RankEnrichmentEnabled: rankEnabled,
			RankSnapshotTTL:       cfg.RankSnapshotTTL,
			RankMaxRequests:       rankRequestsLeft,
		})
		passResult.MatchIDsSeen += result.MatchIDsSeen
		passResult.MatchesInserted += result.MatchesInserted
		passResult.MatchesSkipped += result.MatchesSkipped
		passResult.FrontierAdded += result.FrontierAdded
		passResult.RequestsUsed += result.RequestsUsed
		passResult.RankRequestsUsed += result.RankRequestsUsed
		passResult.RankSnapshotsInserted += result.RankSnapshotsInserted
		passResult.Errors = append(passResult.Errors, result.Errors...)
		passResult.AuthFailed = passResult.AuthFailed || result.AuthFailed
		passResult.RateLimited = passResult.RateLimited || result.RateLimited
		passResult.BudgetExhausted = passResult.BudgetExhausted || result.BudgetExhausted
		passResult.RankBudgetExhausted = passResult.RankBudgetExhausted || result.RankBudgetExhausted
		requestsLeft -= result.RequestsUsed
		if requestsLeft < 0 {
			requestsLeft = 0
		}
		if cfg.RankEnrichmentEnabled {
			rankRequestsLeft -= result.RankRequestsUsed
			if rankRequestsLeft < 0 {
				rankRequestsLeft = 0
			}
		}
		status := frontierStatus(result)
		nextCheckAt := nextFrontierCheck(cfg, result)
		if err := repo.MarkFrontierChecked(ctx, entry, result.MatchIDsSeen, result.MatchesInserted, result.MatchesSkipped, len(result.Errors), result.RequestsUsed+result.RankRequestsUsed, status, nextCheckAt); err != nil {
			log.Printf("frontier update platform=%s puuid=%s: %v", entry.Platform, shortValue(entry.PUUID), err)
		}
		log.Printf(
			"collector platform=%s puuid=%s seen=%d inserted=%d skipped=%d frontier_added=%d requests=%d rank_requests=%d rank_snapshots=%d errors=%d status=%s",
			entry.Platform,
			shortValue(entry.PUUID),
			result.MatchIDsSeen,
			result.MatchesInserted,
			result.MatchesSkipped,
			result.FrontierAdded,
			result.RequestsUsed,
			result.RankRequestsUsed,
			result.RankSnapshotsInserted,
			len(result.Errors),
			status,
		)
		if result.AuthFailed || result.RateLimited || result.BudgetExhausted {
			break
		}
	}
	log.Printf(
		"collector platform pass complete platform=%s seen=%d inserted=%d skipped=%d frontier_added=%d requests=%d rank_requests=%d rank_snapshots=%d errors=%d budget_exhausted=%t rank_budget_exhausted=%t auth_failed=%t rate_limited=%t",
		platform,
		passResult.MatchIDsSeen,
		passResult.MatchesInserted,
		passResult.MatchesSkipped,
		passResult.FrontierAdded,
		passResult.RequestsUsed,
		passResult.RankRequestsUsed,
		passResult.RankSnapshotsInserted,
		len(passResult.Errors),
		passResult.BudgetExhausted,
		passResult.RankBudgetExhausted,
		passResult.AuthFailed,
		passResult.RateLimited,
	)
	return passResult
}

type platformRequestBudget struct {
	TotalRequests int
	MatchRequests int
	RankRequests  int
}

type regionCycleBudget struct {
	Remaining          int
	PlatformsRemaining int
}

func newRegionCycleBudgets(cfg config.Config, platforms []string) map[string]*regionCycleBudget {
	counts := map[string]int{}
	for _, platform := range platforms {
		region, err := riot.RegionForPlatform(platform)
		if err != nil {
			continue
		}
		counts[region]++
	}
	budgets := make(map[string]*regionCycleBudget, len(counts))
	for region, count := range counts {
		budgets[region] = &regionCycleBudget{
			Remaining:          cfg.CollectorUsableRequestsPerRegion(),
			PlatformsRemaining: count,
		}
	}
	return budgets
}

func allocatePlatformBudget(cfg config.Config, budget *regionCycleBudget) platformRequestBudget {
	if budget == nil || budget.Remaining <= 0 || budget.PlatformsRemaining <= 0 {
		return platformRequestBudget{}
	}
	total := budget.Remaining / budget.PlatformsRemaining
	if budget.Remaining%budget.PlatformsRemaining != 0 {
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

func recordRegionBudgetUse(budget *regionCycleBudget, requestsUsed int) {
	if budget == nil {
		return
	}
	if requestsUsed < 0 {
		requestsUsed = 0
	}
	budget.Remaining -= requestsUsed
	if budget.Remaining < 0 {
		budget.Remaining = 0
	}
	budget.PlatformsRemaining--
	if budget.PlatformsRemaining < 0 {
		budget.PlatformsRemaining = 0
	}
}

func debitRegionBudgets(budgets map[string]*regionCycleBudget, requestsByRegion map[string]int) {
	for region, requestsUsed := range requestsByRegion {
		budget := budgets[region]
		if budget == nil || requestsUsed <= 0 {
			continue
		}
		budget.Remaining -= requestsUsed
		if budget.Remaining < 0 {
			budget.Remaining = 0
		}
		log.Printf("collector first cycle budget debit region=%s seed_requests=%d remaining=%d", region, requestsUsed, budget.Remaining)
	}
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
