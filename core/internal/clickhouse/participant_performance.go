package clickhouse

import (
	"context"
	"fmt"
	"strings"

	"winrift/core/internal/analytics"
)

type ParticipantPerformanceBackfillResult struct {
	Rows int
}

func (r *Repository) BackfillParticipantPerformance(ctx context.Context, patch string, queueID uint16) (ParticipantPerformanceBackfillResult, error) {
	patch = strings.TrimSpace(patch)
	if patch == "" {
		return ParticipantPerformanceBackfillResult{}, fmt.Errorf("patch is required")
	}
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	platforms, err := r.rawMatchPlatforms(ctx, patch, queueID)
	if err != nil {
		return ParticipantPerformanceBackfillResult{}, err
	}
	for _, platform := range platforms {
		if err := r.forEachMissingRawPayloadMatchBatch(ctx, "raw_matches", "participant_performance", patch, platform, queueID, func(matchIDs []string) error {
			return r.backfillParticipantPerformanceBatch(ctx, patch, platform, queueID, matchIDs)
		}); err != nil {
			return ParticipantPerformanceBackfillResult{}, err
		}
	}
	var result ParticipantPerformanceBackfillResult
	err = r.db.QueryRowContext(ctx, `SELECT count() FROM participant_performance FINAL WHERE patch = ? AND queue_id = ?`, patch, queueID).Scan(&result.Rows)
	return result, err
}

func (r *Repository) backfillParticipantPerformanceBatch(ctx context.Context, patch, platform string, queueID uint16, matchIDs []string) error {
	matchFilter, matchArgs := rawPayloadMatchFilter(matchIDs)
	queryArgs := []any{patch, platform, queueID}
	queryArgs = append(queryArgs, matchArgs...)
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO participant_performance
		(%s)
		SELECT
			%s
		FROM participants FINAL
		WHERE patch = ?
			AND platform = ?
			AND queue_id = ?
			AND match_id IN (%s)
			AND participant_id > 0`, participantPerformanceColumns, participantPerformanceColumns, matchFilter), queryArgs...)
	return err
}
