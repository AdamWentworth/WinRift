package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"winrift/core/internal/analytics"
	"winrift/core/internal/clickhouse"
)

const defaultItemSlotMinGames = 5
const analyticsResponseCacheTTL = 10 * time.Minute
const championPageBundleCacheTTL = 2 * time.Hour
const championPageBundleCacheKeyPrefix = "champion-page:v4:"

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
	writeJSON(w, http.StatusOK, map[string]any{"results": buildRowsResponse(rows)})
}

func (s Server) analyticsBuildAdvice(w http.ResponseWriter, r *http.Request) {
	request, badRequest := parseBuildAdviceRequest(r.URL.Query())
	if badRequest != "" {
		writeError(w, http.StatusBadRequest, badRequest)
		return
	}
	cacheKey := "build-advice:" + r.URL.RequestURI()
	if body, ok := s.responseCache.get(cacheKey); ok {
		writeJSONBytes(w, http.StatusOK, body, true)
		return
	}
	response, err := s.buildAdviceResponse(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeCachedJSON(w, http.StatusOK, cacheKey, analyticsResponseCacheTTL, response)
}

func (s Server) analyticsChampionPage(w http.ResponseWriter, r *http.Request) {
	request, badRequest := parseChampionPageBundleRequest(r.URL.Query())
	if badRequest != "" {
		writeError(w, http.StatusBadRequest, badRequest)
		return
	}
	request, err := s.resolveChampionPageBundleRequest(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cacheKey := championPageBundleCacheKey(request)
	if body, ok := s.responseCache.get(cacheKey); ok {
		writeJSONBytes(w, http.StatusOK, body, true)
		return
	}
	if body, ok, err := s.repo.CachedChampionPageBundle(r.Context(), cacheKey); err != nil {
		log.Printf("champion page persistent cache lookup failed key=%s err=%v", cacheKey, err)
	} else if ok {
		s.responseCache.set(cacheKey, body, championPageBundleCacheTTL)
		writeJSONBytes(w, http.StatusOK, body, true)
		return
	}
	response, err := s.buildChampionPageBundle(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeCachedChampionPageJSON(w, http.StatusOK, cacheKey, championPageBundleCacheTTL, response)
}

type championPageBundleRequest struct {
	Build         buildAdviceRequest
	GuideMinGames int
	GuideLimit    int
	IndexMinGames int
	IndexLimit    int
	QueueID       uint16
}

func parseChampionPageBundleRequest(query url.Values) (championPageBundleRequest, string) {
	buildRequest, badRequest := parseBuildAdviceRequest(query)
	if badRequest != "" {
		return championPageBundleRequest{}, badRequest
	}
	guideMinGames := queryInt(query.Get("guideMinGames"), 5)
	if guideMinGames <= 0 {
		guideMinGames = 5
	}
	guideLimit := queryInt(query.Get("guideLimit"), 12)
	if guideLimit <= 0 {
		guideLimit = 12
	}
	indexMinGames := queryInt(query.Get("indexMinGames"), 1)
	if indexMinGames <= 0 {
		indexMinGames = 1
	}
	indexLimit := queryInt(query.Get("indexLimit"), 250)
	if indexLimit <= 0 {
		indexLimit = 250
	}
	queueID := uint16(queryInt(query.Get("queueId"), analytics.RankedSoloQueueID))
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	return championPageBundleRequest{
		Build:         buildRequest,
		GuideMinGames: guideMinGames,
		GuideLimit:    guideLimit,
		IndexMinGames: indexMinGames,
		IndexLimit:    indexLimit,
		QueueID:       queueID,
	}, ""
}

func championPageBundleCacheKey(request championPageBundleRequest) string {
	query := url.Values{}
	query.Set("championId", strconv.Itoa(int(request.Build.ChampionID)))
	query.Set("role", request.Build.Role)
	query.Set("itemContext", request.Build.ItemContext)
	query.Set("opponentChampionId", strconv.Itoa(int(request.Build.OpponentChampionID)))
	query.Set("patch", request.Build.Patch)
	query.Set("rankBucket", request.Build.RankBucket)
	query.Set("minGames", strconv.Itoa(request.Build.MinGames))
	query.Set("championMinGames", strconv.Itoa(request.Build.ChampionMinGames))
	query.Set("limit", strconv.Itoa(request.Build.OptionLimit))
	query.Set("guideMinGames", strconv.Itoa(request.GuideMinGames))
	query.Set("guideLimit", strconv.Itoa(request.GuideLimit))
	query.Set("indexMinGames", strconv.Itoa(request.IndexMinGames))
	query.Set("indexLimit", strconv.Itoa(request.IndexLimit))
	query.Set("queueId", strconv.Itoa(int(request.QueueID)))
	return championPageBundleCacheKeyPrefix + query.Encode()
}

func (s Server) buildChampionPageBundle(ctx context.Context, request championPageBundleRequest) (map[string]any, error) {
	buildRequest := request.Build
	indexFilters := map[string]string{
		"role":        buildRequest.Role,
		"patch":       buildRequest.Patch,
		"rank_bucket": buildRequest.RankBucket,
	}
	queryCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		buildAdvice map[string]any
		guide       clickhouse.ChampionGuideData
		index       clickhouse.ChampionGuideIndex
		roleRates   []clickhouse.ChampionRoleRate
		wg          sync.WaitGroup
		mu          sync.Mutex
		firstErr    error
	)
	run := func(fn func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(queryCtx); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				mu.Unlock()
			}
		}()
	}
	run(func(ctx context.Context) error {
		var err error
		buildAdvice, guide, err = s.buildAdviceResponseWithGuide(ctx, buildRequest, request.GuideMinGames, request.GuideLimit)
		return err
	})
	run(func(ctx context.Context) error {
		var err error
		index, err = s.repo.QueryChampionGuideIndex(ctx, indexFilters, request.IndexMinGames, request.IndexLimit)
		return err
	})
	run(func(ctx context.Context) error {
		var err error
		roleRates, err = s.repo.ChampionRoleRatesForPatch(ctx, []uint16{buildRequest.ChampionID}, request.QueueID, buildRequest.Patch)
		return err
	})
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	response := map[string]any{
		"filters": map[string]any{
			"championId":         buildRequest.ChampionID,
			"role":               buildRequest.Role,
			"opponentChampionId": buildRequest.OpponentChampionID,
			"patch":              buildRequest.Patch,
			"rankBucket":         buildRequest.RankBucket,
			"queueId":            request.QueueID,
		},
		"guide":       championGuideResponse(guide),
		"buildAdvice": buildAdvice,
		"guideIndex": map[string]any{
			"results":            championGuideSummariesResponse(index.Results),
			"matchCount":         index.MatchCount,
			"participantSamples": index.ParticipantSamples,
		},
		"roleRates": championRoleRatesResponse(roleRates),
	}
	return response, nil
}

