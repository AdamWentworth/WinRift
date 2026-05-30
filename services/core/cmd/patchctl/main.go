package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"winrift/services/core/internal/clickhouse"
	"winrift/services/core/internal/config"
	"winrift/services/core/internal/riot"
	"winrift/services/core/internal/staticdata"
)

func main() {
	action := flag.String("action", "", "one of: collecting, compile, archive, win-conditions, item-slots, champion-guides, delete-raw")
	patch := flag.String("patch", "", "patch bucket, for example 16.10")
	platform := flag.String("platform", "NA1", "platform route, or ALL for archive/delete-raw")
	queueID := flag.Int("queue", 420, "queue id")
	retainDays := flag.Int("retain-days", 30, "raw retention window after compile")
	backfill := flag.Bool("backfill", false, "backfill retained raw payload data before refreshing derived analytics")
	pruneRaw := flag.Bool("prune-raw", true, "delete archived raw payload/timeline rows after archive compilation")
	flag.Parse()

	if *action == "" || *patch == "" {
		fmt.Fprintln(os.Stderr, "usage: patchctl -action collecting|compile|archive|win-conditions|item-slots|champion-guides|delete-raw -patch 16.10 [-platform NA1|ALL] [-queue 420] [-retain-days 30] [-backfill] [-prune-raw=true]")
		os.Exit(2)
	}

	cfg := config.Load()
	riotClient := riot.NewClient(cfg)
	repo, err := clickhouse.NewRepository(cfg)
	if err != nil {
		log.Fatalf("clickhouse: %v", err)
	}
	staticService := staticdata.NewService(riotClient)

	ctx := context.Background()
	normalizedPlatform := riot.NormalizePlatform(*platform)
	switch *action {
	case "collecting":
		err = repo.MarkPatchCollecting(ctx, *patch, normalizedPlatform, uint16(*queueID))
	case "compile":
		retainedUntil := time.Now().Add(time.Duration(*retainDays) * 24 * time.Hour)
		err = repo.CompilePatchMetrics(ctx, *patch, normalizedPlatform, uint16(*queueID), retainedUntil)
	case "archive":
		retainedUntil := time.Now().Add(time.Duration(*retainDays) * 24 * time.Hour)
		err = archivePatch(ctx, repo, staticService, *patch, normalizedPlatform, uint16(*queueID), retainedUntil, *pruneRaw)
	case "win-conditions":
		err = repo.RefreshWinConditionMetrics(ctx, *patch, normalizedPlatform, uint16(*queueID))
	case "item-slots":
		contexts, contextErr := itemSlotRefreshContexts(ctx, staticService)
		if contextErr != nil {
			err = contextErr
			break
		}
		if err = repo.RefreshItemSlotAnalytics(ctx, *patch, uint16(*queueID), contexts); err != nil {
			break
		}
		loadoutContexts, contextErr := startingLoadoutRefreshContexts(ctx, staticService)
		if contextErr != nil {
			err = contextErr
			break
		}
		err = repo.RefreshStartingLoadoutAnalytics(ctx, *patch, uint16(*queueID), loadoutContexts)
	case "champion-guides":
		if *backfill {
			if _, err = repo.BackfillParticipantPerformance(ctx, *patch, uint16(*queueID)); err != nil {
				break
			}
			if _, err = repo.BackfillChampionGuideEvents(ctx, *patch, uint16(*queueID)); err != nil {
				break
			}
		}
		err = repo.RefreshChampionGuideDerivedAnalytics(ctx, *patch, uint16(*queueID))
	case "delete-raw":
		err = deleteRawPatchData(ctx, repo, *patch, normalizedPlatform, uint16(*queueID))
	default:
		err = fmt.Errorf("unknown action %q", *action)
	}
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("patchctl action=%s patch=%s platform=%s queue=%d complete", *action, *patch, normalizedPlatform, *queueID)
}

