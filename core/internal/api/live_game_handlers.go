package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"winrift/core/internal/analytics"
	"winrift/core/internal/clickhouse"
	"winrift/core/internal/riot"
)

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
