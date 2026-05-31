package clickhouse

import (
	"context"

	"winrift/services/core/internal/analytics"
)

type PatchStat struct {
	Patch              string
	Matches            uint64
	ParticipantSamples uint64
	RawMatches         uint64
	CompiledMatches    uint64
}

func (r *Repository) PatchStats(ctx context.Context, queueID uint16) ([]PatchStat, error) {
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			patch,
			greatest(max(raw_matches), max(participant_matches), max(snapshot_matches)) AS matches,
			greatest(max(participant_samples), max(snapshot_participants)) AS participant_samples,
			max(raw_matches) AS raw_matches,
			max(snapshot_matches) AS compiled_matches
		FROM
		(
			SELECT
				patch,
				uniqExact(match_id) AS raw_matches,
				toUInt64(0) AS participant_matches,
				toUInt64(0) AS snapshot_matches,
				toUInt64(0) AS participant_samples,
				toUInt64(0) AS snapshot_participants
			FROM raw_matches FINAL
			WHERE queue_id = ?
			GROUP BY patch

			UNION ALL

			SELECT
				patch,
				toUInt64(0) AS raw_matches,
				uniqExact(match_id) AS participant_matches,
				toUInt64(0) AS snapshot_matches,
				count() AS participant_samples,
				toUInt64(0) AS snapshot_participants
			FROM participants FINAL
			WHERE queue_id = ?
			GROUP BY patch

			UNION ALL

			SELECT
				patch,
				toUInt64(0) AS raw_matches,
				toUInt64(0) AS participant_matches,
				sum(matches) AS snapshot_matches,
				toUInt64(0) AS participant_samples,
				sum(participants) AS snapshot_participants
			FROM patch_snapshots FINAL
			WHERE queue_id = ?
			GROUP BY patch
		)
		WHERE patch != ''
		GROUP BY patch
		ORDER BY
			toUInt16OrZero(arrayElement(splitByChar('.', patch), 1)) DESC,
			toUInt16OrZero(arrayElement(splitByChar('.', patch), 2)) DESC`,
		queueID,
		queueID,
		queueID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := []PatchStat{}
	for rows.Next() {
		var stat PatchStat
		if err := rows.Scan(&stat.Patch, &stat.Matches, &stat.ParticipantSamples, &stat.RawMatches, &stat.CompiledMatches); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}
	return stats, rows.Err()
}
