package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"winrift/core/internal/analytics"
)

type PatchSnapshot struct {
	Patch            string
	Platform         string
	QueueID          uint16
	Status           string
	Matches          uint64
	Participants     uint64
	RawRetainedUntil time.Time
}

func (r *Repository) MarkPatchCollecting(ctx context.Context, patch, platform string, queueID uint16) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO patch_snapshots (patch, platform, queue_id, status, started_at, matches, participants, notes) VALUES (?, ?, ?, 'collecting', now(), 0, 0, '')`,
		patch, platform, queueID,
	)
	return err
}

func (r *Repository) CompilePatchMetrics(ctx context.Context, patch, platform string, queueID uint16, rawRetainedUntil time.Time) error {
	if err := r.deleteCompiledPatchMetrics(ctx, patch, platform, queueID); err != nil {
		return err
	}
	if err := r.markPatchCompiling(ctx, patch, platform, queueID); err != nil {
		return err
	}
	if err := r.compilePatchBuildMetrics(ctx, patch, platform, queueID); err != nil {
		return err
	}
	if err := r.compilePatchItemTimingMetrics(ctx, patch, platform, queueID); err != nil {
		return err
	}
	if err := r.compilePatchPowerCurveMetrics(ctx, patch, platform, queueID); err != nil {
		return err
	}
	if err := r.RefreshWinConditionMetrics(ctx, patch, platform, queueID); err != nil {
		return err
	}
	if err := r.deleteLiveBuildAggregate(ctx, patch, platform, queueID); err != nil {
		return err
	}
	return r.markPatchClosed(ctx, patch, platform, queueID, rawRetainedUntil)
}

func (r *Repository) DeleteRawPatchData(ctx context.Context, patch, platform string, queueID uint16) error {
	if err := r.requirePatchClosed(ctx, patch, platform, queueID); err != nil {
		return err
	}
	statements := []string{
		`ALTER TABLE raw_timelines DELETE WHERE patch = ? AND platform = ? AND queue_id = ?`,
		`ALTER TABLE raw_matches DELETE WHERE patch = ? AND platform = ? AND queue_id = ?`,
		`ALTER TABLE timeline_participant_frames DELETE WHERE patch = ? AND platform = ? AND queue_id = ?`,
		`ALTER TABLE timeline_item_events DELETE WHERE patch = ? AND platform = ? AND queue_id = ?`,
		`ALTER TABLE timeline_skill_events DELETE WHERE patch = ? AND platform = ? AND queue_id = ?`,
		`ALTER TABLE timeline_combat_events DELETE WHERE patch = ? AND platform = ? AND queue_id = ?`,
		`ALTER TABLE timeline_objective_events DELETE WHERE patch = ? AND platform = ? AND queue_id = ?`,
		`ALTER TABLE champion_bans DELETE WHERE patch = ? AND platform = ? AND queue_id = ?`,
	}
	for _, statement := range statements {
		if _, err := r.db.ExecContext(ctx, statement+` SETTINGS mutations_sync = 2`, patch, platform, queueID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) DeletePatchesOutsideWindow(ctx context.Context, currentPatch string, retentionCount int) ([]string, error) {
	if strings.TrimSpace(currentPatch) == "" {
		return nil, nil
	}
	patches, err := r.StoredPatches(ctx)
	if err != nil {
		return nil, err
	}
	var prune []string
	for _, patch := range patches {
		if !analytics.PatchInWindow(patch, currentPatch, retentionCount) {
			prune = append(prune, patch)
		}
	}
	if len(prune) == 0 {
		return nil, nil
	}
	for _, patch := range prune {
		platforms, err := r.PatchPlatforms(ctx, patch, analytics.RankedSoloQueueID)
		if err != nil {
			return nil, err
		}
		for _, platform := range platforms {
			if err := r.DeleteRawPatchData(ctx, patch, platform, analytics.RankedSoloQueueID); err != nil {
				return nil, err
			}
		}
	}
	return prune, nil
}

func (r *Repository) PatchPlatforms(ctx context.Context, patch string, queueID uint16) ([]string, error) {
	patch = strings.TrimSpace(patch)
	if patch == "" {
		return nil, nil
	}
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT platform
		FROM
		(
			SELECT platform FROM raw_matches WHERE patch = ? AND queue_id = ?
			UNION ALL SELECT platform FROM participants WHERE patch = ? AND queue_id = ?
			UNION ALL SELECT platform FROM participant_matchups WHERE patch = ? AND queue_id = ?
			UNION ALL SELECT platform FROM participant_performance WHERE patch = ? AND queue_id = ?
			UNION ALL SELECT platform FROM patch_build_metrics WHERE patch = ? AND queue_id = ?
			UNION ALL SELECT platform FROM patch_snapshots WHERE patch = ? AND queue_id = ?
		)
		WHERE platform != ''
		ORDER BY platform`,
		patch, queueID,
		patch, queueID,
		patch, queueID,
		patch, queueID,
		patch, queueID,
		patch, queueID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	platforms := []string{}
	for rows.Next() {
		var platform string
		if err := rows.Scan(&platform); err != nil {
			return nil, err
		}
		platforms = append(platforms, platform)
	}
	return platforms, rows.Err()
}

