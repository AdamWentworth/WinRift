package clickhouse

import (
	"context"
	"fmt"
	"strings"

	"winrift/services/core/internal/analytics"
)

type ChampionGuideEventBackfillResult struct {
	Matches     int
	SkillEvents int
	Bans        int
}

type championBanRate struct {
	Bans    int
	Games   int
	BanRate float64
}

func (r *Repository) BackfillChampionGuideEvents(ctx context.Context, patch string, queueID uint16) (ChampionGuideEventBackfillResult, error) {
	patch = strings.TrimSpace(patch)
	if patch == "" {
		return ChampionGuideEventBackfillResult{}, fmt.Errorf("patch is required")
	}
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	for _, statement := range []string{
		`ALTER TABLE timeline_skill_events DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2`,
		`ALTER TABLE champion_bans DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2`,
	} {
		if _, err := r.db.ExecContext(ctx, statement, patch, queueID); err != nil {
			return ChampionGuideEventBackfillResult{}, err
		}
	}
	platforms, err := r.rawMatchPlatforms(ctx, patch, queueID)
	if err != nil {
		return ChampionGuideEventBackfillResult{}, err
	}
	for _, platform := range platforms {
		if err := r.backfillChampionGuideEventsForPlatform(ctx, patch, platform, queueID); err != nil {
			return ChampionGuideEventBackfillResult{}, err
		}
	}
	var result ChampionGuideEventBackfillResult
	err = r.db.QueryRowContext(ctx, `
		SELECT
			(SELECT count() FROM raw_matches WHERE patch = ? AND queue_id = ?),
			(SELECT count() FROM timeline_skill_events WHERE patch = ? AND queue_id = ?),
			(SELECT count() FROM champion_bans WHERE patch = ? AND queue_id = ?)`,
		patch,
		queueID,
		patch,
		queueID,
		patch,
		queueID,
	).Scan(&result.Matches, &result.SkillEvents, &result.Bans)
	return result, err
}

func (r *Repository) rawMatchPlatforms(ctx context.Context, patch string, queueID uint16) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT platform FROM raw_matches WHERE patch = ? AND queue_id = ? ORDER BY platform`, patch, queueID)
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

func (r *Repository) backfillChampionGuideEventsForPlatform(ctx context.Context, patch, platform string, queueID uint16) error {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO timeline_skill_events
		(match_id, platform, patch, queue_id, timestamp_ms, participant_id, skill_slot, skill_order, level_up_type)
		SELECT
			match_id,
			platform,
			patch,
			queue_id,
			timestamp_ms,
			participant_id,
			skill_slot,
			toUInt8(skill_order) AS skill_order,
			level_up_type
		FROM
		(
			SELECT
				match_id,
				platform,
				patch,
				queue_id,
				timestamp_ms,
				participant_id,
				skill_slot,
				row_number() OVER (PARTITION BY match_id, participant_id ORDER BY timestamp_ms, skill_slot) AS skill_order,
				level_up_type
			FROM
			(
				SELECT
					match_id,
					platform,
					patch,
					queue_id,
					multiIf(
						JSONExtractInt(event, 'timestamp') > 0,
						toUInt32(JSONExtractInt(event, 'timestamp')),
						toUInt32(JSONExtractInt(frame, 'timestamp'))
					) AS timestamp_ms,
					toUInt8(JSONExtractInt(event, 'participantId')) AS participant_id,
					toUInt8(JSONExtractInt(event, 'skillSlot')) AS skill_slot,
					JSONExtractString(event, 'levelUpType') AS level_up_type
				FROM raw_timelines AS rt
				ARRAY JOIN JSONExtractArrayRaw(JSONExtractRaw(raw_json, 'info'), 'frames') AS frame
				ARRAY JOIN JSONExtractArrayRaw(frame, 'events') AS event
				WHERE patch = ?
					AND platform = ?
					AND queue_id = ?
					AND JSONExtractString(event, 'type') = 'SKILL_LEVEL_UP'
			)
			WHERE participant_id > 0 AND skill_slot BETWEEN 1 AND 4
		)
		WHERE skill_order BETWEEN 1 AND 18`,
		patch,
		platform,
		queueID,
	); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO champion_bans
		(match_id, platform, patch, queue_id, team_id, champion_id, pick_turn)
		SELECT
			match_id,
			platform,
			patch,
			queue_id,
			toUInt16(JSONExtractInt(team, 'teamId')) AS team_id,
			toUInt16(JSONExtractInt(ban, 'championId')) AS champion_id,
			toUInt8(JSONExtractInt(ban, 'pickTurn')) AS pick_turn
		FROM raw_matches AS rm
		ARRAY JOIN JSONExtractArrayRaw(JSONExtractRaw(raw_json, 'info'), 'teams') AS team
		ARRAY JOIN JSONExtractArrayRaw(team, 'bans') AS ban
		WHERE patch = ?
			AND platform = ?
			AND queue_id = ?
			AND champion_id > 0`,
		patch,
		platform,
		queueID,
	); err != nil {
		return err
	}
	return nil
}

func (r *Repository) RefreshChampionGuideDerivedAnalytics(ctx context.Context, patch string, queueID uint16) error {
	patch = strings.TrimSpace(patch)
	if patch == "" {
		return fmt.Errorf("patch is required")
	}
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE champion_skill_analytics DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2`, patch, queueID); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE champion_ban_analytics DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2`, patch, queueID); err != nil {
		return err
	}
	if err := r.refreshChampionSkillAnalytics(ctx, patch, queueID); err != nil {
		return err
	}
	return r.refreshChampionBanAnalytics(ctx, patch, queueID)
}

func (r *Repository) refreshChampionSkillAnalytics(ctx context.Context, patch string, queueID uint16) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO champion_skill_analytics
		(patch, platform, queue_id, champion_id, role, rank_bucket, skill_order_signature, wins, games)
		WITH skill_paths AS
		(
			SELECT
				match_id,
				participant_id,
				arrayStringConcat(
					arrayMap(x -> toString(tupleElement(x, 2)),
						arraySort(x -> tupleElement(x, 1), groupArray((skill_order, skill_slot)))
					),
					'-'
				) AS skill_order_signature
			FROM timeline_skill_events
			WHERE patch = ? AND queue_id = ? AND skill_slot BETWEEN 1 AND 4
			GROUP BY match_id, participant_id
			HAVING skill_order_signature != ''
		)
		SELECT
			pm.patch AS patch,
			'ALL' AS platform,
			pm.queue_id AS queue_id,
			pm.champion_id AS champion_id,
			pm.role AS role,
			multiIf(
				s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
				pm.rank_bucket
			) AS rank_bucket,
			sp.skill_order_signature AS skill_order_signature,
			toUInt64(sum(pm.win)) AS wins,
			toUInt64(count()) AS games
		FROM participant_matchups AS pm FINAL
		INNER JOIN skill_paths AS sp
			ON pm.match_id = sp.match_id
			AND pm.participant_id = sp.participant_id
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
			ON s.platform = pm.platform AND s.puuid = pm.puuid
		WHERE pm.patch = ? AND pm.queue_id = ?
		GROUP BY
			pm.patch,
			pm.queue_id,
			pm.champion_id,
			pm.role,
			rank_bucket,
			sp.skill_order_signature`,
		patch,
		queueID,
		patch,
		queueID,
	)
	return err
}

