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

const defaultItemSlotMinGames = 5

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
	mux.HandleFunc("GET /api/summoner/profile", s.summonerProfile)
	mux.HandleFunc("GET /api/live-game", s.liveGame)
	mux.HandleFunc("GET /api/analytics/builds", s.analyticsBuilds)
	mux.HandleFunc("GET /api/analytics/champion-guides", s.analyticsChampionGuideIndex)
	mux.HandleFunc("GET /api/analytics/champion-guide", s.analyticsChampionGuide)
	mux.HandleFunc("GET /api/analytics/item-slots", s.analyticsItemSlots)
	mux.HandleFunc("POST /api/analytics/item-slots/batch", s.analyticsItemSlotsBatch)
	mux.HandleFunc("GET /api/analytics/champion-roles", s.analyticsChampionRoles)
	mux.HandleFunc("POST /api/analytics/win-conditions", s.analyticsWinConditions)
	mux.HandleFunc("GET /api/analytics/win-conditions/diagnostics", s.analyticsWinConditionDiagnostics)
	mux.HandleFunc("GET /api/static/{kind}", s.staticData)
	mux.HandleFunc("POST /api/dev/collector/seed", s.seedCollector)
	mux.HandleFunc("POST /api/dev/analytics/item-slots/refresh", s.refreshItemSlotAnalytics)
	mux.HandleFunc("POST /api/dev/analytics/champion-guides/refresh", s.refreshChampionGuideAnalytics)
	mux.HandleFunc("POST /api/dev/analytics/summoner-profiles/refresh", s.refreshSummonerProfileAnalytics)
	return s.cors(mux)
}

func (s Server) health(w http.ResponseWriter, r *http.Request) {
	riotStatus := "ok"
	if riot.AuthFailureMarkerExists(s.cfg) {
		riotStatus = "auth_failed"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "riotApi": riotStatus})
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