func (r *Repository) StoredPatches(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT patch
		FROM
		(
			SELECT patch FROM raw_matches
			UNION ALL SELECT patch FROM raw_timelines
			UNION ALL SELECT patch FROM participants
			UNION ALL SELECT patch FROM participant_matchups
			UNION ALL SELECT patch FROM participant_performance
			UNION ALL SELECT patch_bucket AS patch FROM build_analytics_mv
			UNION ALL SELECT patch FROM timeline_participant_frames
			UNION ALL SELECT patch FROM timeline_item_events
			UNION ALL SELECT patch FROM timeline_skill_events
			UNION ALL SELECT patch FROM timeline_combat_events
			UNION ALL SELECT patch FROM timeline_objective_events
			UNION ALL SELECT patch FROM champion_bans
			UNION ALL SELECT patch FROM patch_build_metrics
			UNION ALL SELECT patch FROM patch_item_timing_metrics
			UNION ALL SELECT patch FROM item_slot_analytics
			UNION ALL SELECT patch FROM starting_loadout_analytics
			UNION ALL SELECT patch FROM champion_skill_analytics
			UNION ALL SELECT patch FROM champion_ban_analytics
			UNION ALL SELECT patch FROM team_kill_summary
			UNION ALL SELECT patch FROM champion_guide_summary_analytics
			UNION ALL SELECT patch FROM champion_guide_scope_analytics
			UNION ALL SELECT patch FROM champion_matchup_analytics
			UNION ALL SELECT patch FROM champion_signature_analytics
			UNION ALL SELECT patch FROM build_signature_analytics
			UNION ALL SELECT patch FROM champion_build_variant_analytics
			UNION ALL SELECT patch FROM patch_power_curve_metrics
			UNION ALL SELECT patch FROM match_team_win_conditions
			UNION ALL SELECT patch FROM patch_win_condition_metrics
			UNION ALL SELECT patch FROM patch_snapshots
		)
		WHERE patch != ''
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	patches := []string{}
	for rows.Next() {
		var patch string
		if err := rows.Scan(&patch); err != nil {
			return nil, err
		}
		patches = append(patches, patch)
	}
	sort.Slice(patches, func(i, j int) bool {
		return patchLess(patches[i], patches[j])
	})
	return patches, rows.Err()
}

