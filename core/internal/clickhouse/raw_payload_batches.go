package clickhouse

import (
	"context"
	"fmt"
	"log"
	"strings"
)

const rawPayloadBackfillBatchSize = 250

var rawPayloadTables = map[string]struct{}{
	"raw_matches":   {},
	"raw_timelines": {},
}

var rawPayloadBackfillTables = map[string]struct{}{
	"participant_performance": {},
	"timeline_skill_events":   {},
	"champion_bans":           {},
}

func (r *Repository) forEachMissingRawPayloadMatchBatch(
	ctx context.Context,
	sourceTable string,
	destinationTable string,
	patch string,
	platform string,
	queueID uint16,
	process func([]string) error,
) error {
	if _, ok := rawPayloadTables[sourceTable]; !ok {
		return fmt.Errorf("unsupported raw payload table %q", sourceTable)
	}
	if _, ok := rawPayloadBackfillTables[destinationTable]; !ok {
		return fmt.Errorf("unsupported backfill destination table %q", destinationTable)
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT match_id
		FROM %s
		PREWHERE patch = ? AND queue_id = ?
		WHERE platform = ?
		ORDER BY match_id`, sourceTable)
	rows, err := r.db.QueryContext(ctx, query, patch, queueID, platform)
	if err != nil {
		return err
	}
	sourceMatchIDs := []string{}
	for rows.Next() {
		var matchID string
		if err := rows.Scan(&matchID); err != nil {
			_ = rows.Close()
			return err
		}
		matchID = strings.TrimSpace(matchID)
		if matchID != "" {
			sourceMatchIDs = append(sourceMatchIDs, matchID)
		}
	}
	rowsErr := rows.Err()
	_ = rows.Close()
	if rowsErr != nil {
		return rowsErr
	}
	if len(sourceMatchIDs) == 0 {
		log.Printf(
			"raw payload backfill empty source=%s destination=%s patch=%s platform=%s queue=%d",
			sourceTable,
			destinationTable,
			patch,
			platform,
			queueID,
		)
		return nil
	}

	totalBatches := (len(sourceMatchIDs) + rawPayloadBackfillBatchSize - 1) / rawPayloadBackfillBatchSize
	missingTotal := 0
	for start := 0; start < len(sourceMatchIDs); start += rawPayloadBackfillBatchSize {
		end := start + rawPayloadBackfillBatchSize
		if end > len(sourceMatchIDs) {
			end = len(sourceMatchIDs)
		}
		missingMatchIDs, err := r.missingRawPayloadDestinationMatches(
			ctx,
			destinationTable,
			patch,
			platform,
			queueID,
			sourceMatchIDs[start:end],
		)
		if err != nil {
			return err
		}
		if len(missingMatchIDs) > 0 {
			if err := process(missingMatchIDs); err != nil {
				return err
			}
			missingTotal += len(missingMatchIDs)
		}
		batchNumber := start/rawPayloadBackfillBatchSize + 1
		if batchNumber == 1 || batchNumber%10 == 0 || batchNumber == totalBatches {
			log.Printf(
				"raw payload backfill progress source=%s destination=%s patch=%s platform=%s queue=%d batch=%d/%d checked=%d/%d missing_processed=%d",
				sourceTable,
				destinationTable,
				patch,
				platform,
				queueID,
				batchNumber,
				totalBatches,
				end,
				len(sourceMatchIDs),
				missingTotal,
			)
		}
	}
	if missingTotal == 0 {
		log.Printf(
			"raw payload backfill current source=%s destination=%s patch=%s platform=%s queue=%d",
			sourceTable,
			destinationTable,
			patch,
			platform,
			queueID,
		)
	}
	return nil
}

func (r *Repository) missingRawPayloadDestinationMatches(
	ctx context.Context,
	destinationTable string,
	patch string,
	platform string,
	queueID uint16,
	matchIDs []string,
) ([]string, error) {
	if len(matchIDs) == 0 {
		return nil, nil
	}
	matchFilter, matchArgs := rawPayloadMatchFilter(matchIDs)
	queryArgs := make([]any, 0, len(matchArgs)+3)
	queryArgs = append(queryArgs, matchArgs...)
	queryArgs = append(queryArgs, patch, platform, queueID)
	query := fmt.Sprintf(`
		SELECT DISTINCT match_id
		FROM %s
		PREWHERE match_id IN (%s)
		WHERE patch = ? AND platform = ? AND queue_id = ?`, destinationTable, matchFilter)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	existing := make(map[string]struct{}, len(matchIDs))
	for rows.Next() {
		var matchID string
		if err := rows.Scan(&matchID); err != nil {
			return nil, err
		}
		existing[strings.TrimSpace(matchID)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	missing := make([]string, 0, len(matchIDs))
	for _, matchID := range matchIDs {
		if _, ok := existing[matchID]; !ok {
			missing = append(missing, matchID)
		}
	}
	return missing, nil
}

func rawPayloadMatchFilter(matchIDs []string) (string, []any) {
	placeholders := make([]string, len(matchIDs))
	args := make([]any, len(matchIDs))
	for i, matchID := range matchIDs {
		placeholders[i] = "?"
		args[i] = matchID
	}
	return strings.Join(placeholders, ", "), args
}