func (s Server) summonerProfile(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	gameName := strings.TrimSpace(query.Get("gameName"))
	tagLine := strings.TrimSpace(query.Get("tagLine"))
	platform := s.defaultPlatform(query.Get("platform"))
	if gameName == "" || tagLine == "" {
		writeError(w, http.StatusBadRequest, "gameName and tagLine are required")
		return
	}
	alias, err := s.repo.FindAccountAlias(r.Context(), platform, gameName, tagLine)
	if err != nil {
		if clickhouse.IsNoRows(err) {
			writeError(w, http.StatusNotFound, "Summoner profile not found in stored aliases")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	queueID := uint16(analytics.RankedSoloQueueID)
	stats, err := s.repo.SummonerProfileStats(r.Context(), alias.Platform, alias.PUUID, queueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	topChampions, err := s.repo.SummonerTopChampions(r.Context(), alias.Platform, alias.PUUID, queueID, 24)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recentMatches, err := s.repo.SummonerRecentMatches(r.Context(), alias.Platform, alias.PUUID, queueID, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response := map[string]any{
		"account":       accountAliasResponse(alias),
		"summary":       summonerProfileStatsResponse(stats),
		"topChampions":  championPerformanceRowsResponse(topChampions),
		"recentMatches": summonerRecentMatchesResponse(recentMatches),
	}
	rank, err := s.repo.LatestRankSnapshot(r.Context(), alias.Platform, alias.PUUID, "RANKED_SOLO_5x5")
	if err == nil {
		response["rank"] = rankSnapshotResponse(rank)
	} else if !clickhouse.IsNoRows(err) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
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

func (s Server) analyticsChampionGuideIndex(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filters := map[string]string{
		"role":        strings.ToUpper(query.Get("role")),
		"patch":       query.Get("patch"),
		"rank_bucket": strings.ToUpper(query.Get("rankBucket")),
	}
	minGames := queryInt(query.Get("minGames"), 1)
	limit := queryInt(query.Get("limit"), 250)
	index, err := s.repo.QueryChampionGuideIndex(r.Context(), filters, minGames, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"results":            championGuideSummariesResponse(index.Results),
		"matchCount":         index.MatchCount,
		"participantSamples": index.ParticipantSamples,
	})
}

func (s Server) analyticsChampionGuide(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filters := map[string]string{
		"champion_id": query.Get("championId"),
		"role":        strings.ToUpper(query.Get("role")),
		"patch":       query.Get("patch"),
		"rank_bucket": strings.ToUpper(query.Get("rankBucket")),
	}
	if strings.TrimSpace(filters["champion_id"]) == "" {
		writeError(w, http.StatusBadRequest, "championId is required")
		return
	}
	minGames := queryInt(query.Get("minGames"), 5)
	limit := queryInt(query.Get("limit"), 12)
	guide, err := s.repo.QueryChampionGuide(r.Context(), filters, minGames, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, championGuideResponse(guide))
}

func (s Server) analyticsItemSlots(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	request := itemSlotAnalyticsRequest{
		ChampionID:         uint16(queryInt(query.Get("championId"), 0)),
		Role:               strings.ToUpper(query.Get("role")),
		ItemContext:        strings.ToUpper(query.Get("itemContext")),
		OpponentChampionID: uint16(queryInt(query.Get("opponentChampionId"), 0)),
		Patch:              query.Get("patch"),
		RankBucket:         strings.ToUpper(query.Get("rankBucket")),
		MinGames:           queryInt(query.Get("minGames"), defaultItemSlotMinGames),
		Limit:              queryInt(query.Get("limit"), 6),
		Fallback:           queryBool(query.Get("fallback")),
	}
	scopedRows, err := s.queryScopedItemSlots(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": itemSlotRowsResponse(scopedRows)})
}

func (s Server) analyticsItemSlotsBatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Requests []itemSlotAnalyticsRequest `json:"requests"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if len(body.Requests) > 20 {
		writeError(w, http.StatusBadRequest, "too many item-slot requests")
		return
	}
	results := make([]map[string]any, 0, len(body.Requests))
	for _, request := range body.Requests {
		scopedRows, err := s.queryScopedItemSlots(r.Context(), request)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		results = append(results, map[string]any{
			"key":     request.Key,
			"results": itemSlotRowsResponse(scopedRows),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

type itemSlotAnalyticsRequest struct {
	Key                string `json:"key"`
	ChampionID         uint16 `json:"championId"`
	Role               string `json:"role"`
	ItemContext        string `json:"itemContext"`
	OpponentChampionID uint16 `json:"opponentChampionId"`
	Patch              string `json:"patch"`
	RankBucket         string `json:"rankBucket"`
	MinGames           int    `json:"minGames"`
	Limit              int    `json:"limit"`
	Fallback           bool   `json:"fallback"`
}

func (request itemSlotAnalyticsRequest) filters() map[string]string {
	filters := map[string]string{
		"role":        strings.ToUpper(request.Role),
		"patch":       strings.TrimSpace(request.Patch),
		"rank_bucket": strings.ToUpper(strings.TrimSpace(request.RankBucket)),
	}
	if request.ChampionID > 0 {
		filters["champion_id"] = strconv.Itoa(int(request.ChampionID))
	}
	if request.OpponentChampionID > 0 {
		filters["opponent_champion_id"] = strconv.Itoa(int(request.OpponentChampionID))
	}
	return filters
}

func (s Server) queryScopedItemSlots(ctx context.Context, request itemSlotAnalyticsRequest) ([]scopedItemSlotRow, error) {
	filters := request.filters()
	minGames := request.MinGames
	if minGames <= 0 {
		minGames = defaultItemSlotMinGames
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 6
	}
	itemContext := normalizedItemContext(request.ItemContext, filters["role"])
	includeJungle := itemContext == "JUNGLE"
	includeSupport := itemContext == "SUPPORT"
	buildItemIDs, err := s.static.BuildItemIDs(ctx, "", includeJungle, includeSupport)
	if err != nil {
		return nil, err
	}
	if request.Fallback {
		return s.queryItemSlotFallbacks(ctx, filters, itemContext, buildItemIDs, minGames, limit)
	}
	rows, err := s.repo.QueryItemSlots(ctx, filters, itemContext, buildItemIDs, minGames, limit)
	if err != nil {
		return nil, err
	}
	return scopedItemSlotRows(rows, itemSlotScope{Key: "requested", Label: "Requested sample"}), nil
}

func itemSlotRowsResponse(scopedRows []scopedItemSlotRow) []map[string]any {
	results := make([]map[string]any, 0, len(scopedRows))
	for _, scoped := range scopedRows {
		row := scoped.Row
		results = append(results, map[string]any{
			"championId": row.ChampionID, "role": row.Role, "opponentChampionId": row.OpponentChampionID,
			"patchBucket": row.PatchBucket, "rankBucket": row.RankBucket,
			"itemSlot": row.ItemSlot, "itemId": row.ItemID,
			"wins": row.Wins, "games": row.Games, "winRate": round(row.WinRate * 100), "confidence": round(row.Confidence * 100),
			"sampleScope": scoped.Scope.Key, "sampleScopeLabel": scoped.Scope.Label, "fallback": scoped.Scope.Fallback,
		})
	}
	return results
}

func championGuideResponse(guide clickhouse.ChampionGuideData) map[string]any {
	return map[string]any{
		"summary":          championGuideSummaryResponse(guide.Summary),
		"toughestMatchups": championGuideMatchupRowsResponse(guide.ToughestMatchups),
		"bestMatchups":     championGuideMatchupRowsResponse(guide.BestMatchups),
		"topRunes":         championGuideSignatureRowsResponse(guide.TopRunes, "runeSignature"),
		"topSpells":        championGuideSignatureRowsResponse(guide.TopSpells, "spellSignature"),
		"topSkillOrders":   championGuideSkillOrderRowsResponse(guide.TopSkillOrders),
		"topItemPaths":     championGuideItemPathRowsResponse(guide.TopItemPaths),
	}
}

func championGuideSummaryResponse(row clickhouse.ChampionGuideSummary) map[string]any {
	return map[string]any{
		"championId":                 row.ChampionID,
		"role":                       row.Role,
		"patchBucket":                row.PatchBucket,
		"rankBucket":                 row.RankBucket,
		"wins":                       row.Wins,
		"games":                      row.Games,
		"bans":                       row.Bans,
		"winRate":                    round(row.WinRate * 100),
		"confidence":                 round(row.Confidence * 100),
		"pickRate":                   round(row.PickRate * 100),
		"banRate":                    round(row.BanRate * 100),
		"avgKills":                   round(row.AvgKills),
		"avgDeaths":                  round(row.AvgDeaths),
		"avgAssists":                 round(row.AvgAssists),
		"kda":                        round(row.KDA),
		"avgGoldEarned":              round(row.AvgGoldEarned),
		"avgCs":                      round(row.AvgCS),
		"avgDamageDealtToChampions":  round(row.AvgDamageDealtToChampions),
		"avgDamageTaken":             round(row.AvgDamageTaken),
		"avgDamageSelfMitigated":     round(row.AvgDamageSelfMitigated),
		"avgDamageDealtToObjectives": round(row.AvgDamageDealtToObjectives),
		"avgDamageDealtToStructures": round(row.AvgDamageDealtToStructures),
		"avgVisionScore":             round(row.AvgVisionScore),
		"avgTimeCcingOthers":         round(row.AvgTimeCCingOthers),
		"avgTeamUtility":             round(row.AvgTeamUtility),
		"avgStructureTakedowns":      round(row.AvgStructureTakedowns),
		"avgObjectiveTakedowns":      round(row.AvgObjectiveTakedowns),
		"avgTotalTimeSpentDead":      round(row.AvgTotalTimeSpentDead),
		"avgTimePlayed":              round(row.AvgTimePlayed),
		"killParticipation":          round(row.KillParticipation * 100),
		"tierScore":                  round(row.TierScore),
		"winScore":                   round(row.WinScore),
		"sampleScore":                round(row.SampleScore),
		"pickScore":                  round(row.PickScore),
		"banScore":                   round(row.BanScore),
		"impactScore":                round(row.ImpactScore),
		"damageScore":                round(row.DamageScore),
		"economyScore":               round(row.EconomyScore),
		"visionScore":                round(row.VisionScore),
		"objectiveScore":             round(row.ObjectiveScore),
		"utilityScore":               round(row.UtilityScore),
		"survivabilityScore":         round(row.SurvivabilityScore),
		"roleRank":                   row.RoleRank,
		"roleRankTotal":              row.RoleRankTotal,
	}
}

func championGuideSummariesResponse(rows []clickhouse.ChampionGuideSummary) []map[string]any {
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		results = append(results, championGuideSummaryResponse(row))
	}
	return results
}

func championGuideMatchupRowsResponse(rows []clickhouse.ChampionGuideMatchupRow) []map[string]any {
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		results = append(results, map[string]any{
			"opponentChampionId": row.OpponentChampionID,
			"wins":               row.Wins,
			"games":              row.Games,
			"winRate":            round(row.WinRate * 100),
			"confidence":         round(row.Confidence * 100),
		})
	}
	return results
}

func championGuideSignatureRowsResponse(rows []clickhouse.ChampionGuideSignatureRow, key string) []map[string]any {
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		results = append(results, map[string]any{
			key:          row.Signature,
			"wins":       row.Wins,
			"games":      row.Games,
			"winRate":    round(row.WinRate * 100),
			"confidence": round(row.Confidence * 100),
		})
	}
	return results
}

func championGuideSkillOrderRowsResponse(rows []clickhouse.ChampionGuideSkillOrderRow) []map[string]any {
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		results = append(results, map[string]any{
			"skillOrderSignature": row.Signature,
			"wins":                row.Wins,
			"games":               row.Games,
			"winRate":             round(row.WinRate * 100),
			"confidence":          round(row.Confidence * 100),
		})
	}
	return results
}

func championGuideItemPathRowsResponse(rows []clickhouse.ChampionGuideItemPathRow) []map[string]any {
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		results = append(results, map[string]any{
			"core3Signature":      row.Core3Signature,
			"finalItemsSignature": row.FinalItemsSignature,
			"wins":                row.Wins,
			"games":               row.Games,
			"winRate":             round(row.WinRate * 100),
			"confidence":          round(row.Confidence * 100),
		})
	}
	return results
}

type itemSlotScope struct {
	Key      string
	Label    string
	Fallback bool
	Filters  map[string]string
}

type scopedItemSlotRow struct {
	Row   clickhouse.ItemSlotRow
	Scope itemSlotScope
}

func (s Server) queryItemSlotFallbacks(ctx context.Context, filters map[string]string, itemContext string, buildItemIDs []uint32, minGames, limit int) ([]scopedItemSlotRow, error) {
	scopes := itemSlotFallbackScopes(filters)
	out := []scopedItemSlotRow{}
	coveredSlots := map[uint8]bool{}
	for _, scope := range scopes {
		rows, err := s.repo.QueryItemSlots(ctx, scope.Filters, itemContext, buildItemIDs, minGames, limit)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if coveredSlots[row.ItemSlot] {
				continue
			}
			out = append(out, scopedItemSlotRow{Row: row, Scope: scope})
			coveredSlots[row.ItemSlot] = true
			if len(coveredSlots) >= 6 {
				return out, nil
			}
		}
	}
	return out, nil
}

func normalizedItemContext(itemContext, role string) string {
	switch strings.ToUpper(strings.TrimSpace(itemContext)) {
	case "JUNGLE":
		return "JUNGLE"
	case "SUPPORT", "UTILITY":
		return "SUPPORT"
	}
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case "JUNGLE":
		return "JUNGLE"
	case "UTILITY":
		return "SUPPORT"
	default:
		return "DEFAULT"
	}
}

func itemSlotFallbackScopes(filters map[string]string) []itemSlotScope {
	patch := strings.TrimSpace(filters["patch"])
	opponent := strings.TrimSpace(filters["opponent_champion_id"])
	scopes := []itemSlotScope{
		{Key: "exact_patch_matchup", Label: itemSlotScopeLabel(patch != "", opponent != ""), Filters: cloneStringMap(filters)},
	}
	seen := map[string]bool{itemSlotScopeSignature(filters): true}
	addScope := func(key, label string, fallbackFilters map[string]string) {
		signature := itemSlotScopeSignature(fallbackFilters)
		if seen[signature] {
			return
		}
		seen[signature] = true
		scopes = append(scopes, itemSlotScope{Key: key, Label: label, Fallback: true, Filters: fallbackFilters})
	}
	if patch != "" {
		allPatchMatchup := cloneStringMap(filters)
		allPatchMatchup["patch"] = ""
		addScope("all_patch_matchup", "Exact matchup, all stored patches", allPatchMatchup)
	}
	if opponent != "" {
		championPatch := cloneStringMap(filters)
		championPatch["opponent_champion_id"] = ""
		addScope("patch_champion", itemSlotChampionScopeLabel(patch != ""), championPatch)
	}
	if patch != "" || opponent != "" {
		championAll := cloneStringMap(filters)
		championAll["patch"] = ""
		championAll["opponent_champion_id"] = ""
		addScope("all_champion", "Champion overall, all stored patches", championAll)
	}
	return scopes
}

func scopedItemSlotRows(rows []clickhouse.ItemSlotRow, scope itemSlotScope) []scopedItemSlotRow {
	out := make([]scopedItemSlotRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, scopedItemSlotRow{Row: row, Scope: scope})
	}
	return out
}

func itemSlotScopeLabel(hasPatch, hasOpponent bool) string {
	switch {
	case hasPatch && hasOpponent:
		return "Current patch exact matchup"
	case hasOpponent:
		return "Exact matchup"
	case hasPatch:
		return "Current patch champion overall"
	default:
		return "Champion overall"
	}
}

func itemSlotChampionScopeLabel(hasPatch bool) string {
	if hasPatch {
		return "Current patch champion overall"
	}
	return "Champion overall"
}

func itemSlotScopeSignature(filters map[string]string) string {
	return strings.Join([]string{
		filters["champion_id"],
		filters["role"],
		filters["opponent_champion_id"],
		filters["patch"],
		filters["rank_bucket"],
	}, "|")
}

func cloneStringMap(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
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
		refreshed = append(refreshed, patch)
		log.Printf("item slot analytics refresh complete patch=%s queue=%d contexts=%d", patch, queueID, len(contexts))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"patches":  refreshed,
		"queueId":  queueID,
		"contexts": itemSlotRefreshContextKeys(contexts),
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
	log.Printf("summoner profile analytics refresh complete queue=%d profile_rows=%d champion_rows=%d duration=%s", queueID, result.ProfileRows, result.ChampionRows, time.Since(startedAt).Round(time.Millisecond))
	writeJSON(w, http.StatusOK, map[string]any{
		"queueId":      queueID,
		"profileRows":  result.ProfileRows,
		"championRows": result.ChampionRows,
		"durationMs":   time.Since(startedAt).Milliseconds(),
	})
}

func (s Server) itemSlotRefreshContexts(ctx context.Context) ([]clickhouse.ItemSlotAnalyticsContext, error) {
	defaultItems, err := s.static.BuildItemIDs(ctx, "", false, false)
	if err != nil {
		return nil, err
	}
	jungleItems, err := s.static.BuildItemIDs(ctx, "", true, false)
	if err != nil {
		return nil, err
	}
	supportItems, err := s.static.BuildItemIDs(ctx, "", false, true)
	if err != nil {
		return nil, err
	}
	return []clickhouse.ItemSlotAnalyticsContext{
		{Key: "DEFAULT", ItemIDs: defaultItems},
		{Key: "JUNGLE", ItemIDs: jungleItems},
		{Key: "SUPPORT", ItemIDs: supportItems},
	}, nil
}

func itemSlotRefreshContextKeys(contexts []clickhouse.ItemSlotAnalyticsContext) []string {
	keys := make([]string, 0, len(contexts))
	for _, context := range contexts {
		keys = append(keys, context.Key)
	}
	return keys
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
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
	if riot.IsAuthFailure(err) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"code":   "RIOT_API_KEY_UNAVAILABLE",
			"detail": "Riot API key is unavailable. Refresh the key and recreate the API/worker containers.",
		})
		return
	}
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

func championPerformanceRowsResponse(rows []clickhouse.ChampionPerformance) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, championPerformanceResponse(row))
	}
	return out
}

func summonerProfileStatsResponse(stats clickhouse.SummonerProfileStats) map[string]any {
	response := map[string]any{
		"puuid":      stats.PUUID,
		"platform":   stats.Platform,
		"queueId":    stats.QueueID,
		"games":      stats.Games,
		"wins":       stats.Wins,
		"losses":     stats.Losses,
		"kills":      stats.Kills,
		"deaths":     stats.Deaths,
		"assists":    stats.Assists,
		"avgKills":   round(stats.AvgKills),
		"avgDeaths":  round(stats.AvgDeaths),
		"avgAssists": round(stats.AvgAssists),
		"kda":        round(stats.KDA),
		"winRate":    round(stats.WinRate * 100),
	}
	if !stats.FirstSeen.IsZero() {
		response["firstSeen"] = stats.FirstSeen
	}
	if !stats.LastSeen.IsZero() {
		response["lastSeen"] = stats.LastSeen
	}
	return response
}

func summonerRecentMatchesResponse(rows []clickhouse.SummonerRecentMatch) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"matchId":            row.MatchID,
			"platform":           row.Platform,
			"patch":              row.Patch,
			"queueId":            row.QueueID,
			"championId":         row.ChampionID,
			"role":               row.Role,
			"win":                row.Win,
			"kills":              row.Kills,
			"deaths":             row.Deaths,
			"assists":            row.Assists,
			"gameStartTimestamp": row.GameStartTimestamp,
			"durationSeconds":    row.DurationSeconds,
		})
	}
	return out
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

func queryBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
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
