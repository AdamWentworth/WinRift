package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"winrift/services/core/internal/analytics"
	"winrift/services/core/internal/clickhouse"
	"winrift/services/core/internal/collector"
	"winrift/services/core/internal/config"
	"winrift/services/core/internal/riot"
	"winrift/services/core/internal/staticdata"
	"winrift/services/core/internal/winconditions"
)

type Server struct {
	cfg       config.Config
	riot      *riot.Client
	repo      *clickhouse.Repository
	static    *staticdata.Service
	collector collector.Collector
	winConds  winconditions.Analyzer
}

func NewServer(cfg config.Config, riotClient *riot.Client, repo *clickhouse.Repository, staticService *staticdata.Service) Server {
	catalog, err := winconditions.LoadCatalog()
	if err != nil {
		log.Printf("win condition catalog load failed err=%v", err)
	}
	return Server{cfg: cfg, riot: riotClient, repo: repo, static: staticService, collector: collector.New(riotClient, repo), winConds: winconditions.NewAnalyzer(catalog)}
}

func (s Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/account/resolve", s.resolveAccount)
	mux.HandleFunc("GET /api/account/alias", s.accountAlias)
	mux.HandleFunc("GET /api/account/aliases", s.accountAliases)
	mux.HandleFunc("GET /api/live-game", s.liveGame)
	mux.HandleFunc("GET /api/analytics/builds", s.analyticsBuilds)
	mux.HandleFunc("GET /api/analytics/item-slots", s.analyticsItemSlots)
	mux.HandleFunc("GET /api/analytics/champion-roles", s.analyticsChampionRoles)
	mux.HandleFunc("POST /api/analytics/win-conditions", s.analyticsWinConditions)
	mux.HandleFunc("GET /api/static/{kind}", s.staticData)
	mux.HandleFunc("POST /api/dev/collector/seed", s.seedCollector)
	return s.cors(mux)
}

