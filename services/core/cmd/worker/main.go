package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
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
	if err := seedFrontier(context.Background(), cfg, riotClient, repo); err != nil {
		if isRiotAuthError(err) {
			log.Fatalf("collector stopping: Riot API key is missing, expired, or not authorized")
		}
		log.Printf("seed frontier: %v", err)
	}
	interval := cfg.CollectorInterval
	for {
		ctx := context.Background()
		entries, err := repo.FetchDueFrontier(ctx, cfg.DefaultPlatform, cfg.CollectorFrontierBatchSize)
		if err != nil {
			log.Printf("frontier fetch: %v", err)
			time.Sleep(interval)
			continue
		}
		if len(entries) == 0 {
			log.Printf("collector idle: no due frontier rows")
		}
		requestsLeft := cfg.CollectorMaxRequests
		rankRequestsLeft := cfg.RankEnrichmentMaxRequests
		for _, entry := range entries {
			if cfg.CollectorMaxRequests > 0 && requestsLeft <= 0 {
				log.Printf("collector request budget exhausted before puuid=%s", entry.PUUID)
				break
			}
			rankEnabled := cfg.RankEnrichmentEnabled
			if cfg.RankEnrichmentEnabled && cfg.RankEnrichmentMaxRequests > 0 && rankRequestsLeft <= 0 {
				log.Printf("collector rank enrichment budget exhausted")
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
			if cfg.CollectorMaxRequests > 0 {
				requestsLeft -= result.RequestsUsed
			}
			if cfg.RankEnrichmentEnabled && cfg.RankEnrichmentMaxRequests > 0 {
				rankRequestsLeft -= result.RankRequestsUsed
			}
			status := frontierStatus(result)
			nextCheckAt := nextFrontierCheck(cfg, result)
			if err := repo.MarkFrontierChecked(ctx, entry, result.MatchIDsSeen, result.MatchesInserted, result.MatchesSkipped, len(result.Errors), result.RequestsUsed+result.RankRequestsUsed, status, nextCheckAt); err != nil {
				log.Printf("frontier update puuid=%s: %v", entry.PUUID, err)
			}
			log.Printf(
				"collector puuid=%s seen=%d inserted=%d skipped=%d frontier_added=%d requests=%d rank_requests=%d rank_snapshots=%d errors=%d status=%s",
				entry.PUUID,
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
			if result.AuthFailed {
				log.Fatalf("collector stopping: Riot API key is missing, expired, or not authorized")
			}
			if result.RateLimited || result.BudgetExhausted {
				break
			}
		}

		time.Sleep(interval)
	}
}

func seedFrontier(ctx context.Context, cfg config.Config, riotClient *riot.Client, repo *clickhouse.Repository) error {
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
			log.Printf("frontier seed added source=env-puuid puuid=%s", puuid)
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
			log.Printf("frontier seed added source=env-riot-id riot_id=%s", value)
		}
	}
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
