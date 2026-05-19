package clickhouse

import (
	"context"
	"fmt"
	"strings"
)

type TeamCompositionFilters struct {
	QueueID uint16
	Patch   string
	MaxRows int
}

type TeamCompositionRow struct {
	MatchID         string
	Platform        string
	Patch           string
	QueueID         uint16
	TeamID          uint16
	Win             bool
	DurationSeconds uint32
	ChampionIDs     []uint16
	RankBuckets     []string
}

func (r *Repository) QueryTeamCompositions(ctx context.Context, filters TeamCompositionFilters) ([]TeamCompositionRow, error) {
	queueID := filters.QueueID
	if queueID == 0 {
		queueID = 420
	}
	query := `
		WITH participant_ranked AS
		(
			SELECT
				p.match_id,
				p.platform,
				p.patch,
				p.queue_id,
				p.team_id,
				p.champion_id,
				p.win,
				multiIf(
					s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
					p.rank_bucket
				) AS rank_value
			FROM participants AS p FINAL
			LEFT JOIN
			(
				SELECT
					platform,
					puuid,
					argMax(rank_bucket, fetched_at) AS snapshot_rank_bucket
				FROM summoner_rank_snapshots FINAL
				WHERE queue_type = 'RANKED_SOLO_5x5'
				GROUP BY platform, puuid
			) AS s
				ON s.platform = p.platform AND s.puuid = p.puuid
			WHERE p.queue_id = ?`
	args := []any{queueID}
	if strings.TrimSpace(filters.Patch) != "" {
		query += " AND p.patch = ?"
		args = append(args, strings.TrimSpace(filters.Patch))
	}
	query += `
		)
		SELECT
			pr.match_id,
			any(pr.platform) AS platform,
			any(pr.patch) AS patch,
			any(pr.queue_id) AS queue_id,
			pr.team_id,
			toUInt8(max(pr.win)) AS win,
			any(rm.duration_seconds) AS duration_seconds,
			groupArray(toUInt16(pr.champion_id)) AS champion_ids,
			groupArray(rank_value) AS rank_buckets
		FROM participant_ranked AS pr
		INNER JOIN raw_matches AS rm FINAL ON rm.match_id = pr.match_id
		GROUP BY pr.match_id, pr.team_id
		HAVING length(champion_ids) = 5
		ORDER BY max(rm.game_end_timestamp) DESC`
	if filters.MaxRows > 0 {
		query += " LIMIT ?"
		args = append(args, filters.MaxRows)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query team compositions: %w", err)
	}
	defer rows.Close()
	out := []TeamCompositionRow{}
	for rows.Next() {
		var row TeamCompositionRow
		var win uint8
		if err := rows.Scan(&row.MatchID, &row.Platform, &row.Patch, &row.QueueID, &row.TeamID, &win, &row.DurationSeconds, &row.ChampionIDs, &row.RankBuckets); err != nil {
			return nil, fmt.Errorf("scan team composition: %w", err)
		}
		row.Win = win > 0
		out = append(out, row)
	}
	return out, rows.Err()
}
