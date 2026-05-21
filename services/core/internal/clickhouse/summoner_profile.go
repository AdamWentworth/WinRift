package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"winrift/services/core/internal/analytics"
)

type SummonerProfileStats struct {
	PUUID      string
	Platform   string
	QueueID    uint16
	Games      int
	Wins       int
	Losses     int
	Kills      int
	Deaths     int
	Assists    int
	AvgKills   float64
	AvgDeaths  float64
	AvgAssists float64
	KDA        float64
	WinRate    float64
	LastSeen   time.Time
}

type SummonerRecentMatch struct {
	MatchID            string
	Platform           string
	Patch              string
	QueueID            uint16
	ChampionID         uint16
	Role               string
	Win                bool
	Kills              int
	Deaths             int
	Assists            int
	GameStartTimestamp uint64
	DurationSeconds    uint32
}

func (r *Repository) FindAccountAlias(ctx context.Context, platform, gameName, tagLine string) (AccountAlias, error) {
	var alias AccountAlias
	err := r.db.QueryRowContext(
		ctx,
		`SELECT
			puuid,
			platform,
			argMax(game_name, updated_at) AS game_name,
			tag_line,
			max(last_seen_at) AS last_seen_at
		FROM riot_account_aliases FINAL
		WHERE platform = ? AND game_name_normalized = ? AND tag_line = ?
		GROUP BY puuid, platform, tag_line
		ORDER BY last_seen_at DESC
		LIMIT 1`,
		platform,
		normalizeAliasName(gameName),
		tagLine,
	).Scan(&alias.PUUID, &alias.Platform, &alias.GameName, &alias.TagLine, &alias.LastSeen)
	if err != nil {
		return AccountAlias{}, err
	}
	return alias, nil
}

func (r *Repository) LatestRankSnapshot(ctx context.Context, platform, puuid, queueType string) (analytics.RankSnapshot, error) {
	if queueType == "" {
		queueType = rankedSoloQueueType
	}
	var snapshot analytics.RankSnapshot
	var leaguePoints int16
	var wins, losses uint32
	err := r.db.QueryRowContext(
		ctx,
		`SELECT
			puuid,
			platform,
			queue_type,
			tier,
			division,
			league_points,
			wins,
			losses,
			rank_bucket,
			fetched_at,
			expires_at
		FROM summoner_rank_snapshots FINAL
		WHERE platform = ? AND puuid = ? AND queue_type = ?
		ORDER BY fetched_at DESC
		LIMIT 1`,
		platform,
		puuid,
		queueType,
	).Scan(
		&snapshot.PUUID,
		&snapshot.Platform,
		&snapshot.QueueType,
		&snapshot.Tier,
		&snapshot.Division,
		&leaguePoints,
		&wins,
		&losses,
		&snapshot.RankBucket,
		&snapshot.FetchedAt,
		&snapshot.ExpiresAt,
	)
	if err != nil {
		return analytics.RankSnapshot{}, err
	}
	snapshot.LeaguePoints = int(leaguePoints)
	snapshot.Wins = int(wins)
	snapshot.Losses = int(losses)
	return snapshot, nil
}

func (r *Repository) SummonerProfileStats(ctx context.Context, platform, puuid string, queueID uint16) (SummonerProfileStats, error) {
	stats := SummonerProfileStats{PUUID: puuid, Platform: platform, QueueID: queueID}
	var games, wins, kills, deaths, assists uint64
	var lastSeen time.Time
	err := r.db.QueryRowContext(
		ctx,
		`SELECT
			count(),
			sum(win),
			sum(kills),
			sum(deaths),
			sum(assists),
			max(ingested_at)
		FROM participants FINAL
		WHERE platform = ? AND puuid = ? AND queue_id = ?`,
		platform,
		puuid,
		queueID,
	).Scan(&games, &wins, &kills, &deaths, &assists, &lastSeen)
	if err != nil {
		return SummonerProfileStats{}, err
	}
	stats.Games = int(games)
	stats.Wins = int(wins)
	stats.Losses = int(games - wins)
	stats.Kills = int(kills)
	stats.Deaths = int(deaths)
	stats.Assists = int(assists)
	stats.LastSeen = lastSeen
	applySummonerProfileAverages(&stats)
	return stats, nil
}

