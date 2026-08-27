package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"winrift/core/internal/analytics"
	"winrift/core/internal/clickhouse"
)

type ChampionPagePrewarmOptions struct {
	Patch               string
	ChampionIDs         []uint16
	Roles               []string
	PerRole             int
	MinGames            int
	CanonicalOnly       bool
	Concurrency         int
	MatchupsPerChampion int
	MatchupMinGames     int
	MaxMatchupBundles   int
	RankBucket          string
	QueueID             uint16
}

type ChampionPagePrewarmResult struct {
	Candidates        int
	Stored            int
	Skipped           int
	Cached            int
	Errors            int
	MatchupCandidates int
	MatchupStored     int
	MatchupSkipped    int
	MatchupCached     int
	MissingGuideIDs   []uint16
}

type ChampionPageHydrationResult struct {
	Candidates      int
	Loaded          int
	Missing         int
	MissingGuideIDs []uint16
}

// ClearChampionPageBundleMemoryCache releases worker-side build bodies after
// they have been persisted. The API runs in a separate process and hydrates its
// own long-lived response cache, so the prewarm worker does not need to retain
// every archived payload.
func (s Server) ClearChampionPageBundleMemoryCache() {
	s.responseCache.deletePrefix(championPageBundleCacheKeyPrefix)
}

