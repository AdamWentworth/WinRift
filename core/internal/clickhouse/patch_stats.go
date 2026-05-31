package clickhouse

import (
	"context"

	"winrift/core/internal/analytics"
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
			greatest(raw_matches, participant_matches, snapshot_matches) AS matches,
			greatest(participant_samples, snapshot_participants) AS participant_samples,
			raw_matches,
			snapshot_matches AS compiled_matches
		FROM
		(
			SELECT
				patch,
				max(raw_match_count) AS raw_matches,
				max(participant_match_count) AS participant_matches,
				max(snapshot_match_count) AS snapshot_matches,
				max(participant_sample_count) AS participant_samples,
				max(snapshot_participant_count) AS snapshot_participants
			FROM
			(
				SELECT
					patch,
					uniqExact(match_id) AS raw_match_count,
					toUInt64(0) AS participant_match_count,
					toUInt64(0) AS snapshot_match_count,
					toUInt64(0) AS participant_sample_count,
					toUInt64(0) AS snapshot_participant_count
				FROM raw_matches FINAL
				WHERE queue_id = ?
				GROUP BY patch

				UNION ALL

				SELECT
					patch,
					toUInt64(0) AS raw_match_count,
					uniqExact(match_id) AS participant_match_count,
					toUInt64(0) AS snapshot_match_count,
					count() AS participant_sample_count,
					toUInt64(0) AS snapshot_participant_count
				FROM participants FINAL
				WHERE queue_id = ?
				GROUP BY patch

				UNION ALL

				SELECT
					patch,
					toUInt64(0) AS raw_match_count,
					toUInt64(0) AS participant_match_count,
					sum(matches) AS snapshot_match_count,
					toUInt64(0) AS participant_sample_count,
					sum(participants) AS snapshot_participant_count
				FROM patch_snapshots FINAL
				WHERE queue_id = ?
				GROUP BY patch
			)
			WHERE patch != ''
			GROUP BY patch
		)
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