func (s Server) resolveChampionPageBundleRequest(ctx context.Context, request championPageBundleRequest) (championPageBundleRequest, error) {
	request.Build.Role = strings.ToUpper(strings.TrimSpace(request.Build.Role))
	if request.Build.Role == "" {
		roleRates, err := s.repo.ChampionRoleRatesForPatch(ctx, []uint16{request.Build.ChampionID}, request.QueueID, request.Build.Patch)
		if err != nil {
			return request, err
		}
		request.Build.Role = primaryChampionRole(roleRates, request.Build.ChampionID)
	}
	request.Build.ItemContext = normalizedItemContext(request.Build.ItemContext, request.Build.Role)
	return request, nil
}

func primaryChampionRole(rows []clickhouse.ChampionRoleRate, championID uint16) string {
	best := clickhouse.ChampionRoleRate{}
	for _, row := range rows {
		if row.ChampionID != championID || strings.TrimSpace(row.Role) == "" {
			continue
		}
		if best.Role == "" || row.Games > best.Games || (row.Games == best.Games && row.PickRate > best.PickRate) {
			best = row
		}
	}
	return strings.ToUpper(strings.TrimSpace(best.Role))
}

type buildAdviceRequest struct {
	ChampionID         uint16
	Role               string
	OpponentChampionID uint16
	Patch              string
	RankBucket         string
	MinGames           int
	ChampionMinGames   int
	OptionLimit        int
	ItemContext        string
}