func (s Server) PrewarmChampionPageBundles(ctx context.Context, options ChampionPagePrewarmOptions) (ChampionPagePrewarmResult, error) {
	patch := strings.TrimSpace(options.Patch)
	if patch == "" {
		patch = strings.TrimSpace(s.cfg.CollectorCurrentPatch)
	}
	if patch == "" {
		return ChampionPagePrewarmResult{}, nil
	}
	roles := championPagePrewarmRoles(options.Roles)
	perRole := options.PerRole
	if perRole <= 0 {
		perRole = 200
	}
	minGames := options.MinGames
	if minGames <= 0 {
		minGames = 50
	}
	matchupsPerChampion := options.MatchupsPerChampion
	if matchupsPerChampion < 0 {
		matchupsPerChampion = 0
	}
	matchupMinGames := options.MatchupMinGames
	if matchupMinGames <= 0 {
		matchupMinGames = 15
	}
	maxMatchupBundles := options.MaxMatchupBundles
	if maxMatchupBundles < 0 {
		maxMatchupBundles = 0
	}
	rankBucket := strings.ToUpper(strings.TrimSpace(options.RankBucket))
	queueID := options.QueueID
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}

	result := ChampionPagePrewarmResult{}
	var firstErr error
	seenKeys := map[string]bool{}
	canonicalRequests, err := s.championPageCanonicalRequests(ctx, patch, rankBucket, queueID, perRole, options.ChampionIDs)
	if err != nil {
		result.Errors++
		firstErr = err
	}
	canonicalResult, canonicalErr := s.prewarmChampionPageRequests(ctx, canonicalRequests, options.Concurrency, false, true)
	mergeChampionPagePrewarmResult(&result, canonicalResult)
	if canonicalErr != nil && firstErr == nil {
		firstErr = canonicalErr
	}
	if options.CanonicalOnly {
		return result, firstErr
	}
	for _, role := range roles {
		filters := map[string]string{
			"role":        role,
			"patch":       patch,
			"rank_bucket": rankBucket,
		}
		index, err := s.repo.QueryChampionGuideIndex(ctx, filters, minGames, perRole)
		if err != nil {
			result.Errors++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, summary := range index.Results {
			if summary.ChampionID == 0 {
				continue
			}
			summaryRole := strings.ToUpper(strings.TrimSpace(summary.Role))
			if summaryRole == "" {
				summaryRole = role
			}
			request := championPagePrewarmRequest(summary.ChampionID, summaryRole, patch, rankBucket, queueID)
			if err := s.prewarmChampionPageBundle(ctx, request, seenKeys, false, &result); err != nil {
				result.Errors++
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if matchupsPerChampion <= 0 || (maxMatchupBundles > 0 && result.MatchupCandidates >= maxMatchupBundles) {
				continue
			}
			matchupFilters := map[string]string{
				"champion_id": strconv.Itoa(int(summary.ChampionID)),
				"role":        summaryRole,
				"patch":       patch,
				"rank_bucket": rankBucket,
			}
			matchups, err := s.repo.QueryCommonChampionGuideMatchups(ctx, matchupFilters, matchupMinGames, matchupsPerChampion)
			if err != nil {
				result.Errors++
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			for _, matchup := range matchups {
				if matchup.OpponentChampionID == 0 {
					continue
				}
				if maxMatchupBundles > 0 && result.MatchupCandidates >= maxMatchupBundles {
					break
				}
				matchupRequest := request
				matchupRequest.Build.OpponentChampionID = matchup.OpponentChampionID
				matchupRequest.Build.MinGames = 3
				if err := s.prewarmChampionPageBundle(ctx, matchupRequest, seenKeys, true, &result); err != nil {
					result.Errors++
					if firstErr == nil {
						firstErr = err
					}
				}
			}
		}
	}
	return result, firstErr
}

func (s Server) championPageCanonicalRequests(ctx context.Context, patch, rankBucket string, queueID uint16, perRole int, requestedChampionIDs []uint16) ([]championPageBundleRequest, error) {
	canonicalFilters := map[string]string{
		"role":        "",
		"patch":       patch,
		"rank_bucket": rankBucket,
	}
	canonicalIndex, err := s.repo.QueryChampionGuideIndex(ctx, canonicalFilters, 1, max(perRole, 250))
	if err != nil {
		return nil, err
	}
	allPatchIndex := canonicalIndex
	if patch != "" {
		allPatchFilters := map[string]string{
			"role":        "",
			"patch":       "",
			"rank_bucket": rankBucket,
		}
		allPatchIndex, err = s.repo.QueryChampionGuideIndex(ctx, allPatchFilters, 1, max(perRole, 250))
		if err != nil {
			return nil, err
		}
	}
	championIDs := championPagePrewarmChampionIDs(canonicalIndex.Results, allPatchIndex.Results)
	championIDs = filterChampionPagePrewarmChampionIDs(championIDs, requestedChampionIDs)
	roleRates, err := s.repo.ChampionRoleRatesForPatch(ctx, championIDs, queueID, patch)
	if err != nil {
		return nil, err
	}
	fallbackRoleRates := roleRates
	if patch != "" {
		fallbackRoleRates, err = s.repo.ChampionRoleRatesForPatch(ctx, championIDs, queueID, "")
		if err != nil {
			return nil, err
		}
	}

	requests := make([]championPageBundleRequest, 0, len(championIDs))
	seenKeys := make(map[string]bool, len(championIDs))
	for _, championID := range championIDs {
		role := primaryChampionRole(roleRates, championID)
		if role == "" {
			role = primaryChampionRole(fallbackRoleRates, championID)
		}
		if role == "" {
			continue
		}
		request := championPagePrewarmRequest(championID, role, patch, rankBucket, queueID)
		cacheKey := championPageBundleCacheKey(request)
		if seenKeys[cacheKey] {
			continue
		}
		seenKeys[cacheKey] = true
		requests = append(requests, request)
	}
	return requests, nil
}

func (s Server) HydrateChampionPageBundles(ctx context.Context, options ChampionPagePrewarmOptions) (ChampionPageHydrationResult, error) {
	patch := strings.TrimSpace(options.Patch)
	if patch == "" {
		patch = strings.TrimSpace(s.cfg.CollectorCurrentPatch)
	}
	if patch == "" {
		return ChampionPageHydrationResult{}, nil
	}
	perRole := options.PerRole
	if perRole <= 0 {
		perRole = 200
	}
	rankBucket := strings.ToUpper(strings.TrimSpace(options.RankBucket))
	queueID := options.QueueID
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	requests, err := s.championPageCanonicalRequests(ctx, patch, rankBucket, queueID, perRole, options.ChampionIDs)
	if err != nil {
		return ChampionPageHydrationResult{}, err
	}
	return s.hydrateChampionPageRequests(ctx, requests, s.repo.CachedChampionPageBundles)
}

func (s Server) hydrateChampionPageRequests(
	ctx context.Context,
	requests []championPageBundleRequest,
	load func(context.Context, []string) (map[string]clickhouse.ChampionPageBundleCacheEntry, error),
) (ChampionPageHydrationResult, error) {
	result := ChampionPageHydrationResult{Candidates: len(requests)}
	keys := make([]string, 0, len(requests))
	for _, request := range requests {
		keys = append(keys, championPageBundleCacheKey(request))
	}
	entries, err := load(ctx, keys)
	if err != nil {
		return result, err
	}
	for _, request := range requests {
		cacheKey := championPageBundleCacheKey(request)
		entry, ok := entries[cacheKey]
		if !ok || len(entry.Body) == 0 || !entry.ExpiresAt.After(time.Now()) {
			result.Missing++
			continue
		}
		ttl := s.championPageBundleTTL(request.Build.Patch)
		s.responseCache.setShared(cacheKey, entry.Body, ttl)
		s.responseCache.setShared(championPageBundleCacheKey(championPageCanonicalAliasRequest(request)), entry.Body, ttl)
		guideGames, err := championPageGuideGames(entry.Body)
		if err != nil {
			return result, fmt.Errorf("decode hydrated champion page champion=%d patch=%s: %w", request.Build.ChampionID, request.Build.Patch, err)
		}
		if guideGames == 0 {
			result.MissingGuideIDs = append(result.MissingGuideIDs, request.Build.ChampionID)
		}
		result.Loaded++
	}
	return result, nil
}

func (s Server) prewarmChampionPageRequests(ctx context.Context, requests []championPageBundleRequest, concurrency int, matchup, storeCanonicalAlias bool) (ChampionPagePrewarmResult, error) {
	if len(requests) == 0 {
		return ChampionPagePrewarmResult{}, nil
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(requests) {
		concurrency = len(requests)
	}
	type outcome struct {
		result ChampionPagePrewarmResult
		err    error
	}
	jobs := make(chan championPageBundleRequest)
	outcomes := make(chan outcome, len(requests))
	var workers sync.WaitGroup
	for range concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for request := range jobs {
				localResult := ChampionPagePrewarmResult{}
				err := s.prewarmChampionPageBundle(ctx, request, map[string]bool{}, matchup, &localResult)
				if err == nil && storeCanonicalAlias {
					var guideGames int
					guideGames, err = s.storeChampionPageCanonicalAlias(ctx, request)
					if err == nil && guideGames == 0 {
						localResult.MissingGuideIDs = append(localResult.MissingGuideIDs, request.Build.ChampionID)
					}
				}
				if err != nil {
					localResult.Errors++
				}
				outcomes <- outcome{result: localResult, err: err}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, request := range requests {
			select {
			case jobs <- request:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(outcomes)
	}()

	result := ChampionPagePrewarmResult{}
	var firstErr error
	for outcome := range outcomes {
		mergeChampionPagePrewarmResult(&result, outcome.result)
		if outcome.err != nil && firstErr == nil {
			firstErr = outcome.err
		}
	}
	if firstErr == nil && ctx.Err() != nil {
		firstErr = ctx.Err()
	}
	return result, firstErr
}

func (s Server) storeChampionPageCanonicalAlias(ctx context.Context, request championPageBundleRequest) (int, error) {
	body, ok := s.responseCache.get(championPageBundleCacheKey(request))
	if !ok {
		return 0, fmt.Errorf("canonical champion page body missing after prewarm champion=%d patch=%s role=%s", request.Build.ChampionID, request.Build.Patch, request.Build.Role)
	}
	guideGames, err := championPageGuideGames(body)
	if err != nil {
		return 0, fmt.Errorf("decode canonical champion page champion=%d patch=%s: %w", request.Build.ChampionID, request.Build.Patch, err)
	}
	aliasRequest := championPageCanonicalAliasRequest(request)
	aliasKey := championPageBundleCacheKey(aliasRequest)
	ttl := s.championPageBundleTTL(request.Build.Patch)
	s.responseCache.setShared(championPageBundleCacheKey(request), body, ttl)
	s.responseCache.setShared(aliasKey, body, ttl)
	storeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.repo.StoreChampionPageBundle(storeCtx, aliasKey, body, ttl); err != nil {
		return 0, err
	}
	return guideGames, nil
}

func championPageGuideGames(body []byte) (int, error) {
	var payload struct {
		Guide struct {
			Summary struct {
				Games int `json:"games"`
			} `json:"summary"`
		} `json:"guide"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, err
	}
	return payload.Guide.Summary.Games, nil
}

func mergeChampionPagePrewarmResult(target *ChampionPagePrewarmResult, addition ChampionPagePrewarmResult) {
	if target == nil {
		return
	}
	target.Candidates += addition.Candidates
	target.Stored += addition.Stored
	target.Skipped += addition.Skipped
	target.Cached += addition.Cached
	target.Errors += addition.Errors
	target.MatchupCandidates += addition.MatchupCandidates
	target.MatchupStored += addition.MatchupStored
	target.MatchupSkipped += addition.MatchupSkipped
	target.MatchupCached += addition.MatchupCached
	target.MissingGuideIDs = append(target.MissingGuideIDs, addition.MissingGuideIDs...)
}

func filterChampionPagePrewarmChampionIDs(championIDs, requested []uint16) []uint16 {
	if len(requested) == 0 {
		return championIDs
	}
	allowed := make(map[uint16]bool, len(requested))
	for _, championID := range requested {
		if championID > 0 {
			allowed[championID] = true
		}
	}
	filtered := make([]uint16, 0, len(allowed))
	for _, championID := range championIDs {
		if allowed[championID] {
			filtered = append(filtered, championID)
		}
	}
	return filtered
}

func championPagePrewarmChampionIDs(indexes ...[]clickhouse.ChampionGuideSummary) []uint16 {
	seen := map[uint16]bool{}
	championIDs := []uint16{}
	for _, index := range indexes {
		for _, summary := range index {
			if summary.ChampionID == 0 || seen[summary.ChampionID] {
				continue
			}
			seen[summary.ChampionID] = true
			championIDs = append(championIDs, summary.ChampionID)
		}
	}
	return championIDs
}

func (s Server) prewarmChampionPageBundle(ctx context.Context, request championPageBundleRequest, seenKeys map[string]bool, matchup bool, result *ChampionPagePrewarmResult) error {
	buildCtx, cancel := context.WithTimeout(ctx, championPageBundleBuildTimeout)
	defer cancel()
	cacheKey := championPageBundleCacheKey(request)
	ttl := s.championPageBundleTTL(request.Build.Patch)
	if seenKeys[cacheKey] {
		return nil
	}
	seenKeys[cacheKey] = true
	result.Candidates++
	if matchup {
		result.MatchupCandidates++
	}
	if body, ok, err := s.repo.CachedChampionPageBundle(buildCtx, cacheKey); err == nil && ok {
		s.responseCache.set(cacheKey, body, ttl)
		result.Skipped++
		result.Cached++
		if matchup {
			result.MatchupSkipped++
			result.MatchupCached++
		}
		return nil
	} else if err != nil {
		return err
	}
	body, err := s.buildChampionPageBundleJSON(buildCtx, request)
	if err != nil {
		return err
	}
	if err := s.repo.StoreChampionPageBundle(buildCtx, cacheKey, body, ttl); err != nil {
		return err
	}
	s.responseCache.set(cacheKey, body, ttl)
	result.Stored++
	if matchup {
		result.MatchupStored++
	}
	return nil
}

func championPagePrewarmRequest(championID uint16, role, patch, rankBucket string, queueID uint16) championPageBundleRequest {
	return championPageBundleRequest{
		Build: buildAdviceRequest{
			ChampionID:       championID,
			Role:             role,
			Patch:            patch,
			RankBucket:       rankBucket,
			MinGames:         defaultItemSlotMinGames,
			ChampionMinGames: max(defaultItemSlotMinGames, 10),
			OptionLimit:      4,
			ItemContext:      normalizedItemContext("", role),
		},
		GuideMinGames: 5,
		GuideLimit:    12,
		IndexMinGames: 1,
		IndexLimit:    250,
		QueueID:       queueID,
	}
}

func championPagePrewarmRoles(roles []string) []string {
	out := []string{}
	seen := map[string]bool{}
	if len(roles) == 0 {
		roles = []string{"TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"}
	}
	for _, role := range roles {
		role = strings.ToUpper(strings.TrimSpace(role))
		if role == "" || seen[role] {
			continue
		}
		seen[role] = true
		out = append(out, role)
	}
	return out
}

func (s Server) buildChampionPageBundleJSON(ctx context.Context, request championPageBundleRequest) ([]byte, error) {
	response, err := s.buildChampionPageBundle(ctx, request)
	if err != nil {
		return nil, err
	}
	return json.Marshal(response)
}
