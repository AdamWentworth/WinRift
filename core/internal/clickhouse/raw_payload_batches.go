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
		SELECT match_id
		FROM %s FINAL
		WHERE patch = ?
			AND platform = ?
			AND queue_id = ?
			AND match_id NOT IN
			(
				SELECT match_id
				FROM %s FINAL
				WHERE patch = ?
					AND platform = ?
					AND queue_id = ?
			)
		ORDER BY match_id`, sourceTable, destinationTable)
	rows, err := r.db.QueryContext(ctx, query, patch, platform, queueID, patch, platform, queueID)
	if err != nil {
		return err
	}
	missingMatchIDs := []string{}
	for rows.Next() {
		var matchID string
		if err := rows.Scan(&matchID); err != nil {
			_ = rows.Close()
			return err
		}
		matchID = strings.TrimSpace(matchID)
		if matchID != "" {
			missingMatchIDs = append(missingMatchIDs, matchID)
		}
	}
	rowsErr := rows.Err()
	_ = rows.Close()
	if rowsErr != nil {
		return rowsErr
	}
	if len(missingMatchIDs) == 0 {
		log.Printf(
			"raw payload backfill current source=%s destination=%s patch=%s platform=%s queue=%d",
			sourceTable,
			destinationTable,
			patch,
			platform,
			queueID,
		)
		return nil
	}

	totalBatches := (len(missingMatchIDs) + rawPayloadBackfillBatchSize - 1) / rawPayloadBackfillBatchSize
	for start := 0; start < len(missingMatchIDs); start += rawPayloadBackfillBatchSize {
		end := start + rawPayloadBackfillBatchSize
		if end > len(missingMatchIDs) {
			end = len(missingMatchIDs)
		}
		if err := process(missingMatchIDs[start:end]); err != nil {
			return err
		}
		batchNumber := start/rawPayloadBackfillBatchSize + 1
		if batchNumber == 1 || batchNumber%10 == 0 || batchNumber == totalBatches {
			log.Printf(
				"raw payload backfill progress source=%s destination=%s patch=%s platform=%s queue=%d batch=%d/%d matches=%d/%d",
				sourceTable,
				destinationTable,
				patch,
				platform,
				queueID,
				batchNumber,
				totalBatches,
				end,
				len(missingMatchIDs),
			)
		}
	}
	return nil
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
