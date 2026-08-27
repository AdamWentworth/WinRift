package main

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"winrift/core/internal/analytics"
	"winrift/core/internal/api"
	"winrift/core/internal/clickhouse"
	"winrift/core/internal/config"
	"winrift/core/internal/runstate"
	"winrift/core/internal/staticdata"
)

type refreshStatusRecorder struct {
	path     string
	statuses map[string]runstate.RefreshStatus
}

type refreshSchedulerState struct {
	lastItemSlotRefresh        time.Time
	lastChampionGuideRefresh   time.Time
	lastWinConditionRefresh    time.Time
	lastSummonerProfileRefresh time.Time
	nextLane                   int
}

const (
	championPageStartupPrewarmConcurrency = 4
	championPageStartupReadinessGrace     = 45 * time.Second
)

func newRefreshStatusRecorder(path string) *refreshStatusRecorder {
	return &refreshStatusRecorder{path: path, statuses: map[string]runstate.RefreshStatus{}}
}

func (r *refreshStatusRecorder) start(name, patch string, queueID int, detail string) time.Time {
	startedAt := time.Now()
	if r == nil {
		return startedAt
	}
	status := r.statuses[name]
	status.Name = name
	status.Patch = patch
	status.QueueID = queueID
	status.LastStartedAt = startedAt.UTC()
	status.Detail = detail
	r.statuses[name] = status
	r.flush()
	return startedAt
}

func (r *refreshStatusRecorder) succeed(name string, startedAt time.Time, rows map[string]int) {
	if r == nil {
		return
	}
	status := r.statuses[name]
	status.Name = name
	status.LastSucceededAt = time.Now().UTC()
	status.LastDurationMS = time.Since(startedAt).Milliseconds()
	status.LastError = ""
	status.Rows = rows
	r.statuses[name] = status
	r.flush()
}

func (r *refreshStatusRecorder) fail(name string, startedAt time.Time, err error) {
	if r == nil {
		return
	}
	status := r.statuses[name]
	status.Name = name
	status.LastFailedAt = time.Now().UTC()
	status.LastDurationMS = time.Since(startedAt).Milliseconds()
	if err != nil {
		status.LastError = err.Error()
	}
	r.statuses[name] = status
	r.flush()
}

func (r *refreshStatusRecorder) flush() {
	if r == nil {
		return
	}
	if err := runstate.WriteWorkerRefreshStatus(r.path, r.statuses); err != nil {
		log.Printf("worker refresh status write failed path=%s err=%v", r.path, err)
	}
}

func startRefreshScheduler(ctx context.Context, cfg config.Config, staticService *staticdata.Service, apiServer api.Server, repo *clickhouse.Repository, platforms []string) {
	go func() {
		interval := cfg.AnalyticsRefreshSchedulerInterval
		if interval <= 0 {
			interval = time.Minute
		}
		log.Printf("analytics refresh scheduler started interval=%s policy=start_immediately_one_due_family_per_tick", interval)
		state := refreshSchedulerState{}
		refreshStatus := newRefreshStatusRecorder(cfg.WorkerRefreshStatusPath)
		for {
			runNextRefreshSchedulerLane(ctx, cfg, staticService, apiServer, repo, refreshStatus, platforms, &state)
			select {
			case <-ctx.Done():
				log.Printf("analytics refresh scheduler stopped err=%v", ctx.Err())
				return
			case <-time.After(interval):
			}
		}
	}()
}

