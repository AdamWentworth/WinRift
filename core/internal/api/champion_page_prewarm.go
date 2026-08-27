package api

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"winrift/core/internal/analytics"
	"winrift/core/internal/clickhouse"
)

type ChampionPagePrewarmOptions struct {
	Patch               string
	Roles               []string
	PerRole             int
	MinGames            int
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
	canonicalFilters := map[string]string{
		"role":        "",
		"patch":       patch,
		"rank_bucket": rankBucket,
	}
	canonicalIndex, err := s.repo.QueryChampionGuideIndex(ctx, canonicalFilters, 1, max(perRole, 250))
	if err != nil {
		result.Errors++
		firstErr = err
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
			result.Errors++
			if firstErr == nil {
				firstErr = err
			}
			allPatchIndex.Results = nil
		}
	}
	championIDs := championPagePrewarmChampionIDs(canonicalIndex.Results, allPatchIndex.Results)
	roleRates, roleErr := s.repo.ChampionRoleRatesForPatch(ctx, championIDs, queueID, patch)
	if roleErr != nil {
		result.Errors++
		if firstErr == nil {
			firstErr = roleErr
		}
	} else {
		fallbackRoleRates := roleRates
		if patch != "" {
			fallbackRoleRates, roleErr = s.repo.ChampionRoleRatesForPatch(ctx, championIDs, queueID, "")
			if roleErr != nil {
				result.Errors++
				if firstErr == nil {
					firstErr = roleErr
				}
				fallbackRoleRates = nil
			}
		}
		for _, championID := range championIDs {
			role := primaryChampionRole(roleRates, championID)
			if role == "" {
				role = primaryChampionRole(fallbackRoleRates, championID)
			}
			if role == "" {
				continue
			}
			request := championPagePrewarmRequest(championID, role, patch, rankBucket, queueID)
			if err := s.prewarmChampionPageBundle(ctx, request, seenKeys, false, &result); err != nil {
				result.Errors++
				if firstErr == nil {
					firstErr = err
				}
			}
		}
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
	if body, ok, err := s.repo.CachedChampionPageBundle(ctx, cacheKey); err == nil && ok {
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
	body, err := s.buildChampionPageBundleJSON(ctx, request)
	if err != nil {
		return err
	}
	if err := s.repo.StoreChampionPageBundle(ctx, cacheKey, body, ttl); err != nil {
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
