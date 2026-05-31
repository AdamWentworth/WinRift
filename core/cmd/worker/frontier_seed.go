package main

import (
	"context"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"winrift/core/internal/clickhouse"
	"winrift/core/internal/config"
	"winrift/core/internal/riot"
)

func seedFrontier(ctx context.Context, cfg config.Config, riotClient *riot.Client, repo *clickhouse.Repository, platforms []string) (map[string]int, error) {
	requestsByRegion := map[string]int{}
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
			return requestsByRegion, err
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
		region, err := riot.AccountRegionForPlatform(cfg.DefaultPlatform)
		if err != nil {
			log.Printf("resolve seed %q route failed: %v", value, err)
			continue
		}
		if reserveSharedRequests(ctx, cfg, repo, region, "worker:seed-account", 1) < 1 {
			log.Printf("resolve seed deferred riot_id=%s route=%s reason=shared_budget_exhausted", value, region)
			continue
		}
		requestsByRegion[region]++
		account, err := riotClient.AccountByRiotID(ctx, gameName, tagLine, cfg.DefaultPlatform)
		if err != nil {
			if isRiotAuthError(err) {
				return requestsByRegion, err
			}
			log.Printf("resolve seed %q: %v", value, err)
			continue
		}
		if account == nil {
			continue
		}
		if err := repo.UpsertAccountAlias(ctx, clickhouse.AccountAlias{
			PUUID:    account.PUUID,
			Platform: cfg.DefaultPlatform,
			GameName: account.GameName,
			TagLine:  account.TagLine,
			LastSeen: time.Now(),
		}); err != nil {
			log.Printf("frontier seed alias store failed platform=%s riot_id=%s err=%v", cfg.DefaultPlatform, value, err)
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
			return requestsByRegion, err
		}
		if ok {
			log.Printf("frontier seed added source=env-riot-id platform=%s riot_id=%s", cfg.DefaultPlatform, value)
		}
	}
	if cfg.CollectorAutoSeedChallenger {
		for _, platform := range platforms {
			requestsUsed, err := seedChallengerFrontier(ctx, cfg, riotClient, repo, platform)
			if region, regionErr := riot.RegionForPlatform(platform); regionErr == nil {
				requestsByRegion[region] += requestsUsed
			}
			if err != nil {
				if isRiotAuthError(err) {
					return requestsByRegion, err
				}
				log.Printf("frontier auto seed platform=%s: %v", platform, err)
			}
		}
	}
	return requestsByRegion, nil
}

func seedChallengerFrontier(ctx context.Context, cfg config.Config, riotClient *riot.Client, repo *clickhouse.Repository, platform string) (int, error) {
	limit := cfg.CollectorAutoSeedLimit
	if limit <= 0 {
		limit = 3
	}
	route := riot.NormalizePlatform(platform)
	if reserveSharedRequests(ctx, cfg, repo, route, "worker:seed-challenger", 1) < 1 {
		log.Printf("frontier auto seed deferred platform=%s route=%s reason=shared_budget_exhausted", platform, route)
		return 0, nil
	}
	requestsUsed := 1
	league, err := riotClient.ChallengerLeagueByQueue(ctx, platform, "RANKED_SOLO_5x5")
	if err != nil {
		return requestsUsed, err
	}
	if league == nil {
		return requestsUsed, nil
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
			if reserveSharedRequests(ctx, cfg, repo, route, "worker:seed-summoner", 1) < 1 {
				log.Printf("frontier auto seed summoner lookup deferred platform=%s summoner_id=%s reason=shared_budget_exhausted", platform, shortValue(entry.SummonerID))
				break
			}
			requestsUsed++
			summoner, err := riotClient.SummonerByID(ctx, entry.SummonerID, platform)
			if err != nil {
				if isRiotAuthError(err) {
					return requestsUsed, err
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
			return requestsUsed, err
		}
		if ok {
			inserted++
		}
	}
	log.Printf("frontier auto seed complete platform=%s considered=%d inserted=%d", platform, considered, inserted)
	return requestsUsed, nil
}