func runNextRefreshSchedulerLane(ctx context.Context, cfg config.Config, staticService *staticdata.Service, apiServer api.Server, repo *clickhouse.Repository, refreshStatus *refreshStatusRecorder, platforms []string, state *refreshSchedulerState) {
	if state == nil {
		return
	}
	lanes := []struct {
		run func() bool
	}{
		{
			run: func() bool {
				return maybeRefreshItemSlotAnalytics(ctx, cfg, staticService, repo, refreshStatus, &state.lastItemSlotRefresh)
			},
		},
		{
			run: func() bool {
				return maybeRefreshChampionGuideAnalytics(ctx, cfg, apiServer, repo, refreshStatus, &state.lastChampionGuideRefresh)
			},
		},
		{
			run: func() bool {
				return maybeRefreshWinConditionAnalytics(ctx, cfg, repo, refreshStatus, platforms, &state.lastWinConditionRefresh)
			},
		},
		{
			run: func() bool {
				return maybeRefreshSummonerProfileAnalytics(ctx, cfg, repo, refreshStatus, &state.lastSummonerProfileRefresh)
			},
		},
	}
	for i := 0; i < len(lanes); i++ {
		index := (state.nextLane + i) % len(lanes)
		if lanes[index].run() {
			state.nextLane = (index + 1) % len(lanes)
			return
		}
	}
	log.Printf("analytics refresh scheduler idle: no due refresh families")
}

func prewarmChampionPagesOnStartup(ctx context.Context, cfg config.Config, apiServer api.Server, repo *clickhouse.Repository) {
	if !cfg.ChampionPagePrewarmEnabled {
		return
	}
	currentPatch := strings.TrimSpace(cfg.CollectorCurrentPatch)
	if currentPatch == "" {
		return
	}
	stats, err := repo.PatchStats(ctx, analytics.RankedSoloQueueID)
	if err != nil {
		log.Printf("champion page canonical startup prewarm patch discovery failed err=%v", err)
	}
	patches := championPageStartupPrewarmPatchList(currentPatch, cfg.CollectorPatchRetention, stats)
	hadErrors := false
	var fallbackChampionIDs []uint16
	for index, patch := range patches {
		if index > 0 && hadErrors {
			break
		}
		startedAt := time.Now()
		scope := "all"
		if index > 0 {
			scope = "current-guide-gaps"
		}
		log.Printf("champion page canonical startup prewarm start patch=%s queue=%d concurrency=%d scope=%s requested_champions=%d", patch, analytics.RankedSoloQueueID, championPageStartupPrewarmConcurrency, scope, len(fallbackChampionIDs))
		if index > 0 && len(fallbackChampionIDs) == 0 {
			log.Printf("champion page canonical startup prewarm complete patch=%s queue=%d candidates=0 stored=0 cached=0 skipped=0 errors=0 duration=%s scope=%s", patch, analytics.RankedSoloQueueID, time.Since(startedAt).Round(time.Millisecond), scope)
			continue
		}
		result, prewarmErr := apiServer.PrewarmChampionPageBundles(ctx, api.ChampionPagePrewarmOptions{
			Patch:         patch,
			ChampionIDs:   fallbackChampionIDs,
			CanonicalOnly: true,
			Concurrency:   championPageStartupPrewarmConcurrency,
			RankBucket:    cfg.ChampionPagePrewarmRankBucket,
			QueueID:       analytics.RankedSoloQueueID,
		})
		if prewarmErr != nil {
			hadErrors = true
			log.Printf(
				"champion page canonical startup prewarm completed with errors patch=%s queue=%d candidates=%d stored=%d cached=%d skipped=%d errors=%d duration=%s err=%v",
				patch,
				analytics.RankedSoloQueueID,
				result.Candidates,
				result.Stored,
				result.Cached,
				result.Skipped,
				result.Errors,
				time.Since(startedAt).Round(time.Millisecond),
				prewarmErr,
			)
			continue
		}
		if index == 0 {
			fallbackChampionIDs = append([]uint16(nil), result.MissingGuideIDs...)
		}
		log.Printf(
			"champion page canonical startup prewarm complete patch=%s queue=%d candidates=%d stored=%d cached=%d skipped=%d errors=%d duration=%s scope=%s missing_guide_champions=%d",
			patch,
			analytics.RankedSoloQueueID,
			result.Candidates,
			result.Stored,
			result.Cached,
			result.Skipped,
			result.Errors,
			time.Since(startedAt).Round(time.Millisecond),
			scope,
			len(result.MissingGuideIDs),
		)
	}
	if hadErrors {
		return
	}
	log.Printf("champion page canonical startup readiness grace start duration=%s", championPageStartupReadinessGrace)
	timer := time.NewTimer(championPageStartupReadinessGrace)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
		log.Printf("champion page canonical startup readiness grace complete duration=%s", championPageStartupReadinessGrace)
	}
}

