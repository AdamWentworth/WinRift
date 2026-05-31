package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"winrift/services/core/internal/analytics"
)

const rankedSoloQueueType = "RANKED_SOLO_5x5"

type SummonerLeaderboardRow struct {
	PUUID         string
	Platform      string
	GameName      string
	TagLine       string
	Tier          string
	Division      string
	LeaguePoints  int
	Wins          int
	Losses        int
	RankBucket    string
	FetchedAt     time.Time
	ExpiresAt     time.Time
	LastSeenAt    time.Time
	ProfileIconID uint32
	SummonerLevel uint64
	StoredGames   int
	StoredWins    int
}

func (r *Repository) FetchRankCandidates(ctx context.Context, platform string, limit int, now time.Time) ([]analytics.RankCandidate, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			puuid,
			platform,
			count() AS participant_rows,
			countIf(rank_bucket = 'UNKNOWN') AS unknown_rows
		FROM participants FINAL
		WHERE platform = ?
			AND puuid != ''
			AND puuid NOT IN
			(
				SELECT puuid
				FROM summoner_rank_snapshots FINAL
				WHERE platform = ?
					AND queue_type = ?
					AND expires_at > ?
			)
		GROUP BY puuid, platform
		ORDER BY unknown_rows DESC, participant_rows DESC
		LIMIT ?`,
		platform, platform, rankedSoloQueueType, now, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates := []analytics.RankCandidate{}
	for rows.Next() {
		var candidate analytics.RankCandidate
		if err := rows.Scan(&candidate.PUUID, &candidate.Platform, &candidate.ParticipantRows, &candidate.UnknownRows); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func (r *Repository) FreshRankBuckets(ctx context.Context, platform string, puuids []string, now time.Time) (map[string]string, error) {
	out := map[string]string{}
	seen := map[string]bool{}
	for _, puuid := range puuids {
		if puuid == "" || seen[puuid] {
			continue
		}
		seen[puuid] = true
		var bucket string
		err := r.db.QueryRowContext(
			ctx,
			`SELECT rank_bucket
			FROM summoner_rank_snapshots FINAL
			WHERE platform = ? AND puuid = ? AND queue_type = ? AND expires_at > ?
			ORDER BY fetched_at DESC
			LIMIT 1`,
			platform, puuid, rankedSoloQueueType, now,
		).Scan(&bucket)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		out[puuid] = bucket
	}
	return out, nil
}

func (r *Repository) FreshRankSnapshots(ctx context.Context, platform string, puuids []string, now time.Time) (map[string]analytics.RankSnapshot, error) {
	out := map[string]analytics.RankSnapshot{}
	seen := map[string]bool{}
	for _, puuid := range puuids {
		if puuid == "" || seen[puuid] {
			continue
		}
		seen[puuid] = true
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
			WHERE platform = ? AND puuid = ? AND queue_type = ? AND expires_at > ?
			ORDER BY fetched_at DESC
			LIMIT 1`,
			platform, puuid, rankedSoloQueueType, now,
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
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		snapshot.LeaguePoints = int(leaguePoints)
		snapshot.Wins = int(wins)
		snapshot.Losses = int(losses)
		out[puuid] = snapshot
	}
	return out, nil
}

func (r *Repository) InsertRankSnapshot(ctx context.Context, snapshot analytics.RankSnapshot) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO summoner_rank_snapshots
		(puuid, platform, queue_type, tier, division, league_points, wins, losses, rank_bucket, fetched_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshot.PUUID,
		snapshot.Platform,
		snapshot.QueueType,
		snapshot.Tier,
		snapshot.Division,
		snapshot.LeaguePoints,
		snapshot.Wins,
		snapshot.Losses,
		snapshot.RankBucket,
		snapshot.FetchedAt,
		snapshot.ExpiresAt,
	)
	return err
}