func parseBuildAdviceRequest(query url.Values) (buildAdviceRequest, string) {
	championID := uint16(queryInt(query.Get("championId"), 0))
	if championID == 0 {
		return buildAdviceRequest{}, "championId is required"
	}
	role := strings.ToUpper(strings.TrimSpace(query.Get("role")))
	minGames := queryInt(query.Get("minGames"), defaultItemSlotMinGames)
	if minGames <= 0 {
		minGames = defaultItemSlotMinGames
	}
	championMinGames := queryInt(query.Get("championMinGames"), max(minGames, 10))
	optionLimit := queryInt(query.Get("limit"), 4)
	if optionLimit <= 0 {
		optionLimit = 4
	}
	if optionLimit > 12 {
		optionLimit = 12
	}
	return buildAdviceRequest{
		ChampionID:         championID,
		Role:               role,
		OpponentChampionID: uint16(queryInt(query.Get("opponentChampionId"), 0)),
		Patch:              strings.TrimSpace(query.Get("patch")),
		RankBucket:         strings.ToUpper(strings.TrimSpace(query.Get("rankBucket"))),
		MinGames:           minGames,
		ChampionMinGames:   championMinGames,
		OptionLimit:        optionLimit,
		ItemContext:        normalizedItemContext(strings.ToUpper(query.Get("itemContext")), role),
	}, ""
}

func (s Server) buildAdviceResponse(ctx context.Context, request buildAdviceRequest) (map[string]any, error) {
	response, _, err := s.buildAdviceResponseWithGuide(ctx, request, request.ChampionMinGames, 8)
	return response, err
}