func championPageStartupPrewarmPatchList(currentPatch string, retention int, stats []clickhouse.PatchStat) []string {
	currentPatch = strings.TrimSpace(currentPatch)
	if currentPatch == "" {
		return nil
	}
	var newestPrevious string
	var maturePrevious string
	for _, stat := range stats {
		patch := strings.TrimSpace(stat.Patch)
		if patch == "" || stat.Matches == 0 || compareChampionPagePatches(patch, currentPatch) >= 0 {
			continue
		}
		if newestPrevious == "" || compareChampionPagePatches(patch, newestPrevious) > 0 {
			newestPrevious = patch
		}
		if stat.Matches >= 5000 && (maturePrevious == "" || compareChampionPagePatches(patch, maturePrevious) > 0) {
			maturePrevious = patch
		}
	}
	fallbackPatch := maturePrevious
	if fallbackPatch == "" {
		fallbackPatch = newestPrevious
	}
	if fallbackPatch == "" {
		window := analytics.PatchWindow(currentPatch, retention)
		if len(window) > 1 {
			fallbackPatch = window[1]
		}
	}
	patches := []string{currentPatch}
	if fallbackPatch != "" && fallbackPatch != currentPatch {
		patches = append(patches, fallbackPatch)
	}
	return patches
}

func compareChampionPagePatches(left, right string) int {
	parse := func(value string) (int, int, bool) {
		parts := strings.Split(strings.TrimSpace(value), ".")
		if len(parts) < 2 {
			return 0, 0, false
		}
		major, majorErr := strconv.Atoi(parts[0])
		minor, minorErr := strconv.Atoi(parts[1])
		return major, minor, majorErr == nil && minorErr == nil
	}
	leftMajor, leftMinor, leftOK := parse(left)
	rightMajor, rightMinor, rightOK := parse(right)
	if !leftOK || !rightOK {
		return strings.Compare(strings.TrimSpace(left), strings.TrimSpace(right))
	}
	if leftMajor != rightMajor {
		if leftMajor < rightMajor {
			return -1
		}
		return 1
	}
	if leftMinor < rightMinor {
		return -1
	}
	if leftMinor > rightMinor {
		return 1
	}
	return 0
}

