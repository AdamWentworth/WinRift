package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"winrift/services/core/internal/analytics"
)

const rankedSoloQueueType = "RANKED_SOLO_5x5"

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
