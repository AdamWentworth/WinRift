package main

import (
	"context"
	"log"
	"strings"
	"time"

	"winrift/core/internal/analytics"
	"winrift/core/internal/api"
	"winrift/core/internal/clickhouse"
	"winrift/core/internal/config"
	"winrift/core/internal/runstate"
	"winrift/core/internal/staticdata"
)

const (
	refreshSchedulerPollInterval = time.Minute
	refreshSchedulerLanePause    = 5 * time.Second
)

type refreshStatusRecorder struct {
	path     string
	statuses map[string]runstate.RefreshStatus
}

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

func startRefreshScheduler(ctx context.Context, cfg config.Config, staticService *staticdata.Service, apiServer api.Server, repo *clickhouse.Repository, platforms []string, firstSweepComplete <-chan struct{}) {
	go func() {
		log.Printf("analytics refresh scheduler waiting for first collector sweep")
		select {
		case <-ctx.Done():
			log.Printf("analytics refresh scheduler stopped before first sweep err=%v", ctx.Err())
			return
		case <-firstSweepComplete:
			log.Printf("analytics refresh scheduler starting after first collector sweep")
		}

		var lastItemSlotRefresh time.Time
		var lastChampionGuideRefresh time.Time
		var lastWinConditionRefresh time.Time
		var lastSummonerProfileRefresh time.Time
		refreshStatus := newRefreshStatusRecorder(cfg.WorkerRefreshStatusPath)
		for {
			runRefreshSchedulerPass(ctx, cfg, staticService, apiServer, repo, refreshStatus, platforms, &lastItemSlotRefresh, &lastChampionGuideRefresh, &lastWinConditionRefresh, &lastSummonerProfileRefresh)
			select {
			case <-ctx.Done():
				log.Printf("analytics refresh scheduler stopped err=%v", ctx.Err())
				return
			case <-time.After(refreshSchedulerPollInterval):
			}
		}
	}()
}

func runRefreshSchedulerPass(ctx context.Context, cfg config.Config, staticService *staticdata.Service, apiServer api.Server, repo *clickhouse.Repository, refreshStatus *refreshStatusRecorder, platforms []string, lastItemSlotRefresh, lastChampionGuideRefresh, lastWinConditionRefresh, lastSummonerProfileRefresh *time.Time) {
	maybeRefreshItemSlotAnalytics(ctx, cfg, staticService, repo, refreshStatus, lastItemSlotRefresh)
	pauseRefreshLane(ctx)
	maybeRefreshChampionGuideAnalytics(ctx, cfg, apiServer, repo, refreshStatus, lastChampionGuideRefresh)
	pauseRefreshLane(ctx)
	maybeRefreshWinConditionAnalytics(ctx, cfg, repo, refreshStatus, platforms, lastWinConditionRefresh)
	pauseRefreshLane(ctx)
	maybeRefreshSummonerProfileAnalytics(ctx, cfg, repo, refreshStatus, lastSummonerProfileRefresh)
}