func archivePatch(ctx context.Context, repo *clickhouse.Repository, staticService *staticdata.Service, patch, platform string, queueID uint16, retainedUntil time.Time, pruneRaw bool) error {
	platforms, err := patchPlatforms(ctx, repo, patch, platform, queueID)
	if err != nil {
		return err
	}
	log.Printf("patch archive start patch=%s platforms=%s queue=%d prune_raw=%t", patch, strings.Join(platforms, ","), queueID, pruneRaw)

	performance, err := repo.BackfillParticipantPerformance(ctx, patch, queueID)
	if err != nil {
		return fmt.Errorf("participant performance backfill: %w", err)
	}
	log.Printf("patch archive participant performance patch=%s rows=%d", patch, performance.Rows)

	events, err := repo.BackfillChampionGuideEvents(ctx, patch, queueID)
	if err != nil {
		return fmt.Errorf("champion guide event backfill: %w", err)
	}
	log.Printf("patch archive champion guide events patch=%s matches=%d skill_events=%d bans=%d", patch, events.Matches, events.SkillEvents, events.Bans)

	itemContexts, err := itemSlotRefreshContexts(ctx, staticService)
	if err != nil {
		return fmt.Errorf("item slot contexts: %w", err)
	}
	if err := repo.RefreshItemSlotAnalytics(ctx, patch, queueID, itemContexts); err != nil {
		return fmt.Errorf("item slot analytics: %w", err)
	}
	loadoutContexts, err := startingLoadoutRefreshContexts(ctx, staticService)
	if err != nil {
		return fmt.Errorf("starting loadout contexts: %w", err)
	}
	if err := repo.RefreshStartingLoadoutAnalytics(ctx, patch, queueID, loadoutContexts); err != nil {
		return fmt.Errorf("starting loadout analytics: %w", err)
	}
	log.Printf("patch archive item summaries patch=%s item_contexts=%d loadout_contexts=%d", patch, len(itemContexts), len(loadoutContexts))

	if err := repo.RefreshChampionGuideDerivedAnalytics(ctx, patch, queueID); err != nil {
		return fmt.Errorf("champion guide derived analytics: %w", err)
	}
	log.Printf("patch archive champion guide summaries patch=%s complete", patch)

	profiles, err := repo.RefreshSummonerProfileAnalytics(ctx, queueID)
	if err != nil {
		return fmt.Errorf("summoner profile analytics: %w", err)
	}
	log.Printf("patch archive summoner profile summaries queue=%d profiles=%d champions=%d champion_roles=%d", queueID, profiles.ProfileRows, profiles.ChampionRows, profiles.ChampionRoleRows)

	for _, platform := range platforms {
		log.Printf("patch archive compile platform patch=%s platform=%s queue=%d", patch, platform, queueID)
		if err := repo.CompilePatchMetrics(ctx, patch, platform, queueID, retainedUntil); err != nil {
			return fmt.Errorf("compile %s: %w", platform, err)
		}
	}
	if pruneRaw {
		for _, platform := range platforms {
			log.Printf("patch archive raw prune patch=%s platform=%s queue=%d", patch, platform, queueID)
			if err := repo.DeleteRawPatchData(ctx, patch, platform, queueID); err != nil {
				return fmt.Errorf("raw prune %s: %w", platform, err)
			}
		}
	}
	return nil
}

func deleteRawPatchData(ctx context.Context, repo *clickhouse.Repository, patch, platform string, queueID uint16) error {
	platforms, err := patchPlatforms(ctx, repo, patch, platform, queueID)
	if err != nil {
		return err
	}
	for _, platform := range platforms {
		if err := repo.DeleteRawPatchData(ctx, patch, platform, queueID); err != nil {
			return err
		}
	}
	return nil
}

func patchPlatforms(ctx context.Context, repo *clickhouse.Repository, patch, platform string, queueID uint16) ([]string, error) {
	if strings.EqualFold(strings.TrimSpace(platform), "ALL") {
		platforms, err := repo.PatchPlatforms(ctx, patch, queueID)
		if err != nil {
			return nil, err
		}
		if len(platforms) == 0 {
			return nil, fmt.Errorf("patch %s has no stored platforms for queue %d", patch, queueID)
		}
		return platforms, nil
	}
	return []string{platform}, nil
}

func itemSlotRefreshContexts(ctx context.Context, staticService *staticdata.Service) ([]clickhouse.ItemSlotAnalyticsContext, error) {
	defaultItems, err := staticService.BuildItemIDs(ctx, "", false, false)
	if err != nil {
		return nil, err
	}
	defaultStartingItems, err := staticService.StartingItemIDs(ctx, "", false, false)
	if err != nil {
		return nil, err
	}
	jungleItems, err := staticService.BuildItemIDs(ctx, "", true, false)
	if err != nil {
		return nil, err
	}
	jungleStartingItems, err := staticService.StartingItemIDs(ctx, "", true, false)
	if err != nil {
		return nil, err
	}
	supportItems, err := staticService.BuildItemIDs(ctx, "", false, true)
	if err != nil {
		return nil, err
	}
	supportStartingItems, err := staticService.StartingItemIDs(ctx, "", false, true)
	if err != nil {
		return nil, err
	}
	return []clickhouse.ItemSlotAnalyticsContext{
		{Key: "DEFAULT", ItemIDs: defaultItems, StartingItemIDs: defaultStartingItems},
		{Key: "JUNGLE", ItemIDs: jungleItems, StartingItemIDs: jungleStartingItems},
		{Key: "SUPPORT", ItemIDs: supportItems, StartingItemIDs: supportStartingItems},
	}, nil
}

func startingLoadoutRefreshContexts(ctx context.Context, staticService *staticdata.Service) ([]clickhouse.StartingLoadoutAnalyticsContext, error) {
	defaultCosts, err := staticService.OpeningItemCosts(ctx, "", false, false)
	if err != nil {
		return nil, err
	}
	jungleCosts, err := staticService.OpeningItemCosts(ctx, "", true, false)
	if err != nil {
		return nil, err
	}
	supportCosts, err := staticService.OpeningItemCosts(ctx, "", false, true)
	if err != nil {
		return nil, err
	}
	return []clickhouse.StartingLoadoutAnalyticsContext{
		{Key: "DEFAULT", OpeningItemCosts: defaultCosts},
		{Key: "JUNGLE", OpeningItemCosts: jungleCosts},
		{Key: "SUPPORT", OpeningItemCosts: supportCosts},
	}, nil
}