func (s Server) buildAdviceResponseWithGuide(ctx context.Context, request buildAdviceRequest, guideMinGames, guideLimit int) (map[string]any, clickhouse.ChampionGuideData, error) {
	matchupRequest := itemSlotAnalyticsRequest{
		Key:                      "matchup",
		ChampionID:               request.ChampionID,
		Role:                     request.Role,
		ItemContext:              request.ItemContext,
		OpponentChampionID:       request.OpponentChampionID,
		Patch:                    request.Patch,
		RankBucket:               request.RankBucket,
		MinGames:                 request.MinGames,
		Limit:                    request.OptionLimit,
		Fallback:                 true,
		SuppressChampionFallback: true,
	}
	if guideMinGames <= 0 {
		guideMinGames = request.ChampionMinGames
	}
	if guideLimit <= 0 {
		guideLimit = 8
	}
	championRequest := matchupRequest
	championRequest.Key = "champion"
	championRequest.OpponentChampionID = 0
	championRequest.MinGames = request.ChampionMinGames

	matchupSlots := []scopedItemSlotRow{}
	var err error
	if request.OpponentChampionID > 0 {
		matchupSlots, err = s.queryScopedItemSlots(ctx, matchupRequest)
		if err != nil {
			return nil, clickhouse.ChampionGuideData{}, err
		}
	}
	championSlots, err := s.queryScopedItemSlots(ctx, championRequest)
	if err != nil {
		return nil, clickhouse.ChampionGuideData{}, err
	}
	openingItemCosts, err := s.openingItemCosts(ctx, request.ItemContext)
	if err != nil {
		return nil, clickhouse.ChampionGuideData{}, err
	}
	buildFilters := map[string]string{
		"champion_id":          strconv.Itoa(int(request.ChampionID)),
		"role":                 request.Role,
		"opponent_champion_id": "",
		"patch":                request.Patch,
		"rank_bucket":          request.RankBucket,
	}
	matchupBuilds := []clickhouse.BuildRow{}
	matchupStartingLoadouts := []clickhouse.StartingItemLoadoutRow{}
	if request.OpponentChampionID > 0 {
		buildFilters["opponent_champion_id"] = strconv.Itoa(int(request.OpponentChampionID))
		matchupBuilds, err = s.repo.QueryBuilds(ctx, buildFilters, request.MinGames, 8)
		if err != nil {
			return nil, clickhouse.ChampionGuideData{}, err
		}
		matchupStartingLoadouts, err = s.repo.QueryStartingItemLoadouts(ctx, buildFilters, request.ItemContext, openingItemCosts, request.MinGames, request.OptionLimit)
		if err != nil {
			return nil, clickhouse.ChampionGuideData{}, err
		}
	}
	championFilters := cloneStringMap(buildFilters)
	championFilters["opponent_champion_id"] = ""
	championStartingLoadouts, err := s.repo.QueryStartingItemLoadouts(ctx, championFilters, request.ItemContext, openingItemCosts, request.ChampionMinGames, request.OptionLimit)
	if err != nil {
		return nil, clickhouse.ChampionGuideData{}, err
	}
	championBuilds, err := s.repo.QueryBuilds(ctx, championFilters, request.ChampionMinGames, 8)
	if err != nil {
		return nil, clickhouse.ChampionGuideData{}, err
	}
	guide, err := s.repo.QueryChampionGuide(ctx, championFilters, guideMinGames, guideLimit)
	if err != nil {
		return nil, clickhouse.ChampionGuideData{}, err
	}
	notes := buildAdviceNotes(request.OpponentChampionID > 0, matchupSlots, championSlots)
	return map[string]any{
		"filters": map[string]any{
			"championId":         request.ChampionID,
			"role":               request.Role,
			"opponentChampionId": request.OpponentChampionID,
			"patch":              request.Patch,
			"rankBucket":         request.RankBucket,
			"itemContext":        request.ItemContext,
			"minGames":           request.MinGames,
			"championMinGames":   request.ChampionMinGames,
			"limit":              request.OptionLimit,
		},
		"matchup": map[string]any{
			"available":        request.OpponentChampionID > 0,
			"itemSlots":        itemSlotRowsResponse(matchupSlots),
			"startingLoadouts": startingItemLoadoutRowsResponse(matchupStartingLoadouts),
			"topBuilds":        buildRowsResponse(matchupBuilds),
			"sample":           buildAdviceSampleResponse(matchupSlots),
			"sampleMode":       "champion_matchup",
		},
		"champion": map[string]any{
			"itemSlots":        itemSlotRowsResponse(championSlots),
			"startingLoadouts": startingItemLoadoutRowsResponse(championStartingLoadouts),
			"topBuilds":        buildRowsResponse(championBuilds),
			"topRunes":         championGuideSignatureRowsResponse(guide.TopRunes, "runeSignature"),
			"topSpells":        championGuideSignatureRowsResponse(guide.TopSpells, "spellSignature"),
			"topItemPaths":     championGuideItemPathRowsResponse(guide.TopItemPaths),
			"buildVariants":    championGuideBuildVariantRowsResponse(guide.BuildVariants),
			"summary":          championGuideSummaryResponse(guide.Summary),
			"sample":           buildAdviceSampleResponse(championSlots),
			"sampleMode":       "champion_overall",
			"strictRoleUsed":   request.Role != "",
		},
		"diagnostics": map[string]any{
			"matchup":  buildAdviceItemSlotDiagnostics(matchupSlots),
			"champion": buildAdviceItemSlotDiagnostics(championSlots),
		},
		"notes": notes,
	}, guide, nil
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
	cacheKey := "champion-guide:" + r.URL.RequestURI()
	if body, ok := s.responseCache.get(cacheKey); ok {
		writeJSONBytes(w, http.StatusOK, body, true)
		return
	}
	guide, err := s.repo.QueryChampionGuide(r.Context(), filters, minGames, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeCachedJSON(w, http.StatusOK, cacheKey, analyticsResponseCacheTTL, championGuideResponse(guide))
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

func (s Server) analyticsPatches(w http.ResponseWriter, r *http.Request) {
	queueID := uint16(queryInt(r.URL.Query().Get("queueId"), analytics.RankedSoloQueueID))
	stats, err := s.repo.PatchStats(r.Context(), queueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"currentPatch": s.cfg.CollectorCurrentPatch,
		"queueId":      queueID,
		"results":      patchStatsResponse(stats, s.cfg.CollectorCurrentPatch),
	})
}

type itemSlotAnalyticsRequest struct {
	Key                      string `json:"key"`
	ChampionID               uint16 `json:"championId"`
	Role                     string `json:"role"`
	ItemContext              string `json:"itemContext"`
	OpponentChampionID       uint16 `json:"opponentChampionId"`
	Patch                    string `json:"patch"`
	RankBucket               string `json:"rankBucket"`
	MinGames                 int    `json:"minGames"`
	Limit                    int    `json:"limit"`
	Fallback                 bool   `json:"fallback"`
	SuppressChampionFallback bool   `json:"suppressChampionFallback"`
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
	startingItemIDs, err := s.static.StartingItemIDs(ctx, "", includeJungle, includeSupport)
	if err != nil {
		return nil, err
	}
	if request.Fallback {
		return s.queryItemSlotFallbacks(ctx, filters, itemContext, buildItemIDs, startingItemIDs, minGames, limit, request.SuppressChampionFallback)
	}
	rows, err := s.repo.QueryItemSlots(ctx, filters, itemContext, buildItemIDs, startingItemIDs, minGames, limit)
	if err != nil {
		return nil, err
	}
	return scopedItemSlotRows(rows, itemSlotScope{Key: "requested", Label: "Requested sample"}), nil
}

func (s Server) openingItemCosts(ctx context.Context, itemContext string) (map[uint32]uint32, error) {
	includeJungle := itemContext == "JUNGLE"
	includeSupport := itemContext == "SUPPORT"
	return s.static.OpeningItemCosts(ctx, "", includeJungle, includeSupport)
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

func startingItemLoadoutRowsResponse(rows []clickhouse.StartingItemLoadoutRow) []map[string]any {
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		results = append(results, map[string]any{
			"championId":           row.ChampionID,
			"role":                 row.Role,
			"opponentChampionId":   row.OpponentChampionID,
			"patchBucket":          row.PatchBucket,
			"rankBucket":           row.RankBucket,
			"itemSignature":        row.ItemSignature,
			"wins":                 row.Wins,
			"games":                row.Games,
			"winRate":              round(row.WinRate * 100),
			"confidence":           round(row.Confidence * 100),
			"sampleQuality":        buildAdviceSampleQuality(row.Games),
			"sampleQualityLabel":   buildAdviceSampleQualityLabel(row.Games),
			"confidencePercentage": round(row.Confidence * 100),
		})
	}
	return results
}

func buildRowsResponse(rows []clickhouse.BuildRow) []map[string]any {
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		results = append(results, map[string]any{
			"championId":           row.ChampionID,
			"role":                 row.Role,
			"opponentChampionId":   row.OpponentChampionID,
			"patchBucket":          row.PatchBucket,
			"rankBucket":           row.RankBucket,
			"finalItemsSignature":  row.FinalItemsSignature,
			"core2Signature":       row.Core2Signature,
			"core3Signature":       row.Core3Signature,
			"runeSignature":        row.RuneSignature,
			"spellSignature":       row.SpellSignature,
			"wins":                 row.Wins,
			"games":                row.Games,
			"winRate":              round(row.WinRate * 100),
			"confidence":           round(row.Confidence * 100),
			"sampleQuality":        buildAdviceSampleQuality(row.Games),
			"sampleQualityLabel":   buildAdviceSampleQualityLabel(row.Games),
			"confidencePercentage": round(row.Confidence * 100),
		})
	}
	return results
}

