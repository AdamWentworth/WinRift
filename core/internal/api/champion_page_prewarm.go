package api

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"winrift/core/internal/analytics"
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
	Errors            int
	MatchupCandidates int
	MatchupStored     int
	MatchupSkipped    int
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
			request := championPageBundleRequest{
				Build: buildAdviceRequest{
					ChampionID:       summary.ChampionID,
					Role:             summaryRole,
					Patch:            patch,
					RankBucket:       rankBucket,
					MinGames:         defaultItemSlotMinGames,
					ChampionMinGames: max(defaultItemSlotMinGames, 10),
					OptionLimit:      4,
					ItemContext:      normalizedItemContext("", summaryRole),
				},
				GuideMinGames: 5,
				GuideLimit:    12,
				IndexMinGames: 1,
				IndexLimit:    250,
				QueueID:       queueID,
			}
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

func (s Server) prewarmChampionPageBundle(ctx context.Context, request championPageBundleRequest, seenKeys map[string]bool, matchup bool, result *ChampionPagePrewarmResult) error {
	cacheKey := championPageBundleCacheKey(request)
	if seenKeys[cacheKey] {
		return nil
	}
	seenKeys[cacheKey] = true
	result.Candidates++
	if matchup {
		result.MatchupCandidates++
	}
	if body, ok, err := s.repo.CachedChampionPageBundle(ctx, cacheKey); err == nil && ok {
		s.responseCache.set(cacheKey, body, championPageBundleCacheTTL)
		result.Skipped++
		if matchup {
			result.MatchupSkipped++
		}
		return nil
	} else if err != nil {
		return err
	}
	body, err := s.buildChampionPageBundleJSON(ctx, request)
	if err != nil {
		return err
	}
	if err := s.repo.StoreChampionPageBundle(ctx, cacheKey, body, championPageBundleCacheTTL); err != nil {
		return err
	}
	s.responseCache.set(cacheKey, body, championPageBundleCacheTTL)
	result.Stored++
	if matchup {
		result.MatchupStored++
	}
	return nil
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
