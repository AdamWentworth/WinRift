package main

import (
	"context"
	"log"
	"strings"
	"time"

	"winrift/core/internal/analytics"
	"winrift/core/internal/api"
	"winrift/core/internal/clickhouse"
	"winrift/core/internal/collector"
	"winrift/core/internal/config"
	"winrift/core/internal/riot"
	"winrift/core/internal/staticdata"
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
	staticService := staticdata.NewService(riotClient)
	apiServer := api.NewServer(cfg, riotClient, repo, staticService)
	platforms := collectorPlatforms(cfg)
	platformCountsByRegion := countPlatformsByRegion(platforms)
	writeWorkerHeartbeat(cfg, "starting", len(platforms), 0, 0, "worker startup")
	log.Printf(
		"collector platforms=%s current_patch=%s patch_retention=%d idle_sleep=%s region_request_budget=%d rate_limit=%d/%s reserve=%d manual_match_cap=%d rank_lane_cap=%d alias_lane_enabled=%t alias_lane_cap=%d item_slot_refresh_enabled=%t item_slot_refresh_interval=%s champion_guide_refresh_enabled=%t champion_guide_refresh_interval=%s champion_page_prewarm_enabled=%t champion_page_prewarm_per_role=%d champion_page_prewarm_min_games=%d champion_page_prewarm_matchups_per_champion=%d champion_page_prewarm_matchup_min_games=%d champion_page_prewarm_max_matchup_bundles=%d win_condition_refresh_enabled=%t win_condition_refresh_interval=%s summoner_profile_refresh_enabled=%t summoner_profile_refresh_interval=%s",
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
		cfg.AccountAliasEnrichmentEnabled,
		cfg.AccountAliasMaxRequests,
		cfg.ItemSlotRefreshEnabled,
		cfg.ItemSlotRefreshInterval,
		cfg.ChampionGuideRefreshEnabled,
		cfg.ChampionGuideRefreshInterval,
		cfg.ChampionPagePrewarmEnabled,
		cfg.ChampionPagePrewarmPerRole,
		cfg.ChampionPagePrewarmMinGames,
		cfg.ChampionPagePrewarmMatchupsPerChampion,
		cfg.ChampionPagePrewarmMatchupMinGames,
		cfg.ChampionPagePrewarmMaxMatchupBundles,
		cfg.WinConditionRefreshEnabled,
		cfg.WinConditionRefreshInterval,
		cfg.SummonerProfileRefreshEnabled,
		cfg.SummonerProfileRefreshInterval,
	)
	seedRequestsByRegion, err := seedFrontier(context.Background(), cfg, riotClient, repo, platforms)
	if err != nil {
		if isRiotAuthError(err) {
			stopForRiotAuthFailure(cfg, len(platforms))
		}
		log.Printf("seed frontier: %v", err)
	}
	ledger := newRegionRequestLedger(cfg)
	recordSeedRequests(ledger, seedRequestsByRegion)
	regions := configuredRegions(platforms)
	var lastItemSlotRefresh time.Time
	var lastChampionGuideRefresh time.Time
	var lastWinConditionRefresh time.Time
	var lastSummonerProfileRefresh time.Time
	refreshStatus := newRefreshStatusRecorder(cfg.WorkerRefreshStatusPath)
	maybeRefreshItemSlotAnalytics(context.Background(), cfg, staticService, repo, refreshStatus, &lastItemSlotRefresh)
	maybeRefreshChampionGuideAnalytics(context.Background(), cfg, apiServer, repo, refreshStatus, &lastChampionGuideRefresh)
	maybeRefreshWinConditionAnalytics(context.Background(), cfg, repo, refreshStatus, platforms, &lastWinConditionRefresh)
	maybeRefreshSummonerProfileAnalytics(context.Background(), cfg, repo, refreshStatus, &lastSummonerProfileRefresh)
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
				"collector platform budget platform=%s region=%s total_requests=%d match_requests=%d rank_lane_requests=%d alias_lane_requests=%d region_available=%d region_platforms_remaining=%d",
				platform,
				region,
				budget.TotalRequests,
				budget.MatchRequests,
				budget.RankRequests,
				budget.AliasRequests,
				available,
				remainingPlatforms,
			)
			result := runPlatformPass(ctx, cfg, matchCollector, repo, platform, budget)
			if result.AuthFailed {
				stopForRiotAuthFailure(cfg, len(platforms))
			}
			rateLimitHit := false
			if result.RateLimited {
				rateLimitedRegions[region] = true
				rateLimitHit = true
			}
			rankResult := collector.Result{}
			if !result.AuthFailed && !result.RateLimited && budget.RankRequests > 0 {
				rankResult = runRankPass(ctx, cfg, matchCollector, repo, platform, budget.RankRequests)
				if rankResult.AuthFailed {
					stopForRiotAuthFailure(cfg, len(platforms))
				}
				if rankResult.RateLimited {
					rateLimitedRegions[region] = true
					rateLimitHit = true
				}
			}
			matchAndRankRequestsUsed := result.RequestsUsed + rankResult.RankRequestsUsed
			ledger.Record(region, result.RequestsUsed, time.Now())
			aliasResult := accountAliasPassResult{}
			if !result.AuthFailed && !result.RateLimited && !rankResult.AuthFailed && !rankResult.RateLimited && budget.AliasRequests > 0 {
				accountRegion, accountRegionErr := riot.AccountRegionForPlatform(platform)
				if accountRegionErr != nil {
					log.Printf("account alias lane skipped platform=%s err=%v", platform, accountRegionErr)
				} else if rateLimitedRegions[accountRegion] {
					log.Printf("account alias lane skipped platform=%s account_region=%s reason=account_region_rate_limited_this_pass", platform, accountRegion)
				} else {
					accountAvailable := ledger.Available(accountRegion, time.Now())
					aliasRequests := min(budget.AliasRequests, accountAvailable)
					if aliasRequests <= 0 {
						log.Printf("account alias lane deferred platform=%s account_region=%s reason=account_region_budget_wait wait=%s", platform, accountRegion, ledger.Wait(accountRegion, time.Now()).Round(time.Second))
					} else {
						aliasResult = runAccountAliasPass(ctx, cfg, riotClient, repo, platform, aliasRequests)
						if aliasResult.AuthFailed {
							stopForRiotAuthFailure(cfg, len(platforms))
						}
						if aliasResult.RateLimited {
							rateLimitedRegions[accountRegion] = true
							ledger.MarkFull(accountRegion, time.Now())
						}
						ledger.Record(accountRegion, aliasResult.RequestsUsed, time.Now())
					}
				}
			}
			requestsUsed := matchAndRankRequestsUsed + aliasResult.RequestsUsed
			if rateLimitHit {
				ledger.MarkFull(region, time.Now())
			}
			requestsThisSweep += requestsUsed
			if requestsUsed > 0 || result.MatchIDsSeen > 0 || result.MatchesInserted > 0 || result.MatchesSkipped > 0 || rankResult.RankSnapshotsInserted > 0 || aliasResult.AliasesInserted > 0 {
				platformsWithWork++
			}
		}

		maybeRefreshItemSlotAnalytics(ctx, cfg, staticService, repo, refreshStatus, &lastItemSlotRefresh)
		maybeRefreshChampionGuideAnalytics(ctx, cfg, apiServer, repo, refreshStatus, &lastChampionGuideRefresh)
		maybeRefreshWinConditionAnalytics(ctx, cfg, repo, refreshStatus, platforms, &lastWinConditionRefresh)
		maybeRefreshSummonerProfileAnalytics(ctx, cfg, repo, refreshStatus, &lastSummonerProfileRefresh)
		sleepFor := nextSweepSleep(cfg, ledger, regions, requestsThisSweep)
		writeWorkerHeartbeat(cfg, "active", len(platforms), platformsWithWork, requestsThisSweep, "sleep="+sleepFor.Round(time.Second).String())
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
	route, err := riot.RegionForPlatform(platform)
	if err != nil {
		log.Printf("collector route lookup failed platform=%s err=%v", platform, err)
		return collector.Result{Errors: []string{err.Error()}}
	}
	budget.MatchRequests = reserveSharedRequests(ctx, cfg, repo, route, "worker:matches", budget.MatchRequests)
	if budget.MatchRequests <= 0 {
		log.Printf("collector shared request budget exhausted before platform=%s route=%s frontier_rows=%d", platform, route, len(entries))
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
