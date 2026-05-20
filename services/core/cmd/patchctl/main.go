package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"winrift/services/core/internal/clickhouse"
	"winrift/services/core/internal/config"
	"winrift/services/core/internal/riot"
	"winrift/services/core/internal/staticdata"
)

func main() {
	action := flag.String("action", "", "one of: collecting, compile, win-conditions, item-slots, champion-guides, delete-raw")
	patch := flag.String("patch", "", "patch bucket, for example 16.10")
	platform := flag.String("platform", "NA1", "platform route")
	queueID := flag.Int("queue", 420, "queue id")
	retainDays := flag.Int("retain-days", 30, "raw retention window after compile")
	backfill := flag.Bool("backfill", false, "backfill retained raw payload data before refreshing derived analytics")
	flag.Parse()

	if *action == "" || *patch == "" {
		fmt.Fprintln(os.Stderr, "usage: patchctl -action collecting|compile|win-conditions|item-slots|champion-guides|delete-raw -patch 16.10 [-platform NA1] [-queue 420] [-retain-days 30] [-backfill]")
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
	case "win-conditions":
		err = repo.RefreshWinConditionMetrics(ctx, *patch, normalizedPlatform, uint16(*queueID))
	case "item-slots":
		contexts, contextErr := itemSlotRefreshContexts(ctx, staticService)
		if contextErr != nil {
			err = contextErr
			break
		}
		err = repo.RefreshItemSlotAnalytics(ctx, *patch, uint16(*queueID), contexts)
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
		err = repo.DeleteRawPatchData(ctx, *patch, normalizedPlatform, uint16(*queueID))
	default:
		err = fmt.Errorf("unknown action %q", *action)
	}
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("patchctl action=%s patch=%s platform=%s queue=%d complete", *action, *patch, normalizedPlatform, *queueID)
}

func itemSlotRefreshContexts(ctx context.Context, staticService *staticdata.Service) ([]clickhouse.ItemSlotAnalyticsContext, error) {
	defaultItems, err := staticService.BuildItemIDs(ctx, "", false, false)
	if err != nil {
		return nil, err
	}
	jungleItems, err := staticService.BuildItemIDs(ctx, "", true, false)
	if err != nil {
		return nil, err
	}
	supportItems, err := staticService.BuildItemIDs(ctx, "", false, true)
	if err != nil {
		return nil, err
	}
	return []clickhouse.ItemSlotAnalyticsContext{
		{Key: "DEFAULT", ItemIDs: defaultItems},
		{Key: "JUNGLE", ItemIDs: jungleItems},
		{Key: "SUPPORT", ItemIDs: supportItems},
	}, nil
}
