package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"winrift/core/internal/analytics"
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
	FirstSeen  time.Time
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

type SummonerBuildRecord struct {
	Platform            string
	PUUID               string
	QueueID             uint16
	ChampionID          uint16
	Role                string
	FinalItemsSignature string
	Core2Signature      string
	Core3Signature      string
	RuneSignature       string
	SpellSignature      string
	Games               int
	Wins                int
	Losses              int
	Kills               int
	Deaths              int
	Assists             int
	AvgKills            float64
	AvgDeaths           float64
	AvgAssists          float64
	KDA                 float64
	WinRate             float64
}

type SummonerProfileRefreshResult struct {
	ProfileRows      int
	ChampionRows     int
	ChampionRoleRows int
	IdentityRows     int
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
	stats, err := r.summonerProfileStatsFromSummary(ctx, platform, puuid, queueID)
	if err == nil && stats.Games > 0 {
		return stats, nil
	}
	if err != nil && !IsNoRows(err) {
		return SummonerProfileStats{}, err
	}
	return r.summonerProfileStatsFromParticipants(ctx, platform, puuid, queueID)
}

func (r *Repository) summonerProfileStatsFromSummary(ctx context.Context, platform, puuid string, queueID uint16) (SummonerProfileStats, error) {
	stats := SummonerProfileStats{PUUID: puuid, Platform: platform, QueueID: queueID}
	var games, wins, kills, deaths, assists uint64
	err := r.db.QueryRowContext(
		ctx,
		`SELECT
			games,
			wins,
			kills,
			deaths,
			assists,
			first_seen_at,
			last_seen_at
		FROM summoner_profile_summary FINAL
		WHERE platform = ? AND puuid = ? AND queue_id = ?
		ORDER BY compiled_at DESC
		LIMIT 1`,
		platform,
		puuid,
		queueID,
	).Scan(&games, &wins, &kills, &deaths, &assists, &stats.FirstSeen, &stats.LastSeen)
	if err != nil {
		return SummonerProfileStats{}, err
	}
	stats.Games = int(games)
	stats.Wins = int(wins)
	stats.Losses = int(games - wins)
	stats.Kills = int(kills)
	stats.Deaths = int(deaths)
	stats.Assists = int(assists)
	applySummonerProfileAverages(&stats)
	return stats, nil
}

func (r *Repository) summonerProfileStatsFromParticipants(ctx context.Context, platform, puuid string, queueID uint16) (SummonerProfileStats, error) {
	stats := SummonerProfileStats{PUUID: puuid, Platform: platform, QueueID: queueID}
	var games, wins, kills, deaths, assists uint64
	err := r.db.QueryRowContext(
		ctx,
		`SELECT
			count(),
			sum(win),
			sum(kills),
			sum(deaths),
			sum(assists),
			min(multiIf(rm.game_start_timestamp > 0, toDateTime(intDiv(rm.game_start_timestamp, 1000)), p.ingested_at)),
			max(multiIf(rm.game_start_timestamp > 0, toDateTime(intDiv(rm.game_start_timestamp, 1000)), p.ingested_at))
		FROM participants AS p FINAL
		LEFT JOIN raw_matches AS rm FINAL
			ON rm.match_id = p.match_id
			AND rm.platform = p.platform
		WHERE p.platform = ? AND p.puuid = ? AND p.queue_id = ?`,
		platform,
		puuid,
		queueID,
	).Scan(&games, &wins, &kills, &deaths, &assists, &stats.FirstSeen, &stats.LastSeen)
	if err != nil {
		return SummonerProfileStats{}, err
	}
	stats.Games = int(games)
	stats.Wins = int(wins)
	stats.Losses = int(games - wins)
	stats.Kills = int(kills)
	stats.Deaths = int(deaths)
	stats.Assists = int(assists)
	applySummonerProfileAverages(&stats)
	return stats, nil
}