func buildAdviceSampleResponse(rows []scopedItemSlotRow) map[string]any {
	maxGames := 0
	optionCount := len(rows)
	fallback := false
	scopeLabels := []string{}
	seenLabels := map[string]bool{}
	for _, scoped := range rows {
		if scoped.Row.Games > maxGames {
			maxGames = scoped.Row.Games
		}
		if scoped.Scope.Fallback {
			fallback = true
		}
		if scoped.Scope.Label != "" && !seenLabels[scoped.Scope.Label] {
			seenLabels[scoped.Scope.Label] = true
			scopeLabels = append(scopeLabels, scoped.Scope.Label)
		}
	}
	return map[string]any{
		"maxGames":           maxGames,
		"optionCount":        optionCount,
		"fallbackUsed":       fallback,
		"scopeLabels":        scopeLabels,
		"sampleQuality":      buildAdviceSampleQuality(maxGames),
		"sampleQualityLabel": buildAdviceSampleQualityLabel(maxGames),
	}
}

type buildAdviceDiagnostics struct {
	SelectedSlots          []buildAdviceSlotDiagnostic  `json:"selectedSlots"`
	MissingSlots           []int                        `json:"missingSlots"`
	FallbackSlots          []int                        `json:"fallbackSlots"`
	CurrentPatchExactSlots []int                        `json:"currentPatchExactSlots"`
	AllPatchExactSlots     []int                        `json:"allPatchExactSlots"`
	ChampionWideSlots      []int                        `json:"championWideSlots"`
	ScopeCounts            []buildAdviceScopeDiagnostic `json:"scopeCounts"`
}