func maybeRefreshItemSlotAnalytics(ctx context.Context, cfg config.Config, staticService *staticdata.Service, repo *clickhouse.Repository, refreshStatus *refreshStatusRecorder, lastRefresh *time.Time) bool {
	if !cfg.ItemSlotRefreshEnabled {
		return false
	}
	patch := strings.TrimSpace(cfg.CollectorCurrentPatch)
	if patch == "" {
		return false
	}
	interval := cfg.ItemSlotRefreshInterval
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	if !lastRefresh.IsZero() && time.Since(*lastRefresh) < interval {
		return false
	}
	startedAt := refreshStatus.start("item-slot-analytics", patch, analytics.RankedSoloQueueID, "item slots and starting loadouts")
	log.Printf("item slot analytics scheduled refresh start patch=%s queue=%d interval=%s", patch, analytics.RankedSoloQueueID, interval)
	defer func() {
		*lastRefresh = time.Now()
	}()
	contexts, err := itemSlotRefreshContexts(ctx, staticService)
	if err != nil {
		log.Printf("item slot analytics scheduled refresh skipped patch=%s err=%v", patch, err)
		refreshStatus.fail("item-slot-analytics", startedAt, err)
		return true
	}
	if err := repo.RefreshItemSlotAnalytics(ctx, patch, analytics.RankedSoloQueueID, contexts); err != nil {
		log.Printf("item slot analytics scheduled refresh failed patch=%s queue=%d contexts=%d duration=%s err=%v", patch, analytics.RankedSoloQueueID, len(contexts), time.Since(startedAt).Round(time.Millisecond), err)
		refreshStatus.fail("item-slot-analytics", startedAt, err)
		return true
	}
	loadoutContexts, err := startingLoadoutRefreshContexts(ctx, staticService)
	if err != nil {
		log.Printf("starting loadout analytics scheduled refresh skipped patch=%s err=%v", patch, err)
		refreshStatus.fail("item-slot-analytics", startedAt, err)
		return true
	}
	if err := repo.RefreshStartingLoadoutAnalytics(ctx, patch, analytics.RankedSoloQueueID, loadoutContexts); err != nil {
		log.Printf("starting loadout analytics scheduled refresh failed patch=%s queue=%d contexts=%d duration=%s err=%v", patch, analytics.RankedSoloQueueID, len(loadoutContexts), time.Since(startedAt).Round(time.Millisecond), err)
		refreshStatus.fail("item-slot-analytics", startedAt, err)
		return true
	}
	log.Printf("item slot analytics scheduled refresh complete patch=%s queue=%d item_contexts=%d loadout_contexts=%d duration=%s", patch, analytics.RankedSoloQueueID, len(contexts), len(loadoutContexts), time.Since(startedAt).Round(time.Millisecond))
	refreshStatus.succeed("item-slot-analytics", startedAt, map[string]int{"itemContexts": len(contexts), "loadoutContexts": len(loadoutContexts)})
	return true
}

func itemSlotRefreshContexts(ctx context.Context, staticService *staticdata.Service) ([]clickhouse.ItemSlotAnalyticsContext, error) {
	defaultItems, err := staticService.BuildItemIDs(ctx, "", false, false)
	if err != nil {
		return nil, err
	}
	defaultStartingItems, err := staticService.StartingItemIDs(ctx, "", false, false)
	if err != nil {
		return nil, err
	}
	jungleItems, err := staticService.BuildItemIDs(ctx, "", true, false)
	if err != nil {
		return nil, err
	}
	jungleStartingItems, err := staticService.StartingItemIDs(ctx, "", true, false)
	if err != nil {
		return nil, err
	}
	supportItems, err := staticService.BuildItemIDs(ctx, "", false, true)
	if err != nil {
		return nil, err
	}
	supportStartingItems, err := staticService.StartingItemIDs(ctx, "", false, true)
	if err != nil {
		return nil, err
	}
	return []clickhouse.ItemSlotAnalyticsContext{
		{Key: "DEFAULT", ItemIDs: defaultItems, StartingItemIDs: defaultStartingItems},
		{Key: "JUNGLE", ItemIDs: jungleItems, StartingItemIDs: jungleStartingItems},
		{Key: "SUPPORT", ItemIDs: supportItems, StartingItemIDs: supportStartingItems},
	}, nil
}

func startingLoadoutRefreshContexts(ctx context.Context, staticService *staticdata.Service) ([]clickhouse.StartingLoadoutAnalyticsContext, error) {
	defaultCosts, err := staticService.OpeningItemCosts(ctx, "", false, false)
	if err != nil {
		return nil, err
	}
	jungleCosts, err := staticService.OpeningItemCosts(ctx, "", true, false)
	if err != nil {
		return nil, err
	}
	supportCosts, err := staticService.OpeningItemCosts(ctx, "", false, true)
	if err != nil {
		return nil, err
	}
	return []clickhouse.StartingLoadoutAnalyticsContext{
		{Key: "DEFAULT", OpeningItemCosts: defaultCosts},
		{Key: "JUNGLE", OpeningItemCosts: jungleCosts},
		{Key: "SUPPORT", OpeningItemCosts: supportCosts},
	}, nil
}

