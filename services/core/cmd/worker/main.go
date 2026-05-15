package main

import (
	"context"
	"errors"
	"log"
	"net/http"
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
	riotClient := riot.NewClient(cfg)
	repo, err := clickhouse.NewRepository(cfg)
	if err != nil {
		log.Fatalf("clickhouse: %v", err)
	}

	matchCollector := collector.New(riotClient, repo)
	platforms := collectorPlatforms(cfg)
	log.Printf("collector platforms=%s interval=%s match_requests_per_platform=%d rank_requests_per_platform=%d", strings.Join(platforms, ","), cfg.CollectorInterval, cfg.CollectorMaxRequests, cfg.RankEnrichmentMaxRequests)
	if err := seedFrontier(context.Background(), cfg, riotClient, repo, platforms); err != nil {
		if isRiotAuthError(err) {
			log.Fatalf("collector stopping: Riot API key is missing, expired, or not authorized")
		}
		log.Printf("seed frontier: %v", err)
	}
	interval := cfg.CollectorInterval
	for {
		ctx := context.Background()
		rateLimitedRegions := map[string]bool{}
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
			result := runPlatformPass(ctx, cfg, matchCollector, repo, platform)
			if result.AuthFailed {
				log.Fatalf("collector stopping: Riot API key is missing, expired, or not authorized")
			}
			if result.RateLimited {
				rateLimitedRegions[region] = true
			}
		}

		time.Sleep(interval)
	}
}

func runPlatformPass(ctx context.Context, cfg config.Config, matchCollector collector.Collector, repo *clickhouse.Repository, platform string) collector.Result {
	entries, err := repo.FetchDueFrontier(ctx, platform, cfg.CollectorFrontierBatchSize)
	if err != nil {
		log.Printf("frontier fetch platform=%s: %v", platform, err)
		return collector.Result{Errors: []string{err.Error()}}
	}
	if len(entries) == 0 {
		log.Printf("collector idle platform=%s: no due frontier rows", platform)
		return collector.Result{}
	}
	requestsLeft := cfg.CollectorMaxRequests
	rankRequestsLeft := cfg.RankEnrichmentMaxRequests
	var passResult collector.Result
	for _, entry := range entries {
		if cfg.CollectorMaxRequests > 0 && requestsLeft <= 0 {
			log.Printf("collector request budget exhausted before platform=%s puuid=%s", platform, shortValue(entry.PUUID))
			break
		}
		rankEnabled := cfg.RankEnrichmentEnabled
		if cfg.RankEnrichmentEnabled && cfg.RankEnrichmentMaxRequests > 0 && rankRequestsLeft <= 0 {
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
		if cfg.CollectorMaxRequests > 0 {
			requestsLeft -= result.RequestsUsed
		}
		if cfg.RankEnrichmentEnabled && cfg.RankEnrichmentMaxRequests > 0 {
			rankRequestsLeft -= result.RankRequestsUsed
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

func seedFrontier(ctx context.Context, cfg config.Config, riotClient *riot.Client, repo *clickhouse.Repository, platforms []string) error {
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
			return err
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
		account, err := riotClient.AccountByRiotID(ctx, gameName, tagLine, cfg.DefaultPlatform)
		if err != nil {
			if isRiotAuthError(err) {
				return err
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
			return err
		}
		if ok {
			log.Printf("frontier seed added source=env-riot-id platform=%s riot_id=%s", cfg.DefaultPlatform, value)
		}
	}
	if cfg.CollectorAutoSeedChallenger {
		for _, platform := range platforms {
			if err := seedChallengerFrontier(ctx, cfg, riotClient, repo, platform); err != nil {
				if isRiotAuthError(err) {
					return err
				}
				log.Printf("frontier auto seed platform=%s: %v", platform, err)
			}
		}
	}
	return nil
}

func seedChallengerFrontier(ctx context.Context, cfg config.Config, riotClient *riot.Client, repo *clickhouse.Repository, platform string) error {
	limit := cfg.CollectorAutoSeedLimit
	if limit <= 0 {
		limit = 3
	}
	league, err := riotClient.ChallengerLeagueByQueue(ctx, platform, "RANKED_SOLO_5x5")
	if err != nil {
		return err
	}
	if league == nil {
		return nil
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
			summoner, err := riotClient.SummonerByID(ctx, entry.SummonerID, platform)
			if err != nil {
				if isRiotAuthError(err) {
					return err
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
			return err
		}
		if ok {
			inserted++
		}
	}
	log.Printf("frontier auto seed complete platform=%s considered=%d inserted=%d", platform, considered, inserted)
	return nil
}

func isRiotAuthError(err error) bool {
	var apiErr riot.APIError
	return errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden)
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
