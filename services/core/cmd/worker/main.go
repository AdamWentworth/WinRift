package main

import (
	"context"
	"log"
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
	interval := cfg.CollectorInterval
	for {
		ctx := context.Background()
		puuids := splitCSV(os.Getenv("COLLECTOR_SEED_PUUIDS"))
		for _, value := range splitCSV(os.Getenv("COLLECTOR_SEED_RIOT_IDS")) {
			gameName, tagLine, err := riot.ParseRiotID(value)
			if err != nil {
				log.Printf("invalid riot id %q: %v", value, err)
				continue
			}
			account, err := riotClient.AccountByRiotID(ctx, gameName, tagLine, cfg.DefaultPlatform)
			if err != nil {
				log.Printf("resolve seed %q: %v", value, err)
				continue
			}
			if account != nil {
				puuids = append(puuids, account.PUUID)
			}
		}

		if len(puuids) == 0 {
			log.Printf("collector idle: no seeds configured")
		}
		seen := map[string]bool{}
		for _, puuid := range puuids {
			if seen[puuid] {
				continue
			}
			seen[puuid] = true
			result := matchCollector.CollectFromPUUID(ctx, puuid, cfg.DefaultPlatform, cfg.CollectorDefaultMatchCount)
			log.Printf("collector puuid=%s seen=%d inserted=%d skipped=%d errors=%d", puuid, result.MatchIDsSeen, result.MatchesInserted, result.MatchesSkipped, len(result.Errors))
		}

		time.Sleep(interval)
	}
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
