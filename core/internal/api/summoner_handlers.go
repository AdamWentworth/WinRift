package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"winrift/core/internal/analytics"
	"winrift/core/internal/clickhouse"
)

const summonerProfileResponseCacheTTL = 5 * time.Minute
const summonerLeaderboardResponseCacheTTL = 2 * time.Minute

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
	cacheKey := "summoner-profile:" + alias.Platform + ":" + strings.ToLower(alias.GameName) + ":" + strings.ToLower(alias.TagLine) + ":" + alias.PUUID
	if body, ok := s.responseCache.get(cacheKey); ok {
		writeJSONBytes(w, http.StatusOK, body, true)
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
	topChampionRoles, err := s.repo.SummonerTopChampionRoles(r.Context(), alias.Platform, alias.PUUID, queueID, 60)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recentMatches, err := s.repo.SummonerRecentMatches(r.Context(), alias.Platform, alias.PUUID, queueID, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	topBuilds, err := s.repo.SummonerBuilds(r.Context(), alias.Platform, alias.PUUID, queueID, 36)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	summonerSnapshot := s.summonerAccountSnapshot(r.Context(), alias.Platform, alias.PUUID)
	response := map[string]any{
		"account":          accountAliasResponse(alias),
		"summary":          summonerProfileStatsResponse(stats),
		"topChampions":     championPerformanceRowsResponse(topChampions),
		"topChampionRoles": championPerformanceRowsResponse(topChampionRoles),
		"recentMatches":    summonerRecentMatchesResponse(recentMatches),
		"topBuilds":        summonerBuildRowsResponse(topBuilds),
	}
	if summonerSnapshot != nil {
		response["summoner"] = summonerAccountSnapshotResponse(*summonerSnapshot)
	}
	rank, err := s.repo.LatestRankSnapshot(r.Context(), alias.Platform, alias.PUUID, "RANKED_SOLO_5x5")
	if err == nil {
		response["rank"] = rankSnapshotResponse(rank)
	} else if !clickhouse.IsNoRows(err) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeCachedJSON(w, http.StatusOK, cacheKey, summonerProfileResponseCacheTTL, response)
}

func (s Server) summonerLeaderboard(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	platform := s.defaultPlatform(query.Get("platform"))
	limit := queryInt(query.Get("limit"), 50)
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	cacheKey := fmt.Sprintf("summoner-leaderboard:%s:%d", platform, limit)
	if body, ok := s.responseCache.get(cacheKey); ok {
		writeJSONBytes(w, http.StatusOK, body, true)
		return
	}
	rows, err := s.repo.SummonerRankLeaderboard(r.Context(), platform, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeCachedJSON(w, http.StatusOK, cacheKey, summonerLeaderboardResponseCacheTTL, map[string]any{
		"platform":  platform,
		"queueType": "RANKED_SOLO_5x5",
		"results":   summonerLeaderboardRowsResponse(rows),
	})
}

func championPerformanceResponse(performance clickhouse.ChampionPerformance) map[string]any {
	response := map[string]any{
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
	if performance.Role != "" {
		response["role"] = performance.Role
	}
	return response
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

func summonerBuildRowsResponse(rows []clickhouse.SummonerBuildRecord) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"platform":            row.Platform,
			"queueId":             row.QueueID,
			"championId":          row.ChampionID,
			"role":                row.Role,
			"finalItemsSignature": row.FinalItemsSignature,
			"core2Signature":      row.Core2Signature,
			"core3Signature":      row.Core3Signature,
			"runeSignature":       row.RuneSignature,
			"spellSignature":      row.SpellSignature,
			"games":               row.Games,
			"wins":                row.Wins,
			"losses":              row.Losses,
			"kills":               row.Kills,
			"deaths":              row.Deaths,
			"assists":             row.Assists,
			"avgKills":            round(row.AvgKills),
			"avgDeaths":           round(row.AvgDeaths),
			"avgAssists":          round(row.AvgAssists),
			"kda":                 round(row.KDA),
			"winRate":             round(row.WinRate * 100),
		})
	}
	return out
}

func summonerLeaderboardRowsResponse(rows []clickhouse.SummonerLeaderboardRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for index, row := range rows {
		totalGames := row.Wins + row.Losses
		rank := analytics.RankSnapshot{
			QueueType:    "RANKED_SOLO_5x5",
			Tier:         row.Tier,
			Division:     row.Division,
			LeaguePoints: row.LeaguePoints,
			Wins:         row.Wins,
			Losses:       row.Losses,
			RankBucket:   row.RankBucket,
			FetchedAt:    row.FetchedAt,
			ExpiresAt:    row.ExpiresAt,
		}
		storedWinRate := 0.0
		if row.StoredGames > 0 {
			storedWinRate = round(float64(row.StoredWins) / float64(row.StoredGames) * 100)
		}
		out = append(out, map[string]any{
			"rank":          index + 1,
			"puuid":         row.PUUID,
			"platform":      row.Platform,
			"gameName":      row.GameName,
			"tagLine":       row.TagLine,
			"ranked":        rankSnapshotResponse(rank),
			"profileIconId": row.ProfileIconID,
			"summonerLevel": row.SummonerLevel,
			"rankedGames":   totalGames,
			"storedGames":   row.StoredGames,
			"storedWins":    row.StoredWins,
			"storedWinRate": storedWinRate,
			"lastSeenAt":    row.LastSeenAt,
		})
	}
	return out
}