func (r *Repository) DeletePatches(ctx context.Context, patches []string) error {
	statements := []struct {
		table  string
		column string
	}{
		{table: "raw_timelines", column: "patch"},
		{table: "raw_matches", column: "patch"},
		{table: "participants", column: "patch"},
		{table: "participant_matchups", column: "patch"},
		{table: "participant_performance", column: "patch"},
		{table: "build_analytics_mv", column: "patch_bucket"},
		{table: "timeline_participant_frames", column: "patch"},
		{table: "timeline_item_events", column: "patch"},
		{table: "timeline_skill_events", column: "patch"},
		{table: "timeline_combat_events", column: "patch"},
		{table: "timeline_objective_events", column: "patch"},
		{table: "champion_bans", column: "patch"},
		{table: "patch_build_metrics", column: "patch"},
		{table: "patch_item_timing_metrics", column: "patch"},
		{table: "item_slot_analytics", column: "patch"},
		{table: "starting_loadout_analytics", column: "patch"},
		{table: "champion_skill_analytics", column: "patch"},
		{table: "champion_ban_analytics", column: "patch"},
		{table: "team_kill_summary", column: "patch"},
		{table: "champion_guide_summary_analytics", column: "patch"},
		{table: "champion_guide_scope_analytics", column: "patch"},
		{table: "champion_matchup_analytics", column: "patch"},
		{table: "champion_signature_analytics", column: "patch"},
		{table: "build_signature_analytics", column: "patch"},
		{table: "champion_build_variant_analytics", column: "patch"},
		{table: "patch_power_curve_metrics", column: "patch"},
		{table: "match_team_win_conditions", column: "patch"},
		{table: "patch_win_condition_metrics", column: "patch"},
		{table: "patch_snapshots", column: "patch"},
	}
	for _, patch := range patches {
		patch = strings.TrimSpace(patch)
		if patch == "" {
			continue
		}
		for _, statement := range statements {
			query := fmt.Sprintf("ALTER TABLE %s DELETE WHERE %s = ? SETTINGS mutations_sync = 2", statement.table, statement.column)
			if _, err := r.db.ExecContext(ctx, query, patch); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Repository) requirePatchClosed(ctx context.Context, patch, platform string, queueID uint16) error {
	var status string
	err := r.db.QueryRowContext(
		ctx,
		`SELECT status FROM patch_snapshots FINAL WHERE patch = ? AND platform = ? AND queue_id = ? LIMIT 1`,
		patch, platform, queueID,
	).Scan(&status)
	if err == sql.ErrNoRows {
		return fmt.Errorf("patch %s/%s/%d has no lifecycle snapshot", patch, platform, queueID)
	}
	if err != nil {
		return err
	}
	if status != "closed" {
		return fmt.Errorf("patch %s/%s/%d is %q, not closed", patch, platform, queueID, status)
	}
	return nil
}

func (r *Repository) markPatchCompiling(ctx context.Context, patch, platform string, queueID uint16) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO patch_snapshots
		(patch, platform, queue_id, status, started_at, closed_at, raw_retained_until, matches, participants, compiled_at, notes)
		SELECT
			?, ?, ?, 'compiling', min(toDateTime(intDiv(game_creation, 1000))), NULL, NULL, count(), count() * 10, NULL, ''
		FROM raw_matches FINAL
		WHERE patch = ? AND platform = ? AND queue_id = ?`,
		patch, platform, queueID, patch, platform, queueID,
	)
	return err
}

func (r *Repository) markPatchClosed(ctx context.Context, patch, platform string, queueID uint16, rawRetainedUntil time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO patch_snapshots
		(patch, platform, queue_id, status, started_at, closed_at, raw_retained_until, matches, participants, compiled_at, notes)
		SELECT
			?, ?, ?, 'closed', min(toDateTime(intDiv(game_creation, 1000))), now(), ?, count(), count() * 10, now(), ''
		FROM raw_matches FINAL
		WHERE patch = ? AND platform = ? AND queue_id = ?`,
		patch, platform, queueID, rawRetainedUntil, patch, platform, queueID,
	)
	return err
}

func (r *Repository) deleteCompiledPatchMetrics(ctx context.Context, patch, platform string, queueID uint16) error {
	statements := []string{
		`ALTER TABLE patch_build_metrics DELETE WHERE patch = ? AND platform = ? AND queue_id = ? SETTINGS mutations_sync = 2`,
		`ALTER TABLE patch_item_timing_metrics DELETE WHERE patch = ? AND platform = ? AND queue_id = ? SETTINGS mutations_sync = 2`,
		`ALTER TABLE patch_power_curve_metrics DELETE WHERE patch = ? AND platform = ? AND queue_id = ? SETTINGS mutations_sync = 2`,
	}
	for _, statement := range statements {
		if _, err := r.db.ExecContext(ctx, statement, patch, platform, queueID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) deleteLiveBuildAggregate(ctx context.Context, patch, platform string, queueID uint16) error {
	_, err := r.db.ExecContext(
		ctx,
		`ALTER TABLE build_analytics_mv DELETE WHERE patch_bucket = ? AND platform = ? AND queue_id = ?`,
		patch, platform, queueID,
	)
	return err
}

func (r *Repository) compilePatchBuildMetrics(ctx context.Context, patch, platform string, queueID uint16) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO patch_build_metrics
		(patch, platform, queue_id, champion_id, role, opponent_champion_id, rank_bucket, final_items_signature, core2_signature, core3_signature, rune_signature, spell_signature, wins, games)
		SELECT
			patch,
			platform,
			queue_id,
			champion_id,
			role,
			opponent_champion_id,
			rank_bucket,
			final_items_signature,
			core2_signature,
			core3_signature,
			rune_signature,
			spell_signature,
			toUInt64(sum(win)) AS wins,
			toUInt64(count()) AS games
		FROM participant_matchups FINAL
		WHERE patch = ? AND platform = ? AND queue_id = ?
		GROUP BY
			patch,
			platform,
			queue_id,
			champion_id,
			role,
			opponent_champion_id,
			rank_bucket,
			final_items_signature,
			core2_signature,
			core3_signature,
			rune_signature,
			spell_signature`,
		patch, platform, queueID,
	)
	return err
}

func (r *Repository) compilePatchItemTimingMetrics(ctx context.Context, patch, platform string, queueID uint16) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO patch_item_timing_metrics
		(patch, platform, queue_id, champion_id, role, opponent_champion_id, rank_bucket, item_slot, item_signature, games, avg_timing_ms, p50_timing_ms, p75_timing_ms, p90_timing_ms)
		WITH item_purchases AS
		(
			SELECT
				pm.patch AS patch,
				pm.platform AS platform,
				pm.queue_id AS queue_id,
				pm.champion_id AS champion_id,
				pm.role AS role,
				pm.opponent_champion_id AS opponent_champion_id,
				pm.rank_bucket AS rank_bucket,
				pm.match_id AS match_id,
				pm.participant_id AS participant_id,
				tie.item_id AS item_id,
				min(tie.timestamp_ms) AS first_purchase_ms
			FROM participant_matchups AS pm FINAL
			INNER JOIN timeline_item_events AS tie FINAL
				ON pm.match_id = tie.match_id
				AND pm.participant_id = tie.participant_id
			WHERE pm.patch = ? AND pm.platform = ? AND pm.queue_id = ?
				AND tie.event_type = 'ITEM_PURCHASED'
				AND tie.item_id NOT IN (3340, 3363, 3364, 3330, 3348, 2052)
			GROUP BY
				pm.patch,
				pm.platform,
				pm.queue_id,
				pm.champion_id,
				pm.role,
				pm.opponent_champion_id,
				pm.rank_bucket,
				pm.match_id,
				pm.participant_id,
				tie.item_id
		),
		item_slots AS
		(
			SELECT
				*,
				row_number() OVER (PARTITION BY match_id, participant_id ORDER BY first_purchase_ms, item_id) AS item_slot
			FROM item_purchases
		)
		SELECT
			patch,
			platform,
			queue_id,
			champion_id,
			role,
			opponent_champion_id,
			rank_bucket,
			toUInt8(item_slot) AS item_slot,
			toString(item_id) AS item_signature,
			toUInt64(count()) AS games,
			avg(first_purchase_ms) AS avg_timing_ms,
			quantile(0.50)(first_purchase_ms) AS p50_timing_ms,
			quantile(0.75)(first_purchase_ms) AS p75_timing_ms,
			quantile(0.90)(first_purchase_ms) AS p90_timing_ms
		FROM item_slots
		WHERE item_slot <= 6
		GROUP BY
			patch,
			platform,
			queue_id,
			champion_id,
			role,
			opponent_champion_id,
			rank_bucket,
			item_slot,
			item_id`,
		patch, platform, queueID,
	)
	return err
}

