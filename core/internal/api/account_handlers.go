package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"winrift/core/internal/clickhouse"
	"winrift/core/internal/riot"
)

const summonerAccountSnapshotTTL = 7 * 24 * time.Hour

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
		s.storeSummonerAccountSnapshot(r.Context(), summoner, platform)
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

func (s Server) storeSummonerAccountSnapshot(ctx context.Context, summoner *riot.Summoner, platform string) {
	if summoner == nil {
		return
	}
	now := time.Now()
	if err := s.repo.UpsertSummonerAccountSnapshot(ctx, clickhouse.SummonerAccountSnapshot{
		PUUID:         summoner.PUUID,
		Platform:      platform,
		SummonerID:    summoner.ID,
		AccountID:     summoner.AccountID,
		ProfileIconID: uint32(max(summoner.ProfileIconID, 0)),
		SummonerLevel: uint64(max(summoner.SummonerLevel, 0)),
		FetchedAt:     now,
		ExpiresAt:     now.Add(summonerAccountSnapshotTTL),
	}); err != nil {
		log.Printf("summoner account snapshot store failed puuid=%s platform=%s err=%v", shortValue(summoner.PUUID), platform, err)
	}
}

func (s Server) summonerAccountSnapshot(ctx context.Context, platform, puuid string) *clickhouse.SummonerAccountSnapshot {
	now := time.Now()
	var stale *clickhouse.SummonerAccountSnapshot
	snapshot, err := s.repo.LatestSummonerAccountSnapshot(ctx, platform, puuid)
	if err == nil {
		if snapshot.ExpiresAt.IsZero() || snapshot.ExpiresAt.After(now) {
			return &snapshot
		}
		stale = &snapshot
	} else if !clickhouse.IsNoRows(err) {
		log.Printf("summoner account snapshot lookup failed puuid=%s platform=%s err=%v", shortValue(puuid), platform, err)
		return nil
	}
	if s.cfg.RiotAPIKey == "" || riot.AuthFailureMarkerExists(s.cfg) {
		return stale
	}
	reservation, err := s.repo.ReserveRiotRequests(
		ctx,
		platform,
		"api:summoner-profile-metadata",
		1,
		s.cfg.CollectorUsableRequestsPerRegion(),
		s.cfg.CollectorRateLimitWindow,
		now,
	)
	if err != nil {
		log.Printf("summoner account snapshot budget check failed puuid=%s platform=%s err=%v", shortValue(puuid), platform, err)
		return stale
	}
	if reservation.Granted < 1 {
		log.Printf("summoner account snapshot refresh skipped puuid=%s platform=%s wait=%s used=%d limit=%d", shortValue(puuid), platform, reservation.Wait.Round(time.Second), reservation.Used, reservation.Limit)
		return stale
	}
	summoner, err := s.riot.SummonerByPUUID(ctx, puuid, platform)
	if err != nil {
		log.Printf("summoner account snapshot refresh failed puuid=%s platform=%s err=%v", shortValue(puuid), platform, err)
		return stale
	}
	if summoner == nil {
		return stale
	}
	fresh := clickhouse.SummonerAccountSnapshot{
		PUUID:         summoner.PUUID,
		Platform:      platform,
		SummonerID:    summoner.ID,
		AccountID:     summoner.AccountID,
		ProfileIconID: uint32(max(summoner.ProfileIconID, 0)),
		SummonerLevel: uint64(max(summoner.SummonerLevel, 0)),
		FetchedAt:     now,
		ExpiresAt:     now.Add(summonerAccountSnapshotTTL),
	}
	if fresh.PUUID == "" {
		fresh.PUUID = puuid
	}
	if err := s.repo.UpsertSummonerAccountSnapshot(ctx, fresh); err != nil {
		log.Printf("summoner account snapshot store failed puuid=%s platform=%s err=%v", shortValue(puuid), platform, err)
		return stale
	}
	return &fresh
}

func accountAliasResponse(alias clickhouse.AccountAlias) map[string]any {
	return map[string]any{
		"puuid":    alias.PUUID,
		"platform": alias.Platform,
		"gameName": alias.GameName,
		"tagLine":  alias.TagLine,
	}
}

func summonerAccountSnapshotResponse(snapshot clickhouse.SummonerAccountSnapshot) map[string]any {
	return map[string]any{
		"puuid":          snapshot.PUUID,
		"platform":       snapshot.Platform,
		"summonerId":     snapshot.SummonerID,
		"accountId":      snapshot.AccountID,
		"profileIconId":  snapshot.ProfileIconID,
		"summonerLevel":  snapshot.SummonerLevel,
		"fetchedAt":      snapshot.FetchedAt,
		"expiresAt":      snapshot.ExpiresAt,
		"cacheExpiresAt": snapshot.ExpiresAt,
	}
}
