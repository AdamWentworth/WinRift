package collector

import (
	"context"

	"winrift/services/core/internal/analytics"
	"winrift/services/core/internal/riot"
)

type RiotClient interface {
	MatchIDsByPUUID(ctx context.Context, puuid, platform string, count int) ([]string, error)
	MatchByID(ctx context.Context, matchID, platform string) ([]byte, error)
	TimelineByMatchID(ctx context.Context, matchID, platform string) ([]byte, error)
}

type Repository interface {
	MatchExists(ctx context.Context, matchID string) (bool, error)
	InsertNormalized(ctx context.Context, normalized analytics.NormalizedMatch) error
}

type Collector struct {
	riot RiotClient
	repo Repository
}

type Result struct {
	MatchIDsSeen    int
	MatchesInserted int
	MatchesSkipped  int
	Errors          []string
}

func New(riot RiotClient, repo Repository) Collector {
	return Collector{riot: riot, repo: repo}
}

func (c Collector) CollectFromPUUID(ctx context.Context, puuid, platform string, count int) Result {
	result := Result{}
	matchIDs, err := c.riot.MatchIDsByPUUID(ctx, puuid, platform, count)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	result.MatchIDsSeen = len(matchIDs)
	for _, matchID := range matchIDs {
		exists, err := c.repo.MatchExists(ctx, matchID)
		if err != nil {
			result.Errors = append(result.Errors, matchID+": "+err.Error())
			continue
		}
		if exists {
			result.MatchesSkipped++
			continue
		}
		match, err := c.riot.MatchByID(ctx, matchID, platform)
		if err != nil {
			result.Errors = append(result.Errors, matchID+": "+err.Error())
			if apiErr, ok := err.(riot.APIError); ok && (apiErr.StatusCode == 401 || apiErr.StatusCode == 403) {
				break
			}
			continue
		}
		if !analytics.ShouldIngest(match) {
			result.MatchesSkipped++
			continue
		}
		timeline, err := c.riot.TimelineByMatchID(ctx, matchID, platform)
		if err != nil {
			result.Errors = append(result.Errors, matchID+": "+err.Error())
			continue
		}
		normalized, err := analytics.NormalizeMatch(match, timeline, riot.NormalizePlatform(platform), "UNKNOWN")
		if err != nil {
			result.Errors = append(result.Errors, matchID+": "+err.Error())
			continue
		}
		if err := c.repo.InsertNormalized(ctx, normalized); err != nil {
			result.Errors = append(result.Errors, matchID+": "+err.Error())
			continue
		}
		result.MatchesInserted++
	}
	return result
}