func (r *Repository) SummonerRankLeaderboard(ctx context.Context, platform string, limit int) ([]SummonerLeaderboardRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(
		ctx,
		`WITH latest_rank AS
		(
			SELECT
				puuid,
				platform,
				argMax(tier, fetched_at) AS tier,
				argMax(division, fetched_at) AS division,
				argMax(league_points, fetched_at) AS league_points,
				argMax(wins, fetched_at) AS wins,
				argMax(losses, fetched_at) AS losses,
				argMax(rank_bucket, fetched_at) AS rank_bucket,
				max(fetched_at) AS latest_fetched_at,
				argMax(expires_at, fetched_at) AS expires_at
			FROM summoner_rank_snapshots FINAL
			WHERE platform = ? AND queue_type = ?
			GROUP BY platform, puuid
		),
		latest_alias AS
		(
			SELECT
				puuid,
				platform,
				argMax(game_name, last_seen_at) AS game_name,
				argMax(tag_line, last_seen_at) AS tag_line,
				max(last_seen_at) AS latest_seen_at
			FROM riot_account_aliases FINAL
			WHERE platform = ?
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
			WHERE platform = ?
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
				WHERE platform = ? AND queue_id = ?
			)
			WHERE puuid != '' AND raw_profile_icon_id > 0
			GROUP BY platform, puuid
		),
		latest_profile AS
		(
			SELECT
				puuid,
				platform,
				argMax(games, compiled_at) AS stored_games,
				argMax(wins, compiled_at) AS stored_wins
			FROM summoner_profile_summary FINAL
			WHERE platform = ? AND queue_id = ?
			GROUP BY platform, puuid
		)
		SELECT
			r.puuid,
			r.platform,
			a.game_name,
			a.tag_line,
			r.tier,
			r.division,
			r.league_points,
			r.wins,
			r.losses,
			r.rank_bucket,
			r.latest_fetched_at,
			r.expires_at,
			a.latest_seen_at,
			if(ifNull(s.profile_icon_id, 0) > 0, ifNull(s.profile_icon_id, 0), ifNull(rp.profile_icon_id, 0)) AS profile_icon_id,
			ifNull(s.summoner_level, 0) AS summoner_level,
			ifNull(p.stored_games, 0) AS stored_games,
			ifNull(p.stored_wins, 0) AS stored_wins
		FROM latest_rank AS r
		INNER JOIN latest_alias AS a
			ON a.platform = r.platform AND a.puuid = r.puuid
		LEFT JOIN latest_account AS s
			ON s.platform = r.platform AND s.puuid = r.puuid
		LEFT JOIN latest_raw_profile AS rp
			ON rp.platform = r.platform AND rp.puuid = r.puuid
		LEFT JOIN latest_profile AS p
			ON p.platform = r.platform AND p.puuid = r.puuid
		WHERE a.game_name != '' AND a.tag_line != '' AND r.tier != '' AND r.tier != 'UNRANKED'
		ORDER BY
			multiIf(
				r.tier = 'CHALLENGER', 0,
				r.tier = 'GRANDMASTER', 1,
				r.tier = 'MASTER', 2,
				r.tier = 'DIAMOND', 3,
				r.tier = 'EMERALD', 4,
				r.tier = 'PLATINUM', 5,
				r.tier = 'GOLD', 6,
				r.tier = 'SILVER', 7,
				r.tier = 'BRONZE', 8,
				r.tier = 'IRON', 9,
				10
			) ASC,
			multiIf(
				r.division = 'I', 1,
				r.division = 'II', 2,
				r.division = 'III', 3,
				r.division = 'IV', 4,
				0
			) ASC,
			r.league_points DESC,
			r.wins DESC,
			r.losses ASC,
			a.game_name ASC
		LIMIT ?`,
		platform,
		rankedSoloQueueType,
		platform,
		platform,
		platform,
		analytics.RankedSoloQueueID,
		platform,
		analytics.RankedSoloQueueID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SummonerLeaderboardRow{}
	for rows.Next() {
		var row SummonerLeaderboardRow
		var leaguePoints int16
		var wins, losses uint32
		var storedGames, storedWins uint64
		if err := rows.Scan(
			&row.PUUID,
			&row.Platform,
			&row.GameName,
			&row.TagLine,
			&row.Tier,
			&row.Division,
			&leaguePoints,
			&wins,
			&losses,
			&row.RankBucket,
			&row.FetchedAt,
			&row.ExpiresAt,
			&row.LastSeenAt,
			&row.ProfileIconID,
			&row.SummonerLevel,
			&storedGames,
			&storedWins,
		); err != nil {
			return nil, err
		}
		row.LeaguePoints = int(leaguePoints)
		row.Wins = int(wins)
		row.Losses = int(losses)
		row.StoredGames = int(storedGames)
		row.StoredWins = int(storedWins)
		out = append(out, row)
	}
	return out, rows.Err()
}