func pauseRefreshLane(ctx context.Context) {
	if refreshSchedulerLanePause <= 0 {
		return
	}
	timer := time.NewTimer(refreshSchedulerLanePause)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func maybeRefreshItemSlotAnalytics(ctx context.Context, cfg config.Config, staticService *staticdata.Service, repo *clickhouse.Repository, refreshStatus *refreshStatusRecorder, lastRefresh *time.Time) {
	if !cfg.ItemSlotRefreshEnabled {
		return
	}
	patch := strings.TrimSpace(cfg.CollectorCurrentPatch)
	if patch == "" {
		return
	}
	interval := cfg.ItemSlotRefreshInterval
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	if !lastRefresh.IsZero() && time.Since(*lastRefresh) < interval {
		return
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
		return
	}
	if err := repo.RefreshItemSlotAnalytics(ctx, patch, analytics.RankedSoloQueueID, contexts); err != nil {
		log.Printf("item slot analytics scheduled refresh failed patch=%s queue=%d contexts=%d duration=%s err=%v", patch, analytics.RankedSoloQueueID, len(contexts), time.Since(startedAt).Round(time.Millisecond), err)
		refreshStatus.fail("item-slot-analytics", startedAt, err)
		return
	}
	loadoutContexts, err := startingLoadoutRefreshContexts(ctx, staticService)
	if err != nil {
		log.Printf("starting loadout analytics scheduled refresh skipped patch=%s err=%v", patch, err)
		refreshStatus.fail("item-slot-analytics", startedAt, err)
		return
	}
	if err := repo.RefreshStartingLoadoutAnalytics(ctx, patch, analytics.RankedSoloQueueID, loadoutContexts); err != nil {
		log.Printf("starting loadout analytics scheduled refresh failed patch=%s queue=%d contexts=%d duration=%s err=%v", patch, analytics.RankedSoloQueueID, len(loadoutContexts), time.Since(startedAt).Round(time.Millisecond), err)
		refreshStatus.fail("item-slot-analytics", startedAt, err)
		return
	}
	log.Printf("item slot analytics scheduled refresh complete patch=%s queue=%d item_contexts=%d loadout_contexts=%d duration=%s", patch, analytics.RankedSoloQueueID, len(contexts), len(loadoutContexts), time.Since(startedAt).Round(time.Millisecond))
	refreshStatus.succeed("item-slot-analytics", startedAt, map[string]int{"itemContexts": len(contexts), "loadoutContexts": len(loadoutContexts)})
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

func maybeRefreshChampionGuideAnalytics(ctx context.Context, cfg config.Config, apiServer api.Server, repo *clickhouse.Repository, refreshStatus *refreshStatusRecorder, lastRefresh *time.Time) {
	if !cfg.ChampionGuideRefreshEnabled {
		return
	}
	patch := strings.TrimSpace(cfg.CollectorCurrentPatch)
	if patch == "" {
		return
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
		return
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
		if cfg.ChampionPagePrewarmEnabled {
			prewarmStartedAt := time.Now()
			log.Printf(
				"champion page prewarm start patch=%s queue=%d roles=%s per_role=%d min_games=%d matchup_per_champion=%d matchup_min_games=%d matchup_max_bundles=%d rank_bucket=%s",
				refreshPatch,
				analytics.RankedSoloQueueID,
				strings.Join(cfg.ChampionPagePrewarmRoles, ","),
				cfg.ChampionPagePrewarmPerRole,
				cfg.ChampionPagePrewarmMinGames,
				cfg.ChampionPagePrewarmMatchupsPerChampion,
				cfg.ChampionPagePrewarmMatchupMinGames,
				cfg.ChampionPagePrewarmMaxMatchupBundles,
				cfg.ChampionPagePrewarmRankBucket,
			)
			result, err := apiServer.PrewarmChampionPageBundles(ctx, api.ChampionPagePrewarmOptions{
				Patch:               refreshPatch,
				Roles:               cfg.ChampionPagePrewarmRoles,
				PerRole:             cfg.ChampionPagePrewarmPerRole,
				MinGames:            cfg.ChampionPagePrewarmMinGames,
				MatchupsPerChampion: cfg.ChampionPagePrewarmMatchupsPerChampion,
				MatchupMinGames:     cfg.ChampionPagePrewarmMatchupMinGames,
				MaxMatchupBundles:   cfg.ChampionPagePrewarmMaxMatchupBundles,
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
					refreshPatch,
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
					refreshPatch,
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
		return
	}
	log.Printf("champion guide analytics scheduled refresh complete patches=%s queue=%d duration=%s", patchLabel, analytics.RankedSoloQueueID, time.Since(startedAt).Round(time.Millisecond))
	refreshStatus.succeed("champion-guide-analytics", startedAt, statusDetails)
}

func maybeRefreshWinConditionAnalytics(ctx context.Context, cfg config.Config, repo *clickhouse.Repository, refreshStatus *refreshStatusRecorder, platforms []string, lastRefresh *time.Time) {
	if !cfg.WinConditionRefreshEnabled {
		return
	}
	patch := strings.TrimSpace(cfg.CollectorCurrentPatch)
	if patch == "" {
		return
	}
	interval := cfg.WinConditionRefreshInterval
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	if !lastRefresh.IsZero() && time.Since(*lastRefresh) < interval {
		return
	}
	startedAt := refreshStatus.start("win-condition-analytics", patch, analytics.RankedSoloQueueID, "win condition read models")
	log.Printf("win condition analytics scheduled refresh start patch=%s queue=%d platforms=%d interval=%s", patch, analytics.RankedSoloQueueID, len(platforms), interval)
	defer func() {
		*lastRefresh = time.Now()
	}()
	if err := repo.RefreshWinConditionMetricsForPlatforms(ctx, patch, platforms, analytics.RankedSoloQueueID); err != nil {
		log.Printf("win condition analytics scheduled refresh failed patch=%s queue=%d platforms=%d duration=%s err=%v", patch, analytics.RankedSoloQueueID, len(platforms), time.Since(startedAt).Round(time.Millisecond), err)
		refreshStatus.fail("win-condition-analytics", startedAt, err)
		return
	}
	log.Printf("win condition analytics scheduled refresh complete patch=%s queue=%d platforms=%d duration=%s", patch, analytics.RankedSoloQueueID, len(platforms), time.Since(startedAt).Round(time.Millisecond))
	refreshStatus.succeed("win-condition-analytics", startedAt, map[string]int{"platforms": len(platforms)})
}

func maybeRefreshSummonerProfileAnalytics(ctx context.Context, cfg config.Config, repo *clickhouse.Repository, refreshStatus *refreshStatusRecorder, lastRefresh *time.Time) {
	if !cfg.SummonerProfileRefreshEnabled {
		return
	}
	interval := cfg.SummonerProfileRefreshInterval
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	if !lastRefresh.IsZero() && time.Since(*lastRefresh) < interval {
		return
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
		return
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
}