type buildAdviceSlotDiagnostic struct {
	Slot               int     `json:"slot"`
	Missing            bool    `json:"missing"`
	CandidateCount     int     `json:"candidateCount"`
	ItemID             uint32  `json:"itemId,omitempty"`
	Games              int     `json:"games,omitempty"`
	WinRate            float64 `json:"winRate,omitempty"`
	SampleScope        string  `json:"sampleScope,omitempty"`
	SampleScopeLabel   string  `json:"sampleScopeLabel,omitempty"`
	Fallback           bool    `json:"fallback"`
	OpponentChampionID uint16  `json:"opponentChampionId,omitempty"`
}

type buildAdviceScopeDiagnostic struct {
	Scope    string `json:"scope"`
	Label    string `json:"label"`
	Fallback bool   `json:"fallback"`
	Rows     int    `json:"rows"`
}

func buildAdviceItemSlotDiagnostics(rows []scopedItemSlotRow) buildAdviceDiagnostics {
	firstBySlot := map[uint8]scopedItemSlotRow{}
	countBySlot := map[uint8]int{}
	scopeCounts := []buildAdviceScopeDiagnostic{}
	scopeIndexes := map[string]int{}
	slotSets := struct {
		missing           map[int]bool
		fallback          map[int]bool
		currentPatchExact map[int]bool
		allPatchExact     map[int]bool
		championWide      map[int]bool
	}{
		missing:           map[int]bool{},
		fallback:          map[int]bool{},
		currentPatchExact: map[int]bool{},
		allPatchExact:     map[int]bool{},
		championWide:      map[int]bool{},
	}

	for _, scoped := range rows {
		countBySlot[scoped.Row.ItemSlot]++
		if _, ok := firstBySlot[scoped.Row.ItemSlot]; !ok {
			firstBySlot[scoped.Row.ItemSlot] = scoped
		}
		scopeKey := scoped.Scope.Key
		if scopeKey == "" {
			scopeKey = "unknown"
		}
		if index, ok := scopeIndexes[scopeKey]; ok {
			scopeCounts[index].Rows++
		} else {
			scopeIndexes[scopeKey] = len(scopeCounts)
			scopeCounts = append(scopeCounts, buildAdviceScopeDiagnostic{
				Scope:    scopeKey,
				Label:    scoped.Scope.Label,
				Fallback: scoped.Scope.Fallback,
				Rows:     1,
			})
		}
	}

	selected := make([]buildAdviceSlotDiagnostic, 0, 6)
	for slot := uint8(1); slot <= 6; slot++ {
		scoped, ok := firstBySlot[slot]
		if !ok {
			slotSets.missing[int(slot)] = true
			selected = append(selected, buildAdviceSlotDiagnostic{
				Slot:           int(slot),
				Missing:        true,
				CandidateCount: 0,
				Fallback:       false,
			})
			continue
		}
		if scoped.Scope.Fallback {
			slotSets.fallback[int(slot)] = true
		}
		switch scoped.Scope.Key {
		case "exact_patch_matchup", "requested":
			if !scoped.Scope.Fallback {
				slotSets.currentPatchExact[int(slot)] = true
			}
		case "all_patch_matchup":
			slotSets.allPatchExact[int(slot)] = true
		case "patch_champion", "all_champion":
			slotSets.championWide[int(slot)] = true
		}
		if scoped.Row.OpponentChampionID == 0 {
			slotSets.championWide[int(slot)] = true
		}
		selected = append(selected, buildAdviceSlotDiagnostic{
			Slot:               int(slot),
			Missing:            false,
			CandidateCount:     countBySlot[slot],
			ItemID:             scoped.Row.ItemID,
			Games:              scoped.Row.Games,
			WinRate:            round(scoped.Row.WinRate * 100),
			SampleScope:        scoped.Scope.Key,
			SampleScopeLabel:   scoped.Scope.Label,
			Fallback:           scoped.Scope.Fallback,
			OpponentChampionID: scoped.Row.OpponentChampionID,
		})
	}

	return buildAdviceDiagnostics{
		SelectedSlots:          selected,
		MissingSlots:           sortedSlotSet(slotSets.missing),
		FallbackSlots:          sortedSlotSet(slotSets.fallback),
		CurrentPatchExactSlots: sortedSlotSet(slotSets.currentPatchExact),
		AllPatchExactSlots:     sortedSlotSet(slotSets.allPatchExact),
		ChampionWideSlots:      sortedSlotSet(slotSets.championWide),
		ScopeCounts:            scopeCounts,
	}
}

