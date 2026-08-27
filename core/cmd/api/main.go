package main

import (
	"context"
	"log"
	"net/http"
	"sort"
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
	stablePatchesHydrated := map[string]bool{}
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
				allPatchesReady := true
				totalCandidates := currentResult.Candidates
				totalLoaded := currentResult.Loaded
				for _, archivedPatch := range patches[1:] {
					if stablePatchesHydrated[archivedPatch] {
						continue
					}
					archivedResult, err := server.HydrateChampionPageBundles(ctx, api.ChampionPagePrewarmOptions{
						Patch:      archivedPatch,
						PerRole:    cfg.ChampionPagePrewarmPerRole,
						RankBucket: cfg.ChampionPagePrewarmRankBucket,
						QueueID:    analytics.RankedSoloQueueID,
					})
					totalCandidates += archivedResult.Candidates
					totalLoaded += archivedResult.Loaded
					if err != nil {
						allPatchesReady = false
						log.Printf("champion page API memory hydration failed patch=%s scope=all err=%v", archivedPatch, err)
						continue
					}
					if archivedResult.Candidates == 0 || archivedResult.Missing > 0 {
						allPatchesReady = false
						log.Printf("champion page API memory hydration incomplete patch=%s scope=all candidates=%d loaded=%d missing=%d retry_in=%s", archivedPatch, archivedResult.Candidates, archivedResult.Loaded, archivedResult.Missing, championPageHydrationRetryInterval)
						continue
					}
					if !analytics.PatchInWindow(archivedPatch, patch, cfg.CollectorPatchRetention) {
						stablePatchesHydrated[archivedPatch] = true
					}
					log.Printf("champion page API memory hydration patch complete patch=%s scope=all candidates=%d loaded=%d missing=%d", archivedPatch, archivedResult.Candidates, archivedResult.Loaded, archivedResult.Missing)
				}
				if allPatchesReady {
					log.Printf("champion page API memory hydration complete patch=%s candidates=%d loaded=%d missing=%d missing_guide_champions=%d fallback_patch=%s fallback_loaded=%d refresh_in=%s", patch, currentResult.Candidates, currentResult.Loaded, currentResult.Missing, len(currentResult.MissingGuideIDs), fallbackPatch, fallbackLoaded, championPageHydrationRefreshInterval)
					log.Printf("champion page API memory hydration all patches complete current_patch=%s patches=%s candidates=%d loaded=%d refresh_in=%s", patch, strings.Join(patches, ","), totalCandidates, totalLoaded, championPageHydrationRefreshInterval)
					delay = championPageHydrationRefreshInterval
				}
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
	seen := map[string]bool{currentPatch: true}
	previous := make([]string, 0, len(stats))
	maturePrevious := ""
	for _, stat := range stats {
		patch := strings.TrimSpace(stat.Patch)
		if patch == "" || stat.Matches == 0 || seen[patch] || compareChampionPagePatches(patch, currentPatch) >= 0 {
			continue
		}
		seen[patch] = true
		previous = append(previous, patch)
		if stat.Matches >= 5000 && (maturePrevious == "" || compareChampionPagePatches(patch, maturePrevious) > 0) {
			maturePrevious = patch
		}
	}
	sort.Slice(previous, func(left, right int) bool {
		return compareChampionPagePatches(previous[left], previous[right]) > 0
	})
	if maturePrevious != "" && len(previous) > 0 && previous[0] != maturePrevious {
		ordered := []string{maturePrevious}
		for _, patch := range previous {
			if patch != maturePrevious {
				ordered = append(ordered, patch)
			}
		}
		previous = ordered
	}
	if len(previous) == 0 {
		window := analytics.PatchWindow(currentPatch, retention)
		if len(window) > 1 {
			previous = append(previous, window[1])
		}
	}
	return append([]string{currentPatch}, previous...)
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