func (r *Repository) SummonerTopChampions(ctx context.Context, platform, puuid string, queueID uint16, limit int) ([]ChampionPerformance, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			champion_id,
			count() AS games,
			sum(win) AS wins,
			sum(kills) AS kills,
			sum(deaths) AS deaths,
			sum(assists) AS assists
		FROM participants FINAL
		WHERE platform = ? AND puuid = ? AND queue_id = ?
		GROUP BY champion_id
		ORDER BY games DESC, wins / games DESC, champion_id ASC
		LIMIT ?`,
		platform,
		puuid,
		queueID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChampionPerformance{}
	for rows.Next() {
		var row ChampionPerformance
		var games, wins, kills, deaths, assists uint64
		if err := rows.Scan(&row.ChampionID, &games, &wins, &kills, &deaths, &assists); err != nil {
			return nil, err
		}
		row.PUUID = puuid
		row.Platform = platform
		row.QueueID = queueID
		row.Games = int(games)
		row.Wins = int(wins)
		row.Losses = int(games - wins)
		row.Kills = int(kills)
		row.Deaths = int(deaths)
		row.Assists = int(assists)
		applyChampionPerformanceAverages(&row)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) SummonerRecentMatches(ctx context.Context, platform, puuid string, queueID uint16, limit int) ([]SummonerRecentMatch, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			p.match_id,
			p.platform,
			p.patch,
			p.queue_id,
			p.champion_id,
			p.role,
			p.win,
			p.kills,
			p.deaths,
			p.assists,
			rm.game_start_timestamp,
			rm.duration_seconds
		FROM participants AS p FINAL
		LEFT JOIN raw_matches AS rm FINAL
			ON rm.match_id = p.match_id
		WHERE p.platform = ? AND p.puuid = ? AND p.queue_id = ?
		ORDER BY rm.game_start_timestamp DESC, p.match_id DESC
		LIMIT ?`,
		platform,
		puuid,
		queueID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SummonerRecentMatch{}
	for rows.Next() {
		var row SummonerRecentMatch
		var win uint8
		var kills, deaths, assists uint16
		if err := rows.Scan(
			&row.MatchID,
			&row.Platform,
			&row.Patch,
			&row.QueueID,
			&row.ChampionID,
			&row.Role,
			&win,
			&kills,
			&deaths,
			&assists,
			&row.GameStartTimestamp,
			&row.DurationSeconds,
		); err != nil {
			return nil, err
		}
		row.Win = win > 0
		row.Kills = int(kills)
		row.Deaths = int(deaths)
		row.Assists = int(assists)
		out = append(out, row)
	}
	return out, rows.Err()
}

func IsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func applySummonerProfileAverages(stats *SummonerProfileStats) {
	if stats.Games <= 0 {
		return
	}
	stats.AvgKills = float64(stats.Kills) / float64(stats.Games)
	stats.AvgDeaths = float64(stats.Deaths) / float64(stats.Games)
	stats.AvgAssists = float64(stats.Assists) / float64(stats.Games)
	stats.WinRate = float64(stats.Wins) / float64(stats.Games)
	if stats.Deaths > 0 {
		stats.KDA = float64(stats.Kills+stats.Assists) / float64(stats.Deaths)
	} else {
		stats.KDA = float64(stats.Kills + stats.Assists)
	}
}

func applyChampionPerformanceAverages(performance *ChampionPerformance) {
	if performance.Games <= 0 {
		return
	}
	performance.AvgKills = float64(performance.Kills) / float64(performance.Games)
	performance.AvgDeaths = float64(performance.Deaths) / float64(performance.Games)
	performance.AvgAssists = float64(performance.Assists) / float64(performance.Games)
	performance.WinRate = float64(performance.Wins) / float64(performance.Games)
	if performance.Deaths > 0 {
		performance.KDA = float64(performance.Kills+performance.Assists) / float64(performance.Deaths)
	} else {
		performance.KDA = float64(performance.Kills + performance.Assists)
	}
}