func (r *Repository) compilePatchPowerCurveMetrics(ctx context.Context, patch, platform string, queueID uint16) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO patch_power_curve_metrics
		(patch, platform, queue_id, champion_id, role, opponent_champion_id, rank_bucket, minute_mark, games, avg_level, avg_total_gold, avg_cs, avg_jungle_cs, avg_damage_done_to_champions, avg_damage_taken)
		SELECT
			pm.patch AS patch,
			pm.platform AS platform,
			pm.queue_id AS queue_id,
			pm.champion_id AS champion_id,
			pm.role AS role,
			pm.opponent_champion_id AS opponent_champion_id,
			pm.rank_bucket AS rank_bucket,
			toUInt8(round(tpf.timestamp_ms / 60000)) AS minute_mark,
			toUInt64(count()) AS games,
			avg(tpf.level) AS avg_level,
			avg(tpf.total_gold) AS avg_total_gold,
			avg(tpf.minions_killed) AS avg_cs,
			avg(tpf.jungle_minions_killed) AS avg_jungle_cs,
			avg(tpf.total_damage_done_to_champions) AS avg_damage_done_to_champions,
			avg(tpf.total_damage_taken) AS avg_damage_taken
		FROM participant_matchups AS pm FINAL
		INNER JOIN timeline_participant_frames AS tpf FINAL
			ON pm.match_id = tpf.match_id
			AND pm.participant_id = tpf.participant_id
		WHERE pm.patch = ? AND pm.platform = ? AND pm.queue_id = ?
			AND tpf.timestamp_ms BETWEEN 590000 AND 1210000
			AND toUInt8(round(tpf.timestamp_ms / 60000)) IN (10, 15, 20)
		GROUP BY
			pm.patch,
			pm.platform,
			pm.queue_id,
			pm.champion_id,
			pm.role,
			pm.opponent_champion_id,
			pm.rank_bucket,
			minute_mark`,
		patch, platform, queueID,
	)
	return err
}

func patchLess(left, right string) bool {
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	if len(leftParts) >= 2 && len(rightParts) >= 2 {
		leftMajor, leftMajorErr := strconv.Atoi(leftParts[0])
		leftMinor, leftMinorErr := strconv.Atoi(leftParts[1])
		rightMajor, rightMajorErr := strconv.Atoi(rightParts[0])
		rightMinor, rightMinorErr := strconv.Atoi(rightParts[1])
		if leftMajorErr == nil && leftMinorErr == nil && rightMajorErr == nil && rightMinorErr == nil {
			if leftMajor != rightMajor {
				return leftMajor < rightMajor
			}
			return leftMinor < rightMinor
		}
	}
	return left < right
}