func (s Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s Server) resolveAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GameName string `json:"gameName"`
		TagLine  string `json:"tagLine"`
		Platform string `json:"platform"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	platform := s.defaultPlatform(body.Platform)
	account, err := s.riot.AccountByRiotID(r.Context(), body.GameName, body.TagLine, platform)
	if err != nil {
		writeRiotError(w, err)
		return
	}
	if account == nil {
		writeError(w, http.StatusNotFound, "Riot ID not found")
		return
	}
	s.storeAccountAlias(r.Context(), account, platform)
	summoner, err := s.riot.SummonerByPUUID(r.Context(), account.PUUID, platform)
	if err != nil {
		writeRiotError(w, err)
		return
	}
	response := map[string]any{"puuid": account.PUUID, "gameName": account.GameName, "tagLine": account.TagLine, "platform": platform}
	if summoner != nil {
		response["summonerId"] = summoner.ID
		response["accountId"] = summoner.AccountID
		response["profileIconId"] = summoner.ProfileIconID
		response["summonerLevel"] = summoner.SummonerLevel
	}
	writeJSON(w, http.StatusOK, response)
}

func (s Server) accountAlias(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	gameName := strings.TrimSpace(query.Get("gameName"))
	platform := s.defaultPlatform(query.Get("platform"))
	if gameName == "" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "not_found"})
		return
	}
	aliases, err := s.repo.ResolveAccountAliases(r.Context(), platform, gameName, 6)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(aliases) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"status": "not_found"})
		return
	}
	matches := make([]map[string]any, 0, len(aliases))
	for _, alias := range aliases {
		matches = append(matches, accountAliasResponse(alias))
	}
	if len(aliases) > 1 {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ambiguous", "matches": matches})
		return
	}
	response := accountAliasResponse(aliases[0])
	response["status"] = "found"
	writeJSON(w, http.StatusOK, response)
}

func (s Server) accountAliases(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	gameName := strings.TrimSpace(query.Get("gameName"))
	platform := s.defaultPlatform(query.Get("platform"))
	limit := 6
	if rawLimit := strings.TrimSpace(query.Get("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 && parsed <= 20 {
			limit = parsed
		}
	}
	aliases, err := s.repo.SearchAccountAliases(r.Context(), platform, gameName, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	matches := make([]map[string]any, 0, len(aliases))
	for _, alias := range aliases {
		matches = append(matches, accountAliasResponse(alias))
	}
	writeJSON(w, http.StatusOK, map[string]any{"matches": matches})
}

func (s Server) liveGame(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	gameName := query.Get("gameName")
	tagLine := query.Get("tagLine")
	platform := riot.NormalizePlatform(query.Get("platform"))
	accountRegion, err := riot.AccountRegionForPlatform(platform)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.reserveCriticalRiotRequest(w, r, accountRegion, "api:live-account") {
		return
	}
	account, err := s.riot.AccountByRiotID(r.Context(), gameName, tagLine, platform)
	if err != nil {
		writeRiotError(w, err)
		return
	}
	if account == nil {
		writeError(w, http.StatusNotFound, "Riot ID not found")
		return
	}
	s.storeAccountAlias(r.Context(), account, platform)
	if !s.reserveCriticalRiotRequest(w, r, platform, "api:live-game") {
		return
	}
	game, err := s.riot.ActiveGameByPUUID(r.Context(), account.PUUID, platform)
	if err != nil {
		writeRiotError(w, err)
		return
	}
	if game == nil {
		writeError(w, http.StatusNotFound, "Player is not currently in a live game")
		return
	}
	game["platform"] = platform
	game["puuid"] = account.PUUID
	s.enrichLiveGameRanks(r.Context(), game, platform)
	s.enrichLiveGameChampionStats(r.Context(), game, platform)
	s.queueLiveBackfills(r.Context(), game, platform)
	writeJSON(w, http.StatusOK, game)
}

func (s Server) analyticsBuilds(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filters := map[string]string{
		"champion_id":          query.Get("championId"),
		"role":                 strings.ToUpper(query.Get("role")),
		"opponent_champion_id": query.Get("opponentChampionId"),
		"patch":                query.Get("patch"),
		"rank_bucket":          strings.ToUpper(query.Get("rankBucket")),
	}
	minGames := queryInt(query.Get("minGames"), 5)
	limit := queryInt(query.Get("limit"), 25)
	rows, err := s.repo.QueryBuilds(r.Context(), filters, minGames, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		results = append(results, map[string]any{
			"championId": row.ChampionID, "role": row.Role, "opponentChampionId": row.OpponentChampionID,
			"patchBucket": row.PatchBucket, "rankBucket": row.RankBucket,
			"finalItemsSignature": row.FinalItemsSignature, "core2Signature": row.Core2Signature, "core3Signature": row.Core3Signature,
			"runeSignature": row.RuneSignature, "spellSignature": row.SpellSignature,
			"wins": row.Wins, "games": row.Games, "winRate": round(row.WinRate * 100), "confidence": round(row.Confidence * 100),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s Server) analyticsItemSlots(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filters := map[string]string{
		"champion_id":          query.Get("championId"),
		"role":                 strings.ToUpper(query.Get("role")),
		"opponent_champion_id": query.Get("opponentChampionId"),
		"patch":                query.Get("patch"),
		"rank_bucket":          strings.ToUpper(query.Get("rankBucket")),
	}
	minGames := queryInt(query.Get("minGames"), 1)
	limit := queryInt(query.Get("limit"), 6)
	buildItemIDs, err := s.static.BuildItemIDs(r.Context(), "", filters["role"] == "JUNGLE")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, err := s.repo.QueryItemSlots(r.Context(), filters, buildItemIDs, minGames, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		results = append(results, map[string]any{
			"championId": row.ChampionID, "role": row.Role, "opponentChampionId": row.OpponentChampionID,
			"patchBucket": row.PatchBucket, "rankBucket": row.RankBucket,
			"itemSlot": row.ItemSlot, "itemId": row.ItemID,
			"wins": row.Wins, "games": row.Games, "winRate": round(row.WinRate * 100), "confidence": round(row.Confidence * 100),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s Server) analyticsChampionRoles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	championIDs := queryUint16List(query.Get("championIds"))
	queueID := uint16(queryInt(query.Get("queueId"), 420))
	rows, err := s.repo.ChampionRoleRates(r.Context(), championIDs, queueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		results = append(results, map[string]any{
			"championId": row.ChampionID,
			"role":       row.Role,
			"games":      row.Games,
			"totalGames": row.TotalGames,
			"pickRate":   round(row.PickRate * 100),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s Server) staticData(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	data, err := s.static.Get(r.Context(), kind, r.URL.Query().Get("patch"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

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

func (s Server) defaultPlatform(value string) string {
	if strings.TrimSpace(value) == "" {
		return s.cfg.DefaultPlatform
	}
	return riot.NormalizePlatform(value)
}

func (s Server) cors(next http.Handler) http.Handler {
	origins := map[string]bool{}
	for _, origin := range s.cfg.CORSOrigins {
		origins[origin] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeRiotError(w http.ResponseWriter, err error) {
	var apiErr riot.APIError
	if errors.As(err, &apiErr) {
		writeError(w, apiErr.StatusCode, apiErr.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func (s Server) storeAccountAlias(ctx context.Context, account *riot.Account, platform string) {
	if account == nil {
		return
	}
	if err := s.repo.UpsertAccountAlias(ctx, clickhouse.AccountAlias{
		PUUID:    account.PUUID,
		Platform: platform,
		GameName: account.GameName,
		TagLine:  account.TagLine,
		LastSeen: time.Now(),
	}); err != nil {
		log.Printf("account alias store failed riot_id=%s#%s platform=%s err=%v", account.GameName, account.TagLine, platform, err)
	}
}

func accountAliasResponse(alias clickhouse.AccountAlias) map[string]any {
	return map[string]any{
		"puuid":    alias.PUUID,
		"platform": alias.Platform,
		"gameName": alias.GameName,
		"tagLine":  alias.TagLine,
	}
}

func (s Server) reserveCriticalRiotRequest(w http.ResponseWriter, r *http.Request, route, source string) bool {
	reservation, err := s.repo.ReserveRiotRequests(
		r.Context(),
		route,
		source,
		1,
		s.cfg.CollectorUsableRequestsPerRegion(),
		s.cfg.CollectorRateLimitWindow,
		time.Now(),
	)
	if err != nil {
		log.Printf("riot shared budget check failed route=%s source=%s err=%v", route, source, err)
		writeError(w, http.StatusServiceUnavailable, "Riot request budget is unavailable")
		return false
	}
	if reservation.Granted < 1 {
		log.Printf(
			"riot shared budget exhausted route=%s source=%s wait=%s used=%d limit=%d",
			route,
			source,
			reservation.Wait.Round(time.Second),
			reservation.Used,
			reservation.Limit,
		)
		writeError(w, http.StatusTooManyRequests, "Riot request budget exhausted; try again shortly")
		return false
	}
	return true
}

func (s Server) enrichLiveGameRanks(ctx context.Context, game map[string]any, platform string) {
	participants := liveParticipantMaps(game["participants"])
	if len(participants) == 0 {
		return
	}
	puuids := make([]string, 0, len(participants))
	seen := map[string]bool{}
	for _, participant := range participants {
		puuid, _ := participant["puuid"].(string)
		puuid = strings.TrimSpace(puuid)
		if puuid != "" && !seen[puuid] {
			seen[puuid] = true
			puuids = append(puuids, puuid)
		}
	}
	if len(puuids) == 0 {
		log.Printf("live game rank enrichment skipped platform=%s reason=no_puuids participants=%d", platform, len(participants))
		return
	}
	now := time.Now()
	snapshots, err := s.repo.FreshRankSnapshots(ctx, platform, puuids, now)
	if err != nil {
		log.Printf("live game rank enrichment failed platform=%s participants=%d err=%v", platform, len(participants), err)
		snapshots = map[string]analytics.RankSnapshot{}
	}
	cacheHits := len(snapshots)
	requests := 0
	stored := 0
	if s.cfg.LiveRankEnrichmentEnabled && s.cfg.LiveRankMaxRequests > 0 {
		ttl := s.cfg.RankSnapshotTTL
		if ttl <= 0 {
			ttl = 24 * time.Hour
		}
		missing := make([]string, 0, len(puuids))
		for _, puuid := range puuids {
			if _, ok := snapshots[puuid]; ok {
				continue
			}
			missing = append(missing, puuid)
		}
		maxFetches := min(s.cfg.LiveRankMaxRequests, len(missing))
		reservation, err := s.repo.ReserveRiotRequests(ctx, platform, "api:live-ranks", maxFetches, s.cfg.CollectorUsableRequestsPerRegion(), s.cfg.CollectorRateLimitWindow, now)
		if err != nil {
			log.Printf("live game rank budget failed platform=%s desired=%d err=%v", platform, maxFetches, err)
			maxFetches = 0
		} else {
			maxFetches = reservation.Granted
			if reservation.Granted < reservation.Desired {
				log.Printf(
					"live game rank budget limited platform=%s desired=%d granted=%d wait=%s used=%d limit=%d",
					platform,
					reservation.Desired,
					reservation.Granted,
					reservation.Wait.Round(time.Second),
					reservation.Used,
					reservation.Limit,
				)
			}
		}
		for _, puuid := range missing {
			if requests >= maxFetches {
				break
			}
			requests++
			log.Printf("live game rank fetch start platform=%s puuid=%s request=%d max_requests=%d", platform, shortValue(puuid), requests, s.cfg.LiveRankMaxRequests)
			entries, err := s.riot.LeagueEntriesByPUUID(ctx, puuid, platform)
			if err != nil {
				log.Printf("live game rank fetch failed platform=%s puuid=%s err=%v", platform, shortValue(puuid), err)
				if riot.IsAuthFailure(err) {
					break
				}
				var apiErr riot.APIError
				if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
					break
				}
				continue
			}
			snapshot := liveRankSnapshotFromEntries(puuid, platform, entries, now, now.Add(ttl))
			if err := s.repo.InsertRankSnapshot(ctx, snapshot); err != nil {
				log.Printf("live game rank snapshot store failed platform=%s puuid=%s err=%v", platform, shortValue(puuid), err)
				continue
			}
			snapshots[puuid] = snapshot
			stored++
			log.Printf("live game rank snapshot stored platform=%s puuid=%s tier=%s division=%s bucket=%s", platform, shortValue(puuid), snapshot.Tier, snapshot.Division, snapshot.RankBucket)
		}
	}
	for _, participant := range participants {
		puuid, _ := participant["puuid"].(string)
		if snapshot, ok := snapshots[puuid]; ok {
			participant["rank"] = rankSnapshotResponse(snapshot)
		}
	}
	log.Printf(
		"live game rank enrichment complete platform=%s participants=%d unique_puuids=%d cache_hits=%d fetched=%d stored=%d applied=%d missing=%d",
		platform,
		len(participants),
		len(puuids),
		cacheHits,
		requests,
		stored,
		len(snapshots),
		len(puuids)-len(snapshots),
	)
}

func (s Server) enrichLiveGameChampionStats(ctx context.Context, game map[string]any, platform string) {
	participants := liveParticipantMaps(game["participants"])
	if len(participants) == 0 {
		return
	}
	queueID := uint16(analytics.RankedSoloQueueID)
	keys := make([]clickhouse.ChampionPerformanceKey, 0, len(participants))
	for _, participant := range participants {
		puuid, _ := participant["puuid"].(string)
		championID := uint16FromAny(participant["championId"])
		if strings.TrimSpace(puuid) == "" || championID == 0 {
			continue
		}
		keys = append(keys, clickhouse.ChampionPerformanceKey{
			PUUID:      puuid,
			ChampionID: championID,
			QueueID:    queueID,
		})
	}
	if len(keys) == 0 {
		log.Printf("live game champion stats skipped platform=%s reason=no_keys participants=%d", platform, len(participants))
		return
	}
	stats, err := s.repo.ChampionPerformance(ctx, platform, keys)
	if err != nil {
		log.Printf("live game champion stats failed platform=%s participants=%d err=%v", platform, len(participants), err)
		return
	}
	applied := 0
	for _, participant := range participants {
		puuid, _ := participant["puuid"].(string)
		championID := uint16FromAny(participant["championId"])
		key := clickhouse.ChampionPerformanceKeyString(puuid, championID, queueID)
		if performance, ok := stats[key]; ok {
			participant["championStats"] = championPerformanceResponse(performance)
			applied++
		}
	}
	log.Printf(
		"live game champion stats complete platform=%s participants=%d keys=%d applied=%d missing=%d queue_id=%d",
		platform,
		len(participants),
		len(keys),
		applied,
		len(keys)-applied,
		queueID,
	)
}

func (s Server) queueLiveBackfills(ctx context.Context, game map[string]any, platform string) {
	if !s.cfg.LiveBackfillEnabled || s.cfg.LiveBackfillMaxSeeds <= 0 {
		return
	}
	participants := liveParticipantMaps(game["participants"])
	if len(participants) == 0 {
		return
	}
	minGames := s.cfg.LiveBackfillMinChampionGames
	if minGames < 0 {
		minGames = 0
	}
	priority := int16(s.cfg.LiveBackfillPriority)
	if priority <= 0 {
		priority = 95
	}
	nextCheckAt := time.Now().Add(s.cfg.LiveBackfillDelay)
	seen := map[string]bool{}
	queued := 0
	errorsCount := 0
	for _, participant := range participants {
		if queued >= s.cfg.LiveBackfillMaxSeeds {
			break
		}
		puuid, _ := participant["puuid"].(string)
		puuid = strings.TrimSpace(puuid)
		championID := uint16FromAny(participant["championId"])
		if puuid == "" || championID == 0 || seen[puuid] {
			continue
		}
		seen[puuid] = true
		games := championStatsGames(participant["championStats"])
		if games >= minGames {
			continue
		}
		_, err := s.repo.InsertFrontierSeed(ctx, clickhouse.FrontierSeed{
			PUUID:        puuid,
			Platform:     platform,
			Source:       "live-backfill",
			SourceDetail: "champion:" + strconv.Itoa(int(championID)) + " games:" + strconv.Itoa(games),
			Priority:     priority,
			NextCheckAt:  nextCheckAt,
			Force:        true,
		})
		if err != nil {
			errorsCount++
			log.Printf("live backfill seed failed platform=%s puuid=%s champion_id=%d games=%d err=%v", platform, shortValue(puuid), championID, games, err)
			continue
		}
		queued++
	}
	if queued > 0 || errorsCount > 0 {
		log.Printf(
			"live backfill queued platform=%s participants=%d queued=%d errors=%d min_champion_games=%d max_seeds=%d",
			platform,
			len(participants),
			queued,
			errorsCount,
			minGames,
			s.cfg.LiveBackfillMaxSeeds,
		)
	}
}

func liveParticipantMaps(value any) []map[string]any {
	switch participants := value.(type) {
	case []map[string]any:
		return participants
	case []any:
		out := make([]map[string]any, 0, len(participants))
		for _, participant := range participants {
			if mapped, ok := participant.(map[string]any); ok {
				out = append(out, mapped)
			}
		}
		return out
	default:
		return nil
	}
}

func championStatsGames(value any) int {
	stats, ok := value.(map[string]any)
	if !ok {
		return 0
	}
	return intFromAny(stats["games"])
}

func championPerformanceResponse(performance clickhouse.ChampionPerformance) map[string]any {
	return map[string]any{
		"queueId":    performance.QueueID,
		"championId": performance.ChampionID,
		"games":      performance.Games,
		"wins":       performance.Wins,
		"losses":     performance.Losses,
		"kills":      performance.Kills,
		"deaths":     performance.Deaths,
		"assists":    performance.Assists,
		"avgKills":   round(performance.AvgKills),
		"avgDeaths":  round(performance.AvgDeaths),
		"avgAssists": round(performance.AvgAssists),
		"kda":        round(performance.KDA),
		"winRate":    round(performance.WinRate * 100),
	}
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case uint16:
		return int(typed)
	case uint64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed)
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return 0
}

func rankSnapshotResponse(snapshot analytics.RankSnapshot) map[string]any {
	totalGames := snapshot.Wins + snapshot.Losses
	winRate := 0.0
	if totalGames > 0 {
		winRate = round(float64(snapshot.Wins) / float64(totalGames) * 100)
	}
	return map[string]any{
		"queueType":     snapshot.QueueType,
		"tier":          snapshot.Tier,
		"division":      snapshot.Division,
		"rank":          snapshot.Division,
		"leaguePoints":  snapshot.LeaguePoints,
		"wins":          snapshot.Wins,
		"losses":        snapshot.Losses,
		"totalGames":    totalGames,
		"winRate":       winRate,
		"rankBucket":    snapshot.RankBucket,
		"fetchedAt":     snapshot.FetchedAt,
		"expiresAt":     snapshot.ExpiresAt,
		"rankAvailable": snapshot.Tier != "" && snapshot.Tier != "UNRANKED",
	}
}

func liveRankSnapshotFromEntries(puuid, platform string, entries []riot.LeagueEntry, fetchedAt, expiresAt time.Time) analytics.RankSnapshot {
	snapshot := analytics.RankSnapshot{
		PUUID:      puuid,
		Platform:   riot.NormalizePlatform(platform),
		QueueType:  "RANKED_SOLO_5x5",
		Tier:       "UNRANKED",
		Division:   "",
		RankBucket: "UNRANKED",
		FetchedAt:  fetchedAt,
		ExpiresAt:  expiresAt,
	}
	for _, entry := range entries {
		if strings.ToUpper(entry.QueueType) != "RANKED_SOLO_5X5" {
			continue
		}
		snapshot.QueueType = entry.QueueType
		snapshot.Tier = strings.ToUpper(entry.Tier)
		snapshot.Division = strings.ToUpper(entry.Rank)
		snapshot.LeaguePoints = entry.LeaguePoints
		snapshot.Wins = entry.Wins
		snapshot.Losses = entry.Losses
		snapshot.RankBucket = analytics.RankBucket(entry.Tier)
		return snapshot
	}
	return snapshot
}

func writeError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func queryInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func queryUint16List(value string) []uint16 {
	parts := strings.Split(value, ",")
	out := make([]uint16, 0, len(parts))
	seen := map[uint16]bool{}
	for _, part := range parts {
		parsed, err := strconv.ParseUint(strings.TrimSpace(part), 10, 16)
		if err != nil || parsed == 0 {
			continue
		}
		value := uint16(parsed)
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func uint16FromAny(value any) uint16 {
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return uint16(typed)
		}
	case int64:
		if typed > 0 {
			return uint16(typed)
		}
	case uint16:
		return typed
	case uint64:
		if typed > 0 {
			return uint16(typed)
		}
	case float64:
		if typed > 0 {
			return uint16(typed)
		}
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil && parsed > 0 {
			return uint16(parsed)
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil && parsed > 0 {
			return uint16(parsed)
		}
	}
	return 0
}

func round(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func shortValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 12 {
		return value
	}
	return value[:8] + "..." + value[len(value)-4:]
}
