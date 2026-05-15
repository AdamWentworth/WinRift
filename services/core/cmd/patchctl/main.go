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
)

func main() {
	action := flag.String("action", "", "one of: collecting, compile, delete-raw")
	patch := flag.String("patch", "", "patch bucket, for example 16.10")
	platform := flag.String("platform", "NA1", "platform route")
	queueID := flag.Int("queue", 420, "queue id")
	retainDays := flag.Int("retain-days", 30, "raw retention window after compile")
	flag.Parse()

	if *action == "" || *patch == "" {
		fmt.Fprintln(os.Stderr, "usage: patchctl -action collecting|compile|delete-raw -patch 16.10 [-platform NA1] [-queue 420] [-retain-days 30]")
		os.Exit(2)
	}

	cfg := config.Load()
	repo, err := clickhouse.NewRepository(cfg)
	if err != nil {
		log.Fatalf("clickhouse: %v", err)
	}

	ctx := context.Background()
	normalizedPlatform := riot.NormalizePlatform(*platform)
	switch *action {
	case "collecting":
		err = repo.MarkPatchCollecting(ctx, *patch, normalizedPlatform, uint16(*queueID))
	case "compile":
		retainedUntil := time.Now().Add(time.Duration(*retainDays) * 24 * time.Hour)
		err = repo.CompilePatchMetrics(ctx, *patch, normalizedPlatform, uint16(*queueID), retainedUntil)
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
