package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"winrift/services/core/internal/clickhouse"
	"winrift/services/core/internal/collector"
	"winrift/services/core/internal/config"
	"winrift/services/core/internal/riot"
	"winrift/services/core/internal/staticdata"
)

type Server struct {
	cfg       config.Config
	riot      *riot.Client
	repo      *clickhouse.Repository
	static    *staticdata.Service
	collector collector.Collector
}

func NewServer(cfg config.Config, riotClient *riot.Client, repo *clickhouse.Repository, staticService *staticdata.Service) Server {
	return Server{cfg: cfg, riot: riotClient, repo: repo, static: staticService, collector: collector.New(riotClient, repo)}
}

func (s Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/account/resolve", s.resolveAccount)
	mux.HandleFunc("GET /api/live-game", s.liveGame)
	mux.HandleFunc("GET /api/analytics/builds", s.analyticsBuilds)
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

func (s Server) liveGame(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	gameName := query.Get("gameName")
	tagLine := query.Get("tagLine")
	platform := riot.NormalizePlatform(query.Get("platform"))
	account, err := s.riot.AccountByRiotID(r.Context(), gameName, tagLine, platform)
	if err != nil {
		writeRiotError(w, err)
		return
	}
	if account == nil {
		writeError(w, http.StatusNotFound, "Riot ID not found")
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