func (r *Repository) refreshChampionBanAnalytics(ctx context.Context, patch string, queueID uint16) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO champion_ban_analytics
		(patch, platform, queue_id, champion_id, bans, games)
		WITH total_matches AS
		(
			SELECT toUInt64(count()) AS games
			FROM raw_matches FINAL
			WHERE patch = ? AND queue_id = ?
		)
		SELECT
			cb.patch AS patch,
			'ALL' AS platform,
			cb.queue_id AS queue_id,
			cb.champion_id AS champion_id,
			toUInt64(count()) AS bans,
			any(total_matches.games) AS games
		FROM champion_bans AS cb
		CROSS JOIN total_matches
		WHERE cb.patch = ? AND cb.queue_id = ?
		GROUP BY cb.patch, cb.queue_id, cb.champion_id`,
		patch,
		queueID,
		patch,
		queueID,
	)
	return err
}

func (r *Repository) queryChampionGuideSkillOrders(ctx context.Context, filters map[string]string, minGames, limit int) ([]ChampionGuideSkillOrderRow, error) {
	if minGames <= 0 {
		minGames = 5
	}
	if limit <= 0 {
		limit = 12
	}
	roleScope := analyticsRoleScope(filters["role"])
	query := fmt.Sprintf(`
		SELECT
			skill_order_signature,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games,
			wins / games AS win_rate
		FROM champion_skill_analytics FINAL
		WHERE platform = 'ALL'
			AND champion_id = ?
			AND %s
			AND skill_order_signature != ''`,
		guideRolePredicate(roleScope),
	)
	args := []any{filters["champion_id"]}
	args = append(args, roleScope.args...)
	if filterValue(filters["patch"]) != "" {
		query += " AND patch = ?"
		args = append(args, filterValue(filters["patch"]))
	}
	if filterValue(filters["rank_bucket"]) != "" {
		query += " AND rank_bucket = ?"
		args = append(args, filterValue(filters["rank_bucket"]))
	}
	query += `
		GROUP BY skill_order_signature
		HAVING games >= ?
		ORDER BY games DESC, win_rate DESC
		LIMIT ?`
	args = append(args, minGames, limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChampionGuideSkillOrderRow{}
	for rows.Next() {
		var row ChampionGuideSkillOrderRow
		if err := rows.Scan(&row.Signature, &row.Wins, &row.Games, &row.WinRate); err != nil {
			return nil, err
		}
		row.Confidence = analytics.WilsonLowerBound(row.Wins, row.Games, 1.96)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) queryChampionBanRates(ctx context.Context, filters map[string]string) (map[uint16]championBanRate, error) {
	query := `
		SELECT
			champion_id,
			toUInt64(sum(bans)) AS bans,
			toUInt64(sum(games)) AS games
		FROM champion_ban_analytics FINAL
		WHERE platform = 'ALL'`
	args := []any{}
	if filterValue(filters["champion_id"]) != "" {
		query += " AND champion_id = ?"
		args = append(args, filterValue(filters["champion_id"]))
	}
	if filterValue(filters["patch"]) != "" {
		query += " AND patch = ?"
		args = append(args, filterValue(filters["patch"]))
	}
	query += `
		GROUP BY champion_id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uint16]championBanRate{}
	for rows.Next() {
		var championID uint16
		var bans int
		var games int
		if err := rows.Scan(&championID, &bans, &games); err != nil {
			return nil, err
		}
		row := championBanRate{Bans: bans, Games: games}
		if games > 0 {
			row.BanRate = float64(bans) / float64(games)
		}
		out[championID] = row
	}
	return out, rows.Err()
}

func guideRolePredicate(scope roleAnalyticsScope) string {
	if scope.whereSQL == "" {
		return "1 = 1"
	}
	return scope.whereSQL
}
