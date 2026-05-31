package main

import (
	"context"
	"log"
	"time"

	"winrift/core/internal/clickhouse"
	"winrift/core/internal/collector"
	"winrift/core/internal/config"
	"winrift/core/internal/riot"
)

func runRankPass(ctx context.Context, cfg config.Config, matchCollector collector.Collector, repo *clickhouse.Repository, platform string, rankRequests int) collector.Result {
	if !cfg.RankEnrichmentEnabled || rankRequests <= 0 {
		return collector.Result{}
	}
	route := riot.NormalizePlatform(platform)
	rankRequests = reserveSharedRequests(ctx, cfg, repo, route, "worker:ranks", rankRequests)
	if rankRequests <= 0 {
		log.Printf("rank lane deferred platform=%s route=%s reason=shared_budget_exhausted", platform, route)
		return collector.Result{RankBudgetExhausted: true}
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

type accountAliasPassResult struct {
	RequestsUsed    int
	AliasesInserted int
	Errors          []string
	AuthFailed      bool
	RateLimited     bool
}

func runAccountAliasPass(ctx context.Context, cfg config.Config, riotClient *riot.Client, repo *clickhouse.Repository, platform string, maxRequests int) accountAliasPassResult {
	result := accountAliasPassResult{}
	if !cfg.AccountAliasEnrichmentEnabled || maxRequests <= 0 {
		return result
	}
	normalizedPlatform := riot.NormalizePlatform(platform)
	candidates, err := repo.FetchAccountAliasCandidates(ctx, normalizedPlatform, maxRequests)
	if err != nil {
		result.Errors = append(result.Errors, "account alias candidates: "+err.Error())
		log.Printf("account alias lane candidates failed platform=%s err=%v", normalizedPlatform, err)
		return result
	}
	if len(candidates) == 0 {
		log.Printf("account alias lane complete platform=%s candidates=0 requests=0 aliases=0 errors=0 auth_failed=false rate_limited=false", normalizedPlatform)
		return result
	}
	accountRegion, err := riot.AccountRegionForPlatform(normalizedPlatform)
	if err != nil {
		result.Errors = append(result.Errors, "account alias route: "+err.Error())
		log.Printf("account alias lane route failed platform=%s err=%v", normalizedPlatform, err)
		return result
	}
	maxRequests = reserveSharedRequests(ctx, cfg, repo, accountRegion, "worker:aliases", min(maxRequests, len(candidates)))
	if maxRequests <= 0 {
		log.Printf("account alias lane deferred platform=%s account_region=%s reason=shared_budget_exhausted", normalizedPlatform, accountRegion)
		return result
	}
	log.Printf(
		"account alias lane start platform=%s candidates=%d max_requests=%d",
		normalizedPlatform,
		len(candidates),
		maxRequests,
	)
	for _, candidate := range candidates {
		if result.RequestsUsed >= maxRequests {
			log.Printf("account alias lane budget exhausted platform=%s requests=%d max_requests=%d", normalizedPlatform, result.RequestsUsed, maxRequests)
			break
		}
		result.RequestsUsed++
		log.Printf("account alias fetch start platform=%s puuid=%s requests=%d", normalizedPlatform, shortValue(candidate.PUUID), result.RequestsUsed)
		account, err := riotClient.AccountByPUUID(ctx, candidate.PUUID, normalizedPlatform)
		if err != nil {
			if isRiotAuthError(err) {
				result.AuthFailed = true
			}
			if isRiotRateLimitError(err) {
				result.RateLimited = true
			}
			result.Errors = append(result.Errors, "account alias "+candidate.PUUID+": "+err.Error())
			log.Printf("account alias fetch failed platform=%s puuid=%s err=%v", normalizedPlatform, shortValue(candidate.PUUID), err)
			if result.AuthFailed || result.RateLimited {
				break
			}
			continue
		}
		if account == nil {
			continue
		}
		if err := repo.UpsertAccountAlias(ctx, clickhouse.AccountAlias{
			PUUID:    account.PUUID,
			Platform: normalizedPlatform,
			GameName: account.GameName,
			TagLine:  account.TagLine,
			LastSeen: time.Now(),
		}); err != nil {
			result.Errors = append(result.Errors, "account alias insert: "+err.Error())
			log.Printf("account alias insert failed platform=%s puuid=%s err=%v", normalizedPlatform, shortValue(candidate.PUUID), err)
			continue
		}
		result.AliasesInserted++
		log.Printf(
			"account alias stored platform=%s puuid=%s riot_id=%s#%s",
			normalizedPlatform,
			shortValue(account.PUUID),
			account.GameName,
			account.TagLine,
		)
	}
	log.Printf(
		"account alias lane complete platform=%s candidates=%d requests=%d aliases=%d errors=%d auth_failed=%t rate_limited=%t",
		normalizedPlatform,
		len(candidates),
		result.RequestsUsed,
		result.AliasesInserted,
		len(result.Errors),
		result.AuthFailed,
		result.RateLimited,
	)
	return result
}