func sortedSlotSet(slots map[int]bool) []int {
	out := make([]int, 0, len(slots))
	for slot := 1; slot <= 6; slot++ {
		if slots[slot] {
			out = append(out, slot)
		}
	}
	return out
}

func buildAdviceSampleQuality(games int) string {
	switch {
	case games >= 100:
		return "strong"
	case games >= 30:
		return "moderate"
	case games >= 10:
		return "early"
	case games > 0:
		return "tiny"
	default:
		return "none"
	}
}

func buildAdviceSampleQualityLabel(games int) string {
	switch buildAdviceSampleQuality(games) {
	case "strong":
		return "Strong sample"
	case "moderate":
		return "Moderate sample"
	case "early":
		return "Early sample"
	case "tiny":
		return "Tiny sample"
	default:
		return "No sample"
	}
}

func buildAdviceNotes(hasOpponent bool, matchupSlots, championSlots []scopedItemSlotRow) []string {
	notes := []string{}
	if hasOpponent && len(matchupSlots) == 0 {
		notes = append(notes, "No matchup-specific build sample met the requested threshold yet.")
	}
	if hasOpponent && allItemSlotRowsAreFallback(matchupSlots) && hasChampionWideItemSlotFallback(matchupSlots) {
		notes = append(notes, "No exact matchup item slots met the threshold yet; showing champion-wide slot signals as a baseline.")
	} else if hasOpponent && allItemSlotRowsAreFallback(matchupSlots) {
		notes = append(notes, "No current-patch matchup item slots met the threshold yet; showing exact-matchup rows from broader patch scope.")
	} else if hasOpponent && buildAdviceSampleResponse(matchupSlots)["fallbackUsed"] == true && hasChampionWideItemSlotFallback(matchupSlots) {
		notes = append(notes, "Some matchup slots use champion-wide fallback data where exact samples are thin.")
	} else if hasOpponent && buildAdviceSampleResponse(matchupSlots)["fallbackUsed"] == true {
		notes = append(notes, "Some slots use exact-matchup data from other stored patches.")
	}
	if len(championSlots) == 0 {
		notes = append(notes, "No champion-wide build sample met the requested threshold yet.")
	}
	if len(notes) == 0 {
		notes = append(notes, "Build advice is compiled from stored ranked Solo/Duo games and refreshed summary tables.")
	}
	return notes
}

func allItemSlotRowsAreFallback(rows []scopedItemSlotRow) bool {
	for _, row := range rows {
		if !row.Scope.Fallback {
			return false
		}
	}
	return len(rows) > 0
}