func maybeRefreshChampionGuideAnalytics(ctx context.Context, cfg config.Config, apiServer api.Server, repo *clickhouse.Repository, refreshStatus *refreshStatusRecorder, lastRefresh *time.Time) bool {
	if !cfg.ChampionGuideRefreshEnabled {
		return false
	}
	patch := strings.TrimSpace(cfg.CollectorCurrentPatch)
	if patch == "" {
		return false
	}
	patches := analytics.PatchWindow(patch, cfg.CollectorPatchRetention)
	if len(patches) == 0 {
		patches = []string{patch}
	}
	interval := cfg.ChampionGuideRefreshInterval
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	if !lastRefresh.IsZero() && time.Since(*lastRefresh) < interval {
		return false
	}
	patchLabel := strings.Join(patches, ",")
	startedAt := refreshStatus.start("champion-guide-analytics", patchLabel, analytics.RankedSoloQueueID, "champion guide read models")
	log.Printf("champion guide analytics scheduled refresh start patches=%s queue=%d interval=%s", patchLabel, analytics.RankedSoloQueueID, interval)
	defer func() {
		*lastRefresh = time.Now()
	}()
	statusDetails := map[string]int{"patches": len(patches)}
	var firstErr error
	for _, refreshPatch := range patches {
		refreshStartedAt := time.Now()
		if err := repo.RefreshChampionGuideDerivedAnalytics(ctx, refreshPatch, analytics.RankedSoloQueueID); err != nil {
			log.Printf(
				"champion guide analytics scheduled refresh failed patch=%s queue=%d duration=%s err=%v",
				refreshPatch,
				analytics.RankedSoloQueueID,
				time.Since(refreshStartedAt).Round(time.Millisecond),
				err,
			)
			statusDetails["refreshErrors"]++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		statusDetails["refreshedPatches"]++
	}
	if cfg.ChampionPagePrewarmEnabled {
		prewarmPatches := append([]string(nil), patches...)
		stats, err := repo.PatchStats(ctx, analytics.RankedSoloQueueID)
		if err != nil {
			statusDetails["prewarmPatchDiscoveryErrors"]++
			if firstErr == nil {
				firstErr = err
			}
			log.Printf("champion page prewarm patch discovery failed err=%v", err)
		} else {
			prewarmPatches = championPagePrewarmPatchList(patches, stats)
		}
		statusDetails["prewarmPatches"] = len(prewarmPatches)
		for _, prewarmPatch := range prewarmPatches {
			matchupsPerChampion := cfg.ChampionPagePrewarmMatchupsPerChampion
			maxMatchupBundles := cfg.ChampionPagePrewarmMaxMatchupBundles
			if !analytics.PatchInWindow(prewarmPatch, patch, cfg.CollectorPatchRetention) {
				matchupsPerChampion = 0
				maxMatchupBundles = 0
			}
			prewarmStartedAt := time.Now()
			log.Printf(
				"champion page prewarm start patch=%s queue=%d roles=%s per_role=%d min_games=%d matchup_per_champion=%d matchup_min_games=%d matchup_max_bundles=%d rank_bucket=%s",
				prewarmPatch,
				analytics.RankedSoloQueueID,
				strings.Join(cfg.ChampionPagePrewarmRoles, ","),
				cfg.ChampionPagePrewarmPerRole,
				cfg.ChampionPagePrewarmMinGames,
				matchupsPerChampion,
				cfg.ChampionPagePrewarmMatchupMinGames,
				maxMatchupBundles,
				cfg.ChampionPagePrewarmRankBucket,
			)
			result, err := apiServer.PrewarmChampionPageBundles(ctx, api.ChampionPagePrewarmOptions{
				Patch:               prewarmPatch,
				Roles:               cfg.ChampionPagePrewarmRoles,
				PerRole:             cfg.ChampionPagePrewarmPerRole,
				MinGames:            cfg.ChampionPagePrewarmMinGames,
				Concurrency:         championPageStartupPrewarmConcurrency,
				MatchupsPerChampion: matchupsPerChampion,
				MatchupMinGames:     cfg.ChampionPagePrewarmMatchupMinGames,
				MaxMatchupBundles:   maxMatchupBundles,
				RankBucket:          cfg.ChampionPagePrewarmRankBucket,
				QueueID:             analytics.RankedSoloQueueID,
			})
			statusDetails["prewarmCandidates"] += result.Candidates
			statusDetails["prewarmStored"] += result.Stored
			statusDetails["prewarmSkipped"] += result.Skipped
			statusDetails["prewarmCached"] += result.Cached
			statusDetails["prewarmErrors"] += result.Errors
			statusDetails["prewarmMatchupCandidates"] += result.MatchupCandidates
			statusDetails["prewarmMatchupStored"] += result.MatchupStored
			statusDetails["prewarmMatchupSkipped"] += result.MatchupSkipped
			statusDetails["prewarmMatchupCached"] += result.MatchupCached
			if err != nil {
				log.Printf(
					"champion page prewarm completed with errors patch=%s queue=%d candidates=%d stored=%d cached=%d skipped=%d matchup_candidates=%d matchup_stored=%d matchup_cached=%d matchup_skipped=%d errors=%d duration=%s err=%v",
					prewarmPatch,
					analytics.RankedSoloQueueID,
					result.Candidates,
					result.Stored,
					result.Cached,
					result.Skipped,
					result.MatchupCandidates,
					result.MatchupStored,
					result.MatchupCached,
					result.MatchupSkipped,
					result.Errors,
					time.Since(prewarmStartedAt).Round(time.Millisecond),
					err,
				)
				if firstErr == nil {
					firstErr = err
				}
			} else {
				log.Printf(
					"champion page prewarm complete patch=%s queue=%d candidates=%d stored=%d cached=%d skipped=%d matchup_candidates=%d matchup_stored=%d matchup_cached=%d matchup_skipped=%d errors=%d duration=%s",
					prewarmPatch,
					analytics.RankedSoloQueueID,
					result.Candidates,
					result.Stored,
					result.Cached,
					result.Skipped,
					result.MatchupCandidates,
					result.MatchupStored,
					result.MatchupCached,
					result.MatchupSkipped,
					result.Errors,
					time.Since(prewarmStartedAt).Round(time.Millisecond),
				)
			}
		}
	}
	if firstErr != nil {
		log.Printf("champion guide analytics scheduled refresh completed with errors patches=%s queue=%d duration=%s err=%v", patchLabel, analytics.RankedSoloQueueID, time.Since(startedAt).Round(time.Millisecond), firstErr)
		refreshStatus.fail("champion-guide-analytics", startedAt, firstErr)
		return true
	}
	log.Printf("champion guide analytics scheduled refresh complete patches=%s queue=%d duration=%s", patchLabel, analytics.RankedSoloQueueID, time.Since(startedAt).Round(time.Millisecond))
	refreshStatus.succeed("champion-guide-analytics", startedAt, statusDetails)
	return true
}

func championPagePrewarmPatchList(retained []string, stats []clickhouse.PatchStat) []string {
	seen := map[string]bool{}
	patches := make([]string, 0, len(retained)+len(stats))
	appendPatch := func(patch string) {
		patch = strings.TrimSpace(patch)
		if patch == "" || seen[patch] {
			return
		}
		seen[patch] = true
		patches = append(patches, patch)
	}
	for _, patch := range retained {
		appendPatch(patch)
	}
	for _, stat := range stats {
		appendPatch(stat.Patch)
	}
	return patches
}

func maybeRefreshWinConditionAnalytics(ctx context.Context, cfg config.Config, repo *clickhouse.Repository, refreshStatus *refreshStatusRecorder, platforms []string, lastRefresh *time.Time) bool {
	if !cfg.WinConditionRefreshEnabled {
		return false
	}
	patch := strings.TrimSpace(cfg.CollectorCurrentPatch)
	if patch == "" {
		return false
	}
	interval := cfg.WinConditionRefreshInterval
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	if !lastRefresh.IsZero() && time.Since(*lastRefresh) < interval {
		return false
	}
	startedAt := refreshStatus.start("win-condition-analytics", patch, analytics.RankedSoloQueueID, "win condition read models")
	log.Printf("win condition analytics scheduled refresh start patch=%s queue=%d platforms=%d interval=%s", patch, analytics.RankedSoloQueueID, len(platforms), interval)
	defer func() {
		*lastRefresh = time.Now()
	}()
	if err := repo.RefreshWinConditionMetricsForPlatforms(ctx, patch, platforms, analytics.RankedSoloQueueID); err != nil {
		log.Printf("win condition analytics scheduled refresh failed patch=%s queue=%d platforms=%d duration=%s err=%v", patch, analytics.RankedSoloQueueID, len(platforms), time.Since(startedAt).Round(time.Millisecond), err)
		refreshStatus.fail("win-condition-analytics", startedAt, err)
		return true
	}
	log.Printf("win condition analytics scheduled refresh complete patch=%s queue=%d platforms=%d duration=%s", patch, analytics.RankedSoloQueueID, len(platforms), time.Since(startedAt).Round(time.Millisecond))
	refreshStatus.succeed("win-condition-analytics", startedAt, map[string]int{"platforms": len(platforms)})
	return true
}

func maybeRefreshSummonerProfileAnalytics(ctx context.Context, cfg config.Config, repo *clickhouse.Repository, refreshStatus *refreshStatusRecorder, lastRefresh *time.Time) bool {
	if !cfg.SummonerProfileRefreshEnabled {
		return false
	}
	interval := cfg.SummonerProfileRefreshInterval
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	if !lastRefresh.IsZero() && time.Since(*lastRefresh) < interval {
		return false
	}
	startedAt := refreshStatus.start("summoner-profile-analytics", "", analytics.RankedSoloQueueID, "summoner profile read models")
	log.Printf("summoner profile analytics scheduled refresh start queue=%d interval=%s", analytics.RankedSoloQueueID, interval)
	defer func() {
		*lastRefresh = time.Now()
	}()
	result, err := repo.RefreshSummonerProfileAnalytics(ctx, analytics.RankedSoloQueueID)
	if err != nil {
		log.Printf("summoner profile analytics scheduled refresh failed queue=%d duration=%s err=%v", analytics.RankedSoloQueueID, time.Since(startedAt).Round(time.Millisecond), err)
		refreshStatus.fail("summoner-profile-analytics", startedAt, err)
		return true
	}
	log.Printf("summoner profile analytics scheduled refresh complete queue=%d identity_rows=%d profile_rows=%d champion_rows=%d champion_role_rows=%d recent_match_rows=%d build_rows=%d duration=%s", analytics.RankedSoloQueueID, result.IdentityRows, result.ProfileRows, result.ChampionRows, result.ChampionRoleRows, result.RecentMatchRows, result.BuildRows, time.Since(startedAt).Round(time.Millisecond))
	refreshStatus.succeed("summoner-profile-analytics", startedAt, map[string]int{
		"identityRows":     result.IdentityRows,
		"profileRows":      result.ProfileRows,
		"championRows":     result.ChampionRows,
		"championRoleRows": result.ChampionRoleRows,
		"recentMatchRows":  result.RecentMatchRows,
		"buildRows":        result.BuildRows,
	})
	return true
}