func (r *Repository) RefreshSummonerProfileAnalytics(ctx context.Context, queueID uint16) (SummonerProfileRefreshResult, error) {
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE summoner_identity_summary DELETE WHERE 1 SETTINGS mutations_sync = 2`); err != nil {
		return SummonerProfileRefreshResult{}, err
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE summoner_profile_summary DELETE WHERE queue_id = ? SETTINGS mutations_sync = 2`, queueID); err != nil {
		return SummonerProfileRefreshResult{}, err
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE summoner_champion_summary DELETE WHERE queue_id = ? SETTINGS mutations_sync = 2`, queueID); err != nil {
		return SummonerProfileRefreshResult{}, err
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE summoner_champion_role_summary DELETE WHERE queue_id = ? SETTINGS mutations_sync = 2`, queueID); err != nil {
		return SummonerProfileRefreshResult{}, err
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO summoner_identity_summary
			(platform, puuid, game_name, tag_line, profile_icon_id, summoner_level, last_seen_at, compiled_at)
		WITH latest_alias AS
		(
			SELECT
				puuid,
				platform,
				argMax(game_name, last_seen_at) AS game_name,
				argMax(tag_line, last_seen_at) AS tag_line,
				max(last_seen_at) AS alias_last_seen_at
			FROM riot_account_aliases FINAL
			GROUP BY platform, puuid
		),
		latest_account AS
		(
			SELECT
				puuid,
				platform,
				argMax(profile_icon_id, fetched_at) AS profile_icon_id,
				argMax(summoner_level, fetched_at) AS summoner_level
			FROM summoner_account_snapshots FINAL
			GROUP BY platform, puuid
		),
		latest_raw_profile AS
		(
			SELECT
				platform,
				puuid,
				argMax(raw_profile_icon_id, game_start_timestamp) AS profile_icon_id
			FROM
			(
				SELECT
					platform,
					game_start_timestamp,
					JSONExtractString(participant_json, 'puuid') AS puuid,
					toUInt32(JSONExtractUInt(participant_json, 'profileIcon')) AS raw_profile_icon_id
				FROM raw_matches FINAL
				ARRAY JOIN JSONExtractArrayRaw(raw_json, 'info', 'participants') AS participant_json
				WHERE queue_id = ?
			)
			WHERE puuid != '' AND raw_profile_icon_id > 0
			GROUP BY platform, puuid
		)
		SELECT
			a.platform,
			a.puuid,
			a.game_name,
			a.tag_line,
			if(ifNull(s.profile_icon_id, 0) > 0, ifNull(s.profile_icon_id, 0), ifNull(rp.profile_icon_id, 0)) AS profile_icon_id,
			ifNull(s.summoner_level, 0) AS summoner_level,
			a.alias_last_seen_at,
			now() AS compiled_at
		FROM latest_alias AS a
		LEFT JOIN latest_account AS s
			ON s.platform = a.platform AND s.puuid = a.puuid
		LEFT JOIN latest_raw_profile AS rp
			ON rp.platform = a.platform AND rp.puuid = a.puuid
		WHERE a.game_name != '' AND a.tag_line != ''`, queueID); err != nil {
		return SummonerProfileRefreshResult{}, err
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO summoner_profile_summary
			(platform, queue_id, puuid, games, wins, kills, deaths, assists, first_seen_at, last_seen_at, compiled_at)
		SELECT
			p.platform,
			p.queue_id,
			p.puuid,
			toUInt64(count()) AS games,
			toUInt64(sum(p.win)) AS wins,
			toUInt64(sum(p.kills)) AS kills,
			toUInt64(sum(p.deaths)) AS deaths,
			toUInt64(sum(p.assists)) AS assists,
			min(multiIf(rm.game_start_timestamp > 0, toDateTime(intDiv(rm.game_start_timestamp, 1000)), p.ingested_at)) AS first_seen_at,
			max(multiIf(rm.game_start_timestamp > 0, toDateTime(intDiv(rm.game_start_timestamp, 1000)), p.ingested_at)) AS last_seen_at,
			now() AS compiled_at
		FROM participants AS p FINAL
		LEFT JOIN raw_matches AS rm FINAL
			ON rm.match_id = p.match_id
			AND rm.platform = p.platform
		WHERE p.queue_id = ?
			AND p.puuid != ''
		GROUP BY p.platform, p.queue_id, p.puuid`, queueID); err != nil {
		return SummonerProfileRefreshResult{}, err
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO summoner_champion_summary
			(platform, queue_id, puuid, champion_id, games, wins, kills, deaths, assists, first_seen_at, last_seen_at, compiled_at)
		SELECT
			p.platform,
			p.queue_id,
			p.puuid,
			p.champion_id,
			toUInt64(count()) AS games,
			toUInt64(sum(p.win)) AS wins,
			toUInt64(sum(p.kills)) AS kills,
			toUInt64(sum(p.deaths)) AS deaths,
			toUInt64(sum(p.assists)) AS assists,
			min(multiIf(rm.game_start_timestamp > 0, toDateTime(intDiv(rm.game_start_timestamp, 1000)), p.ingested_at)) AS first_seen_at,
			max(multiIf(rm.game_start_timestamp > 0, toDateTime(intDiv(rm.game_start_timestamp, 1000)), p.ingested_at)) AS last_seen_at,
			now() AS compiled_at
		FROM participants AS p FINAL
		LEFT JOIN raw_matches AS rm FINAL
			ON rm.match_id = p.match_id
			AND rm.platform = p.platform
		WHERE p.queue_id = ?
			AND p.puuid != ''
			AND p.champion_id > 0
	GROUP BY p.platform, p.queue_id, p.puuid, p.champion_id`, queueID); err != nil {
		return SummonerProfileRefreshResult{}, err
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO summoner_champion_role_summary
			(platform, queue_id, puuid, champion_id, role, games, wins, kills, deaths, assists, first_seen_at, last_seen_at, compiled_at)
		SELECT
			p.platform,
			p.queue_id,
			p.puuid,
			p.champion_id,
			p.role,
			toUInt64(count()) AS games,
			toUInt64(sum(p.win)) AS wins,
			toUInt64(sum(p.kills)) AS kills,
			toUInt64(sum(p.deaths)) AS deaths,
			toUInt64(sum(p.assists)) AS assists,
			min(multiIf(rm.game_start_timestamp > 0, toDateTime(intDiv(rm.game_start_timestamp, 1000)), p.ingested_at)) AS first_seen_at,
			max(multiIf(rm.game_start_timestamp > 0, toDateTime(intDiv(rm.game_start_timestamp, 1000)), p.ingested_at)) AS last_seen_at,
			now() AS compiled_at
		FROM participants AS p FINAL
		LEFT JOIN raw_matches AS rm FINAL
			ON rm.match_id = p.match_id
			AND rm.platform = p.platform
		WHERE p.queue_id = ?
			AND p.puuid != ''
			AND p.champion_id > 0
			AND p.role IN ('TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY')
		GROUP BY p.platform, p.queue_id, p.puuid, p.champion_id, p.role`, queueID); err != nil {
		return SummonerProfileRefreshResult{}, err
	}
	result := SummonerProfileRefreshResult{}
	if err := r.db.QueryRowContext(ctx, `SELECT count() FROM summoner_profile_summary FINAL WHERE queue_id = ?`, queueID).Scan(&result.ProfileRows); err != nil {
		return SummonerProfileRefreshResult{}, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT count() FROM summoner_champion_summary FINAL WHERE queue_id = ?`, queueID).Scan(&result.ChampionRows); err != nil {
		return SummonerProfileRefreshResult{}, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT count() FROM summoner_champion_role_summary FINAL WHERE queue_id = ?`, queueID).Scan(&result.ChampionRoleRows); err != nil {
		return SummonerProfileRefreshResult{}, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT count() FROM summoner_identity_summary FINAL`).Scan(&result.IdentityRows); err != nil {
		return SummonerProfileRefreshResult{}, err
	}
	return result, nil
}

func (r *Repository) SummonerTopChampions(ctx context.Context, platform, puuid string, queueID uint16, limit int) ([]ChampionPerformance, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := r.summonerTopChampionsFromSummary(ctx, platform, puuid, queueID, limit)
	if err == nil && len(rows) > 0 {
		return rows, nil
	}
	if err != nil {
		return nil, err
	}
	return r.summonerTopChampionsFromParticipants(ctx, platform, puuid, queueID, limit)
}

func (r *Repository) summonerTopChampionsFromSummary(ctx context.Context, platform, puuid string, queueID uint16, limit int) ([]ChampionPerformance, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			champion_id,
			games,
			wins,
			kills,
			deaths,
			assists
		FROM summoner_champion_summary FINAL
		WHERE platform = ? AND puuid = ? AND queue_id = ?
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

func (r *Repository) summonerTopChampionsFromParticipants(ctx context.Context, platform, puuid string, queueID uint16, limit int) ([]ChampionPerformance, error) {
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

func (r *Repository) SummonerTopChampionRoles(ctx context.Context, platform, puuid string, queueID uint16, limit int) ([]ChampionPerformance, error) {
	if limit <= 0 {
		limit = 30
	}
	rows, err := r.summonerTopChampionRolesFromSummary(ctx, platform, puuid, queueID, limit)
	if err == nil && len(rows) > 0 {
		return rows, nil
	}
	if err != nil {
		return nil, err
	}
	return r.summonerTopChampionRolesFromParticipants(ctx, platform, puuid, queueID, limit)
}

func (r *Repository) summonerTopChampionRolesFromSummary(ctx context.Context, platform, puuid string, queueID uint16, limit int) ([]ChampionPerformance, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			champion_id,
			role,
			games,
			wins,
			kills,
			deaths,
			assists
		FROM summoner_champion_role_summary FINAL
		WHERE platform = ? AND puuid = ? AND queue_id = ?
		ORDER BY games DESC, wins / games DESC, champion_id ASC, role ASC
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
		if err := rows.Scan(&row.ChampionID, &row.Role, &games, &wins, &kills, &deaths, &assists); err != nil {
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

func (r *Repository) summonerTopChampionRolesFromParticipants(ctx context.Context, platform, puuid string, queueID uint16, limit int) ([]ChampionPerformance, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			champion_id,
			role,
			count() AS games,
			sum(win) AS wins,
			sum(kills) AS kills,
			sum(deaths) AS deaths,
			sum(assists) AS assists
		FROM participants FINAL
		WHERE platform = ? AND puuid = ? AND queue_id = ?
			AND champion_id > 0
			AND role IN ('TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY')
		GROUP BY champion_id, role
		ORDER BY games DESC, wins / games DESC, champion_id ASC, role ASC
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
		if err := rows.Scan(&row.ChampionID, &row.Role, &games, &wins, &kills, &deaths, &assists); err != nil {
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
			AND rm.platform = p.platform
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

func (r *Repository) SummonerBuilds(ctx context.Context, platform, puuid string, queueID uint16, limit int) ([]SummonerBuildRecord, error) {
	if limit <= 0 {
		limit = 24
	}
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			champion_id,
			role,
			final_items_signature,
			core2_signature,
			core3_signature,
			rune_signature,
			spell_signature,
			count() AS games,
			sum(win) AS wins,
			sum(kills) AS kills,
			sum(deaths) AS deaths,
			sum(assists) AS assists
		FROM participants FINAL
		WHERE platform = ? AND puuid = ? AND queue_id = ?
			AND champion_id > 0
			AND (final_items_signature != '' OR core3_signature != '')
		GROUP BY
			champion_id,
			role,
			final_items_signature,
			core2_signature,
			core3_signature,
			rune_signature,
			spell_signature
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
	out := []SummonerBuildRecord{}
	for rows.Next() {
		var row SummonerBuildRecord
		var games, wins, kills, deaths, assists uint64
		if err := rows.Scan(
			&row.ChampionID,
			&row.Role,
			&row.FinalItemsSignature,
			&row.Core2Signature,
			&row.Core3Signature,
			&row.RuneSignature,
			&row.SpellSignature,
			&games,
			&wins,
			&kills,
			&deaths,
			&assists,
		); err != nil {
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
		applySummonerBuildAverages(&row)
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

func applySummonerBuildAverages(build *SummonerBuildRecord) {
	if build.Games <= 0 {
		return
	}
	build.AvgKills = float64(build.Kills) / float64(build.Games)
	build.AvgDeaths = float64(build.Deaths) / float64(build.Games)
	build.AvgAssists = float64(build.Assists) / float64(build.Games)
	build.WinRate = float64(build.Wins) / float64(build.Games)
	if build.Deaths > 0 {
		build.KDA = float64(build.Kills+build.Assists) / float64(build.Deaths)
	} else {
		build.KDA = float64(build.Kills + build.Assists)
	}
}