func hasChampionWideItemSlotFallback(rows []scopedItemSlotRow) bool {
	for _, row := range rows {
		if row.Scope.Key == "patch_champion" || row.Scope.Key == "all_champion" || row.Row.OpponentChampionID == 0 {
			return true
		}
	}
	return false
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
		"buildVariants":    championGuideBuildVariantRowsResponse(guide.BuildVariants),
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

func patchStatsResponse(rows []clickhouse.PatchStat, currentPatch string) []map[string]any {
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		results = append(results, map[string]any{
			"patch":              row.Patch,
			"matches":            row.Matches,
			"participantSamples": row.ParticipantSamples,
			"rawMatches":         row.RawMatches,
			"compiledMatches":    row.CompiledMatches,
			"current":            row.Patch == currentPatch,
		})
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

func championGuideBuildVariantRowsResponse(rows []clickhouse.ChampionGuideBuildVariantRow) []map[string]any {
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		results = append(results, map[string]any{
			"variantKey":           row.VariantKey,
			"variantLabel":         row.VariantLabel,
			"variantTags":          row.VariantTags,
			"core2Signature":       row.Core2Signature,
			"core3Signature":       row.Core3Signature,
			"finalItemsSignature":  row.FinalItemsSignature,
			"runeSignature":        row.RuneSignature,
			"spellSignature":       row.SpellSignature,
			"skillOrderSignature":  row.SkillOrderSignature,
			"skillOrderWins":       row.SkillOrderWins,
			"skillOrderGames":      row.SkillOrderGames,
			"skillOrderWinRate":    round(row.SkillOrderWinRate * 100),
			"skillOrderConfidence": round(row.SkillOrderConfidence * 100),
			"wins":                 row.Wins,
			"games":                row.Games,
			"winRate":              round(row.WinRate * 100),
			"confidence":           round(row.Confidence * 100),
			"buildCount":           row.BuildCount,
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

func (s Server) queryItemSlotFallbacks(ctx context.Context, filters map[string]string, itemContext string, buildItemIDs, startingItemIDs []uint32, minGames, limit int, suppressChampionFallback bool) ([]scopedItemSlotRow, error) {
	scopes := itemSlotFallbackScopesWithOptions(filters, suppressChampionFallback)
	out := []scopedItemSlotRow{}
	coveredSlots := map[uint8]int{}
	for _, scope := range scopes {
		rows, err := s.repo.QueryItemSlots(ctx, scope.Filters, itemContext, buildItemIDs, startingItemIDs, minGames, limit)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if coveredSlots[row.ItemSlot] >= limit {
				continue
			}
			out = append(out, scopedItemSlotRow{Row: row, Scope: scope})
			coveredSlots[row.ItemSlot]++
			if itemSlotFallbackComplete(coveredSlots, limit) {
				return out, nil
			}
		}
	}
	return out, nil
}

func itemSlotFallbackComplete(coveredSlots map[uint8]int, limit int) bool {
	if limit <= 0 {
		limit = 1
	}
	for slot := uint8(0); slot <= 6; slot++ {
		if coveredSlots[slot] < limit {
			return false
		}
	}
	return true
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
	return itemSlotFallbackScopesWithOptions(filters, false)
}

func itemSlotFallbackScopesWithOptions(filters map[string]string, suppressChampionFallback bool) []itemSlotScope {
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
	if opponent != "" && !suppressChampionFallback {
		championPatch := cloneStringMap(filters)
		championPatch["opponent_champion_id"] = ""
		addScope("patch_champion", itemSlotChampionScopeLabel(patch != ""), championPatch)
	}
	if (patch != "" || opponent != "") && !suppressChampionFallback {
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
	rows, err := s.repo.ChampionRoleRatesForPatch(r.Context(), championIDs, queueID, query.Get("patch"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, championRoleRatesResponse(rows))
}

func championRoleRatesResponse(rows []clickhouse.ChampionRoleRate) map[string]any {
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
	return map[string]any{"results": results}
}
