package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"winrift/services/core/internal/analytics"
	"winrift/services/core/internal/clickhouse"
	"winrift/services/core/internal/collector"
	"winrift/services/core/internal/riot"
)

func (s Server) seedCollector(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.IsDevelopment() {
		writeError(w, http.StatusNotFound, "not available")
		return
	}
	var body struct {
		RiotIDs []struct {
			GameName string `json:"gameName"`
			TagLine  string `json:"tagLine"`
			Platform string `json:"platform"`
		} `json:"riotIds"`
		PUUIDs       []string `json:"puuids"`
		Platform     string   `json:"platform"`
		MatchCount   int      `json:"matchCount"`
		MaxRequests  int      `json:"maxRequests"`
		FrontierOnly bool     `json:"frontierOnly"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	platform := s.defaultPlatform(body.Platform)
	log.Printf(
		"dev collector seed start riot_ids=%d puuids=%d platform=%s match_count=%d max_requests=%d frontier_only=%t current_patch=%s patch_retention=%d rank_inline_enabled=%t rank_lane_enabled=%t rank_max_requests=%d",
		len(body.RiotIDs),
		len(body.PUUIDs),
		platform,
		body.MatchCount,
		body.MaxRequests,
		body.FrontierOnly,
		s.cfg.CollectorCurrentPatch,
		s.cfg.CollectorPatchRetention,
		false,
		s.cfg.RankEnrichmentEnabled,
		s.cfg.RankEnrichmentMaxRequests,
	)
	type seedTarget struct {
		puuid    string
		platform string
	}
	targets := make([]seedTarget, 0, len(body.PUUIDs)+len(body.RiotIDs))
	errorsOut := []string{}
	frontierSeedsAdded := 0
	now := time.Now()
	for _, puuid := range body.PUUIDs {
		log.Printf("dev collector seed puuid source=body puuid=%s platform=%s", shortValue(puuid), platform)
		targets = append(targets, seedTarget{puuid: puuid, platform: platform})
		ok, err := s.repo.InsertFrontierSeed(r.Context(), clickhouse.FrontierSeed{
			PUUID:        puuid,
			Platform:     platform,
			Source:       "seed-api-puuid",
			SourceDetail: "dev collector seed",
			Priority:     100,
			NextCheckAt:  now,
			Force:        true,
		})
		if err != nil {
			errorsOut = append(errorsOut, err.Error())
			log.Printf("dev collector frontier seed failed source=puuid puuid=%s err=%v", shortValue(puuid), err)
			continue
		}
		if ok {
			frontierSeedsAdded++
			log.Printf("dev collector frontier seed added source=puuid puuid=%s", shortValue(puuid))
		}
	}
	for _, id := range body.RiotIDs {
		idPlatform := platform
		if id.Platform != "" {
			idPlatform = riot.NormalizePlatform(id.Platform)
		}
		log.Printf("dev collector resolving riot_id=%s#%s platform=%s", id.GameName, id.TagLine, idPlatform)
		account, err := s.riot.AccountByRiotID(r.Context(), id.GameName, id.TagLine, idPlatform)
		if err != nil {
			errorsOut = append(errorsOut, err.Error())
			log.Printf("dev collector resolve failed riot_id=%s#%s platform=%s err=%v", id.GameName, id.TagLine, idPlatform, err)
			continue
		}
		if account != nil {
			s.storeAccountAlias(r.Context(), account, idPlatform)
			log.Printf("dev collector resolved riot_id=%s#%s platform=%s puuid=%s", id.GameName, id.TagLine, idPlatform, shortValue(account.PUUID))
			targets = append(targets, seedTarget{puuid: account.PUUID, platform: idPlatform})
			ok, err := s.repo.InsertFrontierSeed(r.Context(), clickhouse.FrontierSeed{
				PUUID:        account.PUUID,
				Platform:     idPlatform,
				Source:       "seed-api-riot-id",
				SourceDetail: id.GameName + "#" + id.TagLine,
				Priority:     100,
				NextCheckAt:  now,
				Force:        true,
			})
			if err != nil {
				errorsOut = append(errorsOut, err.Error())
				log.Printf("dev collector frontier seed failed source=riot_id riot_id=%s#%s err=%v", id.GameName, id.TagLine, err)
				continue
			}
			if ok {
				frontierSeedsAdded++
				log.Printf("dev collector frontier seed added source=riot_id riot_id=%s#%s puuid=%s", id.GameName, id.TagLine, shortValue(account.PUUID))
			}
		} else {
			log.Printf("dev collector riot_id not found riot_id=%s#%s platform=%s", id.GameName, id.TagLine, idPlatform)
		}
	}
	if body.FrontierOnly {
		log.Printf("dev collector seed complete mode=frontier_only frontier_seeds_added=%d errors=%d", frontierSeedsAdded, len(errorsOut))
		writeJSON(w, http.StatusOK, map[string]any{"frontierSeedsAdded": frontierSeedsAdded, "errors": errorsOut})
		return
	}
	count := body.MatchCount
	if count <= 0 {
		count = s.cfg.CollectorDefaultMatchCount
	}
	usableRequests := s.cfg.CollectorUsableRequestsPerRegion()
	maxRequests := body.MaxRequests
	if maxRequests <= 0 {
		maxRequests = s.cfg.CollectorMaxRequests
	}
	rankRequestsLeft := s.cfg.CollectorRankRequestBudget(usableRequests)
	if maxRequests <= 0 {
		maxRequests = usableRequests - rankRequestsLeft
	}
	if rankRequestsLeft+maxRequests > usableRequests {
		maxRequests = usableRequests - rankRequestsLeft
	}
	if maxRequests < 1 {
		maxRequests = 1
	}
	seen := 0
	inserted := 0
	skipped := 0
	frontierAdded := 0
	requestsUsed := 0
	rankRequestsUsed := 0
	rankSnapshotsInserted := 0
	patchBoundaryHits := 0
	unique := map[string]bool{}
	for _, target := range targets {
		uniqueKey := target.platform + "\x00" + target.puuid
		if unique[uniqueKey] {
			continue
		}
		unique[uniqueKey] = true
		requestsLeft := 0
		if maxRequests > 0 {
			requestsLeft = maxRequests - requestsUsed
			if requestsLeft <= 0 {
				break
			}
		}
		rankEnabled := false
		log.Printf("dev collector target start puuid=%s platform=%s requests_left=%d rank_inline_enabled=%t rank_requests_left=%d", shortValue(target.puuid), target.platform, requestsLeft, rankEnabled, rankRequestsLeft)
		result := s.collector.CollectFromPUUIDWithOptions(r.Context(), target.puuid, target.platform, collector.CollectOptions{
			MatchCount:            count,
			MaxRequests:           requestsLeft,
			DiscoveryDelay:        s.cfg.CollectorDiscoveryDelay,
			DiscoveredPriority:    0,
			ApplyCachedRanks:      s.cfg.RankEnrichmentEnabled,
			RankEnrichmentEnabled: rankEnabled,
			RankSnapshotTTL:       s.cfg.RankSnapshotTTL,
			RankMaxRequests:       rankRequestsLeft,
			CurrentPatch:          s.cfg.CollectorCurrentPatch,
			PatchRetentionCount:   s.cfg.CollectorPatchRetention,
		})
		seen += result.MatchIDsSeen
		inserted += result.MatchesInserted
		skipped += result.MatchesSkipped
		frontierAdded += result.FrontierAdded
		requestsUsed += result.RequestsUsed
		rankRequestsUsed += result.RankRequestsUsed
		if s.cfg.RankEnrichmentEnabled && s.cfg.RankEnrichmentMaxRequests > 0 {
			rankRequestsLeft -= result.RankRequestsUsed
		}
		rankSnapshotsInserted += result.RankSnapshotsInserted
		if result.PatchBoundaryReached {
			patchBoundaryHits++
		}
		errorsOut = append(errorsOut, result.Errors...)
		log.Printf(
			"dev collector target complete puuid=%s platform=%s seen=%d inserted=%d skipped=%d frontier_added=%d requests=%d rank_requests=%d rank_snapshots=%d patch_boundary=%t errors=%d",
			shortValue(target.puuid),
			target.platform,
			result.MatchIDsSeen,
			result.MatchesInserted,
			result.MatchesSkipped,
			result.FrontierAdded,
			result.RequestsUsed,
			result.RankRequestsUsed,
			result.RankSnapshotsInserted,
			result.PatchBoundaryReached,
			len(result.Errors),
		)
		if result.AuthFailed || result.RateLimited || result.BudgetExhausted {
			break
		}
	}
	log.Printf(
		"dev collector seed complete seeds=%d frontier_seeds_added=%d match_ids_seen=%d matches_inserted=%d matches_skipped=%d frontier_added=%d requests=%d rank_requests=%d rank_snapshots=%d patch_boundary_hits=%d errors=%d",
		len(unique),
		frontierSeedsAdded,
		seen,
		inserted,
		skipped,
		frontierAdded,
		requestsUsed,
		rankRequestsUsed,
		rankSnapshotsInserted,
		patchBoundaryHits,
		len(errorsOut),
	)
	writeJSON(w, http.StatusOK, map[string]any{
		"seeds": len(unique), "frontierSeedsAdded": frontierSeedsAdded,
		"matchIdsSeen": seen, "matchesInserted": inserted, "matchesSkipped": skipped,
		"frontierAdded": frontierAdded, "requestsUsed": requestsUsed,
		"rankRequestsUsed": rankRequestsUsed, "rankSnapshotsInserted": rankSnapshotsInserted,
		"currentPatch": s.cfg.CollectorCurrentPatch, "patchRetentionCount": s.cfg.CollectorPatchRetention, "patchBoundaryHits": patchBoundaryHits,
		"rankInlineEnabled":     false,
		"rankEnrichmentEnabled": s.cfg.RankEnrichmentEnabled,
		"errors":                errorsOut,
	})
}

func (s Server) refreshItemSlotAnalytics(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.IsDevelopment() {
		writeError(w, http.StatusNotFound, "not available")
		return
	}
	var body struct {
		Patch   string   `json:"patch"`
		Patches []string `json:"patches"`
		QueueID int      `json:"queueId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	patches := body.Patches
	if body.Patch != "" {
		patches = append(patches, body.Patch)
	}
	if len(patches) == 0 {
		if s.cfg.CollectorCurrentPatch != "" {
			patches = append(patches, s.cfg.CollectorCurrentPatch)
		}
		patches = append(patches, analytics.PatchWindow(s.cfg.CollectorCurrentPatch, s.cfg.CollectorPatchRetention)...)
	}
	patches = uniqueStrings(patches)
	queueID := uint16(body.QueueID)
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	contexts, err := s.itemSlotRefreshContexts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	loadoutContexts, err := s.startingLoadoutRefreshContexts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	refreshed := []string{}
	for _, patch := range patches {
		patch = strings.TrimSpace(patch)
		if patch == "" {
			continue
		}
		log.Printf("item slot analytics refresh start patch=%s queue=%d contexts=%d", patch, queueID, len(contexts))
		if err := s.repo.RefreshItemSlotAnalytics(r.Context(), patch, queueID, contexts); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		log.Printf("starting loadout analytics refresh start patch=%s queue=%d contexts=%d", patch, queueID, len(loadoutContexts))
		if err := s.repo.RefreshStartingLoadoutAnalytics(r.Context(), patch, queueID, loadoutContexts); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		refreshed = append(refreshed, patch)
		log.Printf("item slot analytics refresh complete patch=%s queue=%d item_contexts=%d loadout_contexts=%d", patch, queueID, len(contexts), len(loadoutContexts))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"patches":         refreshed,
		"queueId":         queueID,
		"contexts":        itemSlotRefreshContextKeys(contexts),
		"loadoutContexts": startingLoadoutRefreshContextKeys(loadoutContexts),
	})
}

