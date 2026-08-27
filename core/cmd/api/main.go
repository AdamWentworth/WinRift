package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"winrift/core/internal/analytics"
	"winrift/core/internal/api"
	"winrift/core/internal/clickhouse"
	"winrift/core/internal/config"
	"winrift/core/internal/riot"
	"winrift/core/internal/staticdata"
)

const championPageHydrationRetryInterval = 5 * time.Second
const championPageHydrationRefreshInterval = 30 * time.Minute

func main() {
	cfg := config.Load()
	riot.ClearAuthFailureMarker(cfg)
	riotClient := riot.NewClient(cfg)
	repo, err := clickhouse.NewRepository(cfg)
	if err != nil {
		log.Fatalf("clickhouse: %v", err)
	}
	staticService := staticdata.NewService(riotClient)
	server := api.NewServer(cfg, riotClient, repo, staticService)
	go maintainChampionPageMemoryCache(context.Background(), cfg, repo, server)

	log.Printf("winrift api listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}

func maintainChampionPageMemoryCache(ctx context.Context, cfg config.Config, repo *clickhouse.Repository, server api.Server) {
	patch := cfg.CollectorCurrentPatch
	if patch == "" || !cfg.ChampionPagePrewarmEnabled {
		return
	}
	stats, err := repo.PatchStats(ctx, analytics.RankedSoloQueueID)
	if err != nil {
		log.Printf("champion page API memory hydration fallback discovery failed current_patch=%s err=%v", patch, err)
	}
	patches := championPageHydrationPatchList(patch, cfg.CollectorPatchRetention, stats)
	for {
		currentResult, currentErr := server.HydrateChampionPageBundles(ctx, api.ChampionPagePrewarmOptions{
			Patch:      patch,
			PerRole:    cfg.ChampionPagePrewarmPerRole,
			RankBucket: cfg.ChampionPagePrewarmRankBucket,
			QueueID:    analytics.RankedSoloQueueID,
		})
		delay := championPageHydrationRetryInterval
		if currentErr != nil {
			log.Printf("champion page API memory hydration failed patch=%s err=%v", patch, currentErr)
		} else if currentResult.Candidates > 0 && currentResult.Missing == 0 {
			fallbackPatch := ""
			fallbackLoaded := 0
			fallbackMissing := 0
			var fallbackErr error
			if len(patches) > 1 && len(currentResult.MissingGuideIDs) > 0 {
				fallbackPatch = patches[1]
				fallbackResult, err := server.HydrateChampionPageBundles(ctx, api.ChampionPagePrewarmOptions{
					Patch:       fallbackPatch,
					ChampionIDs: currentResult.MissingGuideIDs,
					PerRole:     cfg.ChampionPagePrewarmPerRole,
					RankBucket:  cfg.ChampionPagePrewarmRankBucket,
					QueueID:     analytics.RankedSoloQueueID,
				})
				fallbackErr = err
				fallbackLoaded = fallbackResult.Loaded
				fallbackMissing = fallbackResult.Missing
			}
			if fallbackErr != nil {
				log.Printf("champion page API memory hydration failed patch=%s scope=current-guide-gaps err=%v", fallbackPatch, fallbackErr)
			} else if fallbackMissing > 0 {
				log.Printf("champion page API memory hydration incomplete patch=%s scope=current-guide-gaps loaded=%d missing=%d retry_in=%s", fallbackPatch, fallbackLoaded, fallbackMissing, championPageHydrationRetryInterval)
			} else {
				log.Printf("champion page API memory hydration complete patch=%s candidates=%d loaded=%d missing=%d missing_guide_champions=%d fallback_patch=%s fallback_loaded=%d refresh_in=%s", patch, currentResult.Candidates, currentResult.Loaded, currentResult.Missing, len(currentResult.MissingGuideIDs), fallbackPatch, fallbackLoaded, championPageHydrationRefreshInterval)
				delay = championPageHydrationRefreshInterval
			}
		} else {
			log.Printf("champion page API memory hydration incomplete patch=%s candidates=%d loaded=%d missing=%d retry_in=%s", patch, currentResult.Candidates, currentResult.Loaded, currentResult.Missing, championPageHydrationRetryInterval)
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func championPageHydrationPatchList(currentPatch string, retention int, stats []clickhouse.PatchStat) []string {
	currentPatch = strings.TrimSpace(currentPatch)
	if currentPatch == "" {
		return nil
	}
	var newestPrevious string
	var maturePrevious string
	for _, stat := range stats {
		patch := strings.TrimSpace(stat.Patch)
		if patch == "" || stat.Matches == 0 || compareChampionPagePatches(patch, currentPatch) >= 0 {
			continue
		}
		if newestPrevious == "" || compareChampionPagePatches(patch, newestPrevious) > 0 {
			newestPrevious = patch
		}
		if stat.Matches >= 5000 && (maturePrevious == "" || compareChampionPagePatches(patch, maturePrevious) > 0) {
			maturePrevious = patch
		}
	}
	fallbackPatch := maturePrevious
	if fallbackPatch == "" {
		fallbackPatch = newestPrevious
	}
	if fallbackPatch == "" {
		window := analytics.PatchWindow(currentPatch, retention)
		if len(window) > 1 {
			fallbackPatch = window[1]
		}
	}
	patches := []string{currentPatch}
	if fallbackPatch != "" && fallbackPatch != currentPatch {
		patches = append(patches, fallbackPatch)
	}
	return patches
}

func compareChampionPagePatches(left, right string) int {
	parse := func(value string) (int, int, bool) {
		parts := strings.Split(strings.TrimSpace(value), ".")
		if len(parts) < 2 {
			return 0, 0, false
		}
		major, majorErr := strconv.Atoi(parts[0])
		minor, minorErr := strconv.Atoi(parts[1])
		return major, minor, majorErr == nil && minorErr == nil
	}
	leftMajor, leftMinor, leftOK := parse(left)
	rightMajor, rightMinor, rightOK := parse(right)
	if !leftOK || !rightOK {
		return strings.Compare(strings.TrimSpace(left), strings.TrimSpace(right))
	}
	if leftMajor != rightMajor {
		if leftMajor < rightMajor {
			return -1
		}
		return 1
	}
	if leftMinor < rightMinor {
		return -1
	}
	if leftMinor > rightMinor {
		return 1
	}
	return 0
}
