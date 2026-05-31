package api

import (
	"context"
	"encoding/json"
	"strings"

	"winrift/core/internal/analytics"
)

type ChampionPagePrewarmOptions struct {
	Patch      string
	Roles      []string
	PerRole    int
	MinGames   int
	RankBucket string
	QueueID    uint16
}

type ChampionPagePrewarmResult struct {
	Candidates int
	Stored     int
	Errors     int
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
		perRole = 5
	}
	minGames := options.MinGames
	if minGames <= 0 {
		minGames = 50
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
			cacheKey := championPageBundleCacheKey(request)
			if seenKeys[cacheKey] {
				continue
			}
			seenKeys[cacheKey] = true
			result.Candidates++
			body, err := s.buildChampionPageBundleJSON(ctx, request)
			if err != nil {
				result.Errors++
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if err := s.repo.StoreChampionPageBundle(ctx, cacheKey, body, analyticsResponseCacheTTL); err != nil {
				result.Errors++
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			s.responseCache.set(cacheKey, body, analyticsResponseCacheTTL)
			result.Stored++
		}
	}
	return result, firstErr
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