func (s Server) refreshChampionGuideAnalytics(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.IsDevelopment() {
		writeError(w, http.StatusNotFound, "not available")
		return
	}
	var body struct {
		Patch    string   `json:"patch"`
		Patches  []string `json:"patches"`
		QueueID  int      `json:"queueId"`
		Backfill bool     `json:"backfill"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	patches := body.Patches
	if body.Patch != "" {
		patches = append(patches, body.Patch)
	}
	if len(patches) == 0 {
		if s.cfg.CollectorCurrentPatch != "" {
			patches = append(patches, s.cfg.CollectorCurrentPatch)
		}
		patches = append(patches, analytics.PatchWindow(s.cfg.CollectorCurrentPatch, s.cfg.CollectorPatchRetention)...)
	}
	patches = uniqueStrings(patches)
	queueID := uint16(body.QueueID)
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	results := []map[string]any{}
	for _, patch := range patches {
		patch = strings.TrimSpace(patch)
		if patch == "" {
			continue
		}
		startedAt := time.Now()
		result := map[string]any{"patch": patch}
		if body.Backfill {
			log.Printf("participant performance backfill start patch=%s queue=%d", patch, queueID)
			performanceBackfill, err := s.repo.BackfillParticipantPerformance(r.Context(), patch, queueID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			result["performanceRows"] = performanceBackfill.Rows
			log.Printf("participant performance backfill complete patch=%s queue=%d rows=%d duration=%s", patch, queueID, performanceBackfill.Rows, time.Since(startedAt).Round(time.Millisecond))
			log.Printf("champion guide event backfill start patch=%s queue=%d", patch, queueID)
			backfill, err := s.repo.BackfillChampionGuideEvents(r.Context(), patch, queueID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			result["matches"] = backfill.Matches
			result["skillEvents"] = backfill.SkillEvents
			result["bans"] = backfill.Bans
			log.Printf("champion guide event backfill complete patch=%s queue=%d matches=%d skill_events=%d bans=%d duration=%s", patch, queueID, backfill.Matches, backfill.SkillEvents, backfill.Bans, time.Since(startedAt).Round(time.Millisecond))
		}
		log.Printf("champion guide analytics refresh start patch=%s queue=%d", patch, queueID)
		if err := s.repo.RefreshChampionGuideDerivedAnalytics(r.Context(), patch, queueID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		result["durationMs"] = time.Since(startedAt).Milliseconds()
		results = append(results, result)
		log.Printf("champion guide analytics refresh complete patch=%s queue=%d duration=%s", patch, queueID, time.Since(startedAt).Round(time.Millisecond))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"patches": patches,
		"queueId": queueID,
		"results": results,
	})
}

func (s Server) refreshSummonerProfileAnalytics(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.IsDevelopment() {
		writeError(w, http.StatusNotFound, "not available")
		return
	}
	var body struct {
		QueueID int `json:"queueId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	queueID := uint16(body.QueueID)
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	startedAt := time.Now()
	log.Printf("summoner profile analytics refresh start queue=%d", queueID)
	result, err := s.repo.RefreshSummonerProfileAnalytics(r.Context(), queueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("summoner profile analytics refresh complete queue=%d identity_rows=%d profile_rows=%d champion_rows=%d champion_role_rows=%d duration=%s", queueID, result.IdentityRows, result.ProfileRows, result.ChampionRows, result.ChampionRoleRows, time.Since(startedAt).Round(time.Millisecond))
	writeJSON(w, http.StatusOK, map[string]any{
		"queueId":          queueID,
		"identityRows":     result.IdentityRows,
		"profileRows":      result.ProfileRows,
		"championRows":     result.ChampionRows,
		"championRoleRows": result.ChampionRoleRows,
		"durationMs":       time.Since(startedAt).Milliseconds(),
	})
}

func (s Server) itemSlotRefreshContexts(ctx context.Context) ([]clickhouse.ItemSlotAnalyticsContext, error) {
	defaultItems, err := s.static.BuildItemIDs(ctx, "", false, false)
	if err != nil {
		return nil, err
	}
	defaultStartingItems, err := s.static.StartingItemIDs(ctx, "", false, false)
	if err != nil {
		return nil, err
	}
	jungleItems, err := s.static.BuildItemIDs(ctx, "", true, false)
	if err != nil {
		return nil, err
	}
	jungleStartingItems, err := s.static.StartingItemIDs(ctx, "", true, false)
	if err != nil {
		return nil, err
	}
	supportItems, err := s.static.BuildItemIDs(ctx, "", false, true)
	if err != nil {
		return nil, err
	}
	supportStartingItems, err := s.static.StartingItemIDs(ctx, "", false, true)
	if err != nil {
		return nil, err
	}
	return []clickhouse.ItemSlotAnalyticsContext{
		{Key: "DEFAULT", ItemIDs: defaultItems, StartingItemIDs: defaultStartingItems},
		{Key: "JUNGLE", ItemIDs: jungleItems, StartingItemIDs: jungleStartingItems},
		{Key: "SUPPORT", ItemIDs: supportItems, StartingItemIDs: supportStartingItems},
	}, nil
}

func (s Server) startingLoadoutRefreshContexts(ctx context.Context) ([]clickhouse.StartingLoadoutAnalyticsContext, error) {
	defaultCosts, err := s.static.OpeningItemCosts(ctx, "", false, false)
	if err != nil {
		return nil, err
	}
	jungleCosts, err := s.static.OpeningItemCosts(ctx, "", true, false)
	if err != nil {
		return nil, err
	}
	supportCosts, err := s.static.OpeningItemCosts(ctx, "", false, true)
	if err != nil {
		return nil, err
	}
	return []clickhouse.StartingLoadoutAnalyticsContext{
		{Key: "DEFAULT", OpeningItemCosts: defaultCosts},
		{Key: "JUNGLE", OpeningItemCosts: jungleCosts},
		{Key: "SUPPORT", OpeningItemCosts: supportCosts},
	}, nil
}

func itemSlotRefreshContextKeys(contexts []clickhouse.ItemSlotAnalyticsContext) []string {
	keys := make([]string, 0, len(contexts))
	for _, context := range contexts {
		keys = append(keys, context.Key)
	}
	return keys
}

func startingLoadoutRefreshContextKeys(contexts []clickhouse.StartingLoadoutAnalyticsContext) []string {
	keys := make([]string, 0, len(contexts))
	for _, context := range contexts {
		keys = append(keys, context.Key)
	}
	return keys
}
