package clickhouse

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"winrift/core/internal/analytics"
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

type championBuildVariantAnalyticsRow struct {
	ChampionID uint16
	Role       string
	RankBucket string
	ChampionGuideBuildVariantRow
}

type championBuildVariantRefreshAggregate struct {
	row                 championBuildVariantAnalyticsRow
	representativeGames int
}

type championBuildVariantSkillChoice struct {
	Signature string
	Wins      int
	Games     int
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
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE team_kill_summary DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2`, patch, queueID); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE champion_role_analytics DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2`, patch, queueID); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE champion_guide_summary_analytics DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2`, patch, queueID); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE champion_guide_scope_analytics DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2`, patch, queueID); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE champion_matchup_analytics DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2`, patch, queueID); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE champion_signature_analytics DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2`, patch, queueID); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE build_signature_analytics DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2`, patch, queueID); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE champion_build_variant_analytics DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2`, patch, queueID); err != nil {
		return err
	}
	if err := r.refreshChampionSkillAnalytics(ctx, patch, queueID); err != nil {
		return err
	}
	if err := r.refreshChampionBanAnalytics(ctx, patch, queueID); err != nil {
		return err
	}
	if err := r.refreshTeamKillSummary(ctx, patch, queueID); err != nil {
		return err
	}
	if err := r.refreshChampionRoleAnalytics(ctx, patch, queueID); err != nil {
		return err
	}
	if err := r.refreshChampionGuideSummaryAnalytics(ctx, patch, queueID); err != nil {
		return err
	}
	if err := r.refreshChampionGuideScopeAnalytics(ctx, patch, queueID); err != nil {
		return err
	}
	if err := r.refreshChampionMatchupAnalytics(ctx, patch, queueID); err != nil {
		return err
	}
	if err := r.refreshChampionSignatureAnalytics(ctx, patch, queueID); err != nil {
		return err
	}
	if err := r.refreshBuildSignatureAnalytics(ctx, patch, queueID); err != nil {
		return err
	}
	return r.refreshChampionBuildVariantAnalytics(ctx, patch, queueID)
}

func (r *Repository) refreshChampionRoleAnalytics(ctx context.Context, patch string, queueID uint16) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO champion_role_analytics
		(patch, platform, queue_id, champion_id, role, games)
		SELECT
			patch,
			platform,
			queue_id,
			champion_id,
			role,
			toUInt64(count()) AS games
		FROM participants FINAL
		WHERE patch = ?
			AND queue_id = ?
			AND champion_id > 0
			AND role IN ('TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY')
		GROUP BY patch, platform, queue_id, champion_id, role`,
		patch,
		queueID,
	)
	return err
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

func (r *Repository) refreshTeamKillSummary(ctx context.Context, patch string, queueID uint16) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO team_kill_summary
		(match_id, platform, patch, queue_id, team_id, kills)
		SELECT
			match_id,
			platform,
			patch,
			queue_id,
			team_id,
			team_kills
		FROM
		(
			SELECT
				match_id,
				platform,
				patch,
				queue_id,
				team_id,
				toUInt64(sum(kills)) AS team_kills
			FROM participants FINAL
			WHERE patch = ? AND queue_id = ?
			GROUP BY
				match_id,
				platform,
				patch,
				queue_id,
				team_id
		)`,
		patch,
		queueID,
	)
	return err
}

func (r *Repository) refreshChampionGuideSummaryAnalytics(ctx context.Context, patch string, queueID uint16) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO champion_guide_summary_analytics
		(
			patch,
			platform,
			queue_id,
			champion_id,
			role,
			rank_bucket,
			wins,
			games,
			kills,
			deaths,
			assists,
			gold_earned_sum,
			cs_sum,
			damage_dealt_to_champions_sum,
			damage_taken_sum,
			damage_self_mitigated_sum,
			damage_dealt_to_objectives_sum,
			damage_dealt_to_structures_sum,
			vision_score_sum,
			time_ccing_others_sum,
			team_utility_sum,
			structure_takedowns_sum,
			objective_takedowns_sum,
			total_time_spent_dead_sum,
			time_played_sum,
			kill_participation_sum
		)
		WITH participant_rows AS
		(
			SELECT
				pm.match_id AS match_id,
				pm.platform AS platform,
				pm.patch AS patch,
				pm.queue_id AS queue_id,
				pm.team_id AS team_id,
				pm.champion_id AS champion_id,
				pm.role AS role,
				multiIf(
					s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
					pm.rank_bucket
				) AS rank_bucket,
				pm.win AS win,
				pm.kills AS participant_kills,
				pm.deaths AS participant_deaths,
				pm.assists AS participant_assists,
				multiIf(pp.gold_earned > 0, pp.gold_earned, pm.gold_earned) AS gold_earned,
				multiIf(pp.total_minions_killed > 0, pp.total_minions_killed, pm.total_minions_killed) AS total_minions_killed,
				multiIf(pp.neutral_minions_killed > 0, pp.neutral_minions_killed, pm.neutral_minions_killed) AS neutral_minions_killed,
				multiIf(pp.total_damage_dealt_to_champions > 0, pp.total_damage_dealt_to_champions, pm.total_damage_dealt_to_champions) AS total_damage_dealt_to_champions,
				multiIf(pp.total_damage_taken > 0, pp.total_damage_taken, pm.total_damage_taken) AS total_damage_taken,
				multiIf(pp.damage_self_mitigated > 0, pp.damage_self_mitigated, pm.damage_self_mitigated) AS damage_self_mitigated,
				multiIf(pp.damage_dealt_to_objectives > 0, pp.damage_dealt_to_objectives, pm.damage_dealt_to_objectives) AS damage_dealt_to_objectives,
				multiIf(pp.damage_dealt_to_turrets > 0, pp.damage_dealt_to_turrets, pm.damage_dealt_to_turrets) AS damage_dealt_to_turrets,
				multiIf(pp.damage_dealt_to_buildings > 0, pp.damage_dealt_to_buildings, pm.damage_dealt_to_buildings) AS damage_dealt_to_buildings,
				multiIf(pp.vision_score > 0, pp.vision_score, pm.vision_score) AS vision_score,
				multiIf(pp.time_ccing_others > 0, pp.time_ccing_others, pm.time_ccing_others) AS time_ccing_others,
				multiIf(pp.total_heal > 0, pp.total_heal, pm.total_heal) AS total_heal,
				multiIf(pp.total_heals_on_teammates > 0, pp.total_heals_on_teammates, pm.total_heals_on_teammates) AS total_heals_on_teammates,
				multiIf(pp.total_damage_shielded_on_teammates > 0, pp.total_damage_shielded_on_teammates, pm.total_damage_shielded_on_teammates) AS total_damage_shielded_on_teammates,
				multiIf(pp.turret_takedowns > 0, pp.turret_takedowns, pm.turret_takedowns) AS turret_takedowns,
				multiIf(pp.inhibitor_takedowns > 0, pp.inhibitor_takedowns, pm.inhibitor_takedowns) AS inhibitor_takedowns,
				multiIf(pp.dragon_kills > 0, pp.dragon_kills, pm.dragon_kills) AS dragon_kills,
				multiIf(pp.baron_kills > 0, pp.baron_kills, pm.baron_kills) AS baron_kills,
				multiIf(pp.objectives_stolen > 0, pp.objectives_stolen, pm.objectives_stolen) AS objectives_stolen,
				multiIf(pp.total_time_spent_dead > 0, pp.total_time_spent_dead, pm.total_time_spent_dead) AS total_time_spent_dead,
				multiIf(pp.time_played > 0, pp.time_played, pm.time_played) AS time_played,
				ifNull(tks.kills, toUInt64(0)) AS team_kills
			FROM participant_matchups AS pm FINAL
			LEFT JOIN participant_performance AS pp FINAL
				ON pp.match_id = pm.match_id
				AND pp.platform = pm.platform
				AND pp.participant_id = pm.participant_id
			LEFT JOIN team_kill_summary AS tks FINAL
				ON tks.match_id = pm.match_id
				AND tks.platform = pm.platform
				AND tks.patch = pm.patch
				AND tks.queue_id = pm.queue_id
				AND tks.team_id = pm.team_id
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
		)
		SELECT
			patch,
			'ALL' AS platform,
			queue_id,
			champion_id,
			role,
			rank_bucket,
			toUInt64(sum(win)) AS wins,
			toUInt64(count()) AS games,
			toUInt64(sum(participant_kills)) AS kills,
			toUInt64(sum(participant_deaths)) AS deaths,
			toUInt64(sum(participant_assists)) AS assists,
			toUInt64(sum(gold_earned)) AS gold_earned_sum,
			toUInt64(sum(total_minions_killed + neutral_minions_killed)) AS cs_sum,
			toUInt64(sum(total_damage_dealt_to_champions)) AS damage_dealt_to_champions_sum,
			toUInt64(sum(total_damage_taken)) AS damage_taken_sum,
			toUInt64(sum(damage_self_mitigated)) AS damage_self_mitigated_sum,
			toUInt64(sum(damage_dealt_to_objectives)) AS damage_dealt_to_objectives_sum,
			toUInt64(sum(damage_dealt_to_turrets + damage_dealt_to_buildings)) AS damage_dealt_to_structures_sum,
			toUInt64(sum(vision_score)) AS vision_score_sum,
			toUInt64(sum(time_ccing_others)) AS time_ccing_others_sum,
			toUInt64(sum(total_heal + total_heals_on_teammates + total_damage_shielded_on_teammates)) AS team_utility_sum,
			toUInt64(sum(turret_takedowns + inhibitor_takedowns)) AS structure_takedowns_sum,
			toUInt64(sum(dragon_kills + baron_kills + objectives_stolen)) AS objective_takedowns_sum,
			toUInt64(sum(total_time_spent_dead)) AS total_time_spent_dead_sum,
			toUInt64(sum(time_played)) AS time_played_sum,
			sum(multiIf(team_kills > 0, toFloat64(participant_kills + participant_assists) / toFloat64(team_kills), 0)) AS kill_participation_sum
		FROM participant_rows
		GROUP BY patch, queue_id, champion_id, role, rank_bucket`,
		patch,
		queueID,
	)
	return err
}

func (r *Repository) refreshChampionGuideScopeAnalytics(ctx context.Context, patch string, queueID uint16) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO champion_guide_scope_analytics
		(patch, platform, queue_id, role, rank_bucket, participant_samples, match_count)
		WITH participant_rows AS
		(
			SELECT
				pm.match_id AS match_id,
				pm.patch AS patch,
				pm.queue_id AS queue_id,
				pm.role AS role,
				multiIf(
					s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
					pm.rank_bucket
				) AS rank_bucket
			FROM participant_matchups AS pm FINAL
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
		)
		SELECT
			patch,
			'ALL' AS platform,
			queue_id,
			role,
			rank_bucket,
			toUInt64(count()) AS participant_samples,
			toUInt64(uniqExact(match_id)) AS match_count
		FROM participant_rows
		GROUP BY patch, queue_id, role, rank_bucket
		UNION ALL
		SELECT
			patch,
			'ALL' AS platform,
			queue_id,
			role,
			'ALL' AS rank_bucket,
			toUInt64(count()) AS participant_samples,
			toUInt64(uniqExact(match_id)) AS match_count
		FROM participant_rows
		GROUP BY patch, queue_id, role
		UNION ALL
		SELECT
			patch,
			'ALL' AS platform,
			queue_id,
			'ALL' AS role,
			rank_bucket,
			toUInt64(count()) AS participant_samples,
			toUInt64(uniqExact(match_id)) AS match_count
		FROM participant_rows
		GROUP BY patch, queue_id, rank_bucket
		UNION ALL
		SELECT
			patch,
			'ALL' AS platform,
			queue_id,
			'ALL' AS role,
			'ALL' AS rank_bucket,
			toUInt64(count()) AS participant_samples,
			toUInt64(uniqExact(match_id)) AS match_count
		FROM participant_rows
		GROUP BY patch, queue_id`,
		patch,
		queueID,
	)
	return err
}

func (r *Repository) refreshChampionMatchupAnalytics(ctx context.Context, patch string, queueID uint16) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO champion_matchup_analytics
		(patch, platform, queue_id, champion_id, role, opponent_champion_id, rank_bucket, wins, games)
		SELECT
			pm.patch AS patch,
			'ALL' AS platform,
			pm.queue_id AS queue_id,
			pm.champion_id AS champion_id,
			pm.role AS role,
			pm.opponent_champion_id AS opponent_champion_id,
			multiIf(
				s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
				pm.rank_bucket
			) AS rank_bucket,
			toUInt64(sum(pm.win)) AS wins,
			toUInt64(count()) AS games
		FROM participant_matchups AS pm FINAL
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
		WHERE pm.patch = ?
			AND pm.queue_id = ?
			AND pm.champion_id > 0
			AND pm.opponent_champion_id > 0
		GROUP BY
			pm.patch,
			pm.queue_id,
			pm.champion_id,
			pm.role,
			pm.opponent_champion_id,
			rank_bucket`,
		patch,
		queueID,
	)
	return err
}

func (r *Repository) refreshChampionSignatureAnalytics(ctx context.Context, patch string, queueID uint16) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO champion_signature_analytics
		(patch, platform, queue_id, champion_id, role, rank_bucket, signature_type, signature, wins, games)
		WITH participant_rows AS
		(
			SELECT
				pm.patch AS patch,
				pm.queue_id AS queue_id,
				pm.champion_id AS champion_id,
				pm.role AS role,
				multiIf(
					s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
					pm.rank_bucket
				) AS rank_bucket,
				pm.rune_signature AS rune_signature,
				pm.spell_signature AS spell_signature,
				pm.win AS win
			FROM participant_matchups AS pm FINAL
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
			WHERE pm.patch = ?
				AND pm.queue_id = ?
				AND pm.champion_id > 0
		)
		SELECT
			patch,
			'ALL' AS platform,
			queue_id,
			champion_id,
			role,
			rank_bucket,
			signature_type,
			signature,
			toUInt64(sum(win)) AS wins,
			toUInt64(count()) AS games
		FROM
		(
			SELECT
				patch,
				queue_id,
				champion_id,
				role,
				rank_bucket,
				'rune' AS signature_type,
				rune_signature AS signature,
				win
			FROM participant_rows
			WHERE rune_signature != ''
			UNION ALL
			SELECT
				patch,
				queue_id,
				champion_id,
				role,
				rank_bucket,
				'spell' AS signature_type,
				spell_signature AS signature,
				win
			FROM participant_rows
			WHERE spell_signature != ''
		)
		GROUP BY
			patch,
			queue_id,
			champion_id,
			role,
			rank_bucket,
			signature_type,
			signature`,
		patch,
		queueID,
	)
	return err
}

func (r *Repository) refreshBuildSignatureAnalytics(ctx context.Context, patch string, queueID uint16) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO build_signature_analytics
		(patch, platform, queue_id, champion_id, role, opponent_champion_id, rank_bucket, final_items_signature, core2_signature, core3_signature, rune_signature, spell_signature, wins, games)
		SELECT
			pm.patch AS patch,
			'ALL' AS platform,
			pm.queue_id AS queue_id,
			pm.champion_id AS champion_id,
			pm.role AS role,
			pm.opponent_champion_id AS opponent_champion_id,
			multiIf(
				s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
				pm.rank_bucket
			) AS rank_bucket,
			pm.final_items_signature AS final_items_signature,
			pm.core2_signature AS core2_signature,
			pm.core3_signature AS core3_signature,
			pm.rune_signature AS rune_signature,
			pm.spell_signature AS spell_signature,
			toUInt64(sum(pm.win)) AS wins,
			toUInt64(count()) AS games
		FROM participant_matchups AS pm FINAL
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
			pm.opponent_champion_id,
			rank_bucket,
			pm.final_items_signature,
			pm.core2_signature,
			pm.core3_signature,
			pm.rune_signature,
			pm.spell_signature`,
		patch,
		queueID,
	)
	return err
}

func (r *Repository) refreshChampionBuildVariantAnalytics(ctx context.Context, patch string, queueID uint16) error {
	skillChoices, err := r.queryChampionBuildVariantSkillChoices(ctx, patch, queueID)
	if err != nil {
		return err
	}
	sourceRows, err := r.queryChampionBuildVariantSourceRows(ctx, patch, queueID)
	if err != nil {
		return err
	}
	labelGroups := map[string]*championBuildVariantRefreshAggregate{}
	for _, source := range sourceRows {
		coreKey := buildVariantCoreKey(source.Core3Signature, source.FinalItemsSignature, source.Core2Signature)
		if coreKey == "" {
			continue
		}
		source.VariantKey = coreKey
		source.Core2Signature = coreKey
		source.VariantLabel, source.VariantTags = buildVariantLabelAndTags(source.Core2Signature + "-" + source.Core3Signature + "-" + source.FinalItemsSignature)
		groupKey := buildVariantGroupKey(source.ChampionGuideBuildVariantRow)
		key := championBuildVariantAnalyticsKey(source.ChampionID, source.Role, source.RankBucket, groupKey)
		group := labelGroups[key]
		if group == nil {
			source.VariantKey = groupKey
			group = &championBuildVariantRefreshAggregate{
				row:                 source,
				representativeGames: source.Games,
			}
			labelGroups[key] = group
			continue
		}
		group.row.Wins += source.Wins
		group.row.Games += source.Games
		group.row.BuildCount += source.BuildCount
		group.row.VariantTags = mergeBuildVariantTags(group.row.VariantTags, source.VariantTags)
		if source.Games > group.representativeGames {
			group.row.Core2Signature = source.Core2Signature
			group.row.Core3Signature = source.Core3Signature
			group.row.FinalItemsSignature = source.FinalItemsSignature
			group.row.RuneSignature = source.RuneSignature
			group.row.SpellSignature = source.SpellSignature
			group.representativeGames = source.Games
		}
	}
	out := make([]championBuildVariantAnalyticsRow, 0, len(labelGroups))
	for key, group := range labelGroups {
		if skill, ok := skillChoices[key]; ok {
			group.row.SkillOrderSignature = skill.Signature
			group.row.SkillOrderWins = skill.Wins
			group.row.SkillOrderGames = skill.Games
		}
		out = append(out, group.row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ChampionID != out[j].ChampionID {
			return out[i].ChampionID < out[j].ChampionID
		}
		if out[i].Role != out[j].Role {
			return out[i].Role < out[j].Role
		}
		if out[i].RankBucket != out[j].RankBucket {
			return out[i].RankBucket < out[j].RankBucket
		}
		if out[i].Games != out[j].Games {
			return out[i].Games > out[j].Games
		}
		return out[i].VariantKey < out[j].VariantKey
	})
	return r.insertChampionBuildVariantAnalytics(ctx, patch, queueID, out)
}

func (r *Repository) queryChampionBuildVariantSourceRows(ctx context.Context, patch string, queueID uint16) ([]championBuildVariantAnalyticsRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			champion_id,
			role,
			rank_bucket,
			core2_signature,
			argMax(core3_signature, row_games) AS core3_signature,
			argMax(final_items_signature, row_games) AS final_items_signature,
			argMax(rune_signature, row_games) AS rune_signature,
			argMax(spell_signature, row_games) AS spell_signature,
			toUInt64(sum(row_wins)) AS wins,
			toUInt64(sum(row_games)) AS games,
			count() AS build_count
		FROM
		(
			SELECT
				pm.champion_id AS champion_id,
				pm.role AS role,
				multiIf(
					s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
					pm.rank_bucket
				) AS rank_bucket,
				pm.core2_signature AS core2_signature,
				pm.core3_signature AS core3_signature,
				pm.final_items_signature AS final_items_signature,
				pm.rune_signature AS rune_signature,
				pm.spell_signature AS spell_signature,
				toUInt64(sum(pm.win)) AS row_wins,
				toUInt64(count()) AS row_games
			FROM participant_matchups AS pm FINAL
			LEFT JOIN
			(
				SELECT DISTINCT patch, platform, queue_id
				FROM patch_build_metrics FINAL
			) AS cbm
				ON cbm.patch = pm.patch
				AND cbm.platform = pm.platform
				AND cbm.queue_id = pm.queue_id
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
			WHERE pm.patch = ?
				AND pm.queue_id = ?
				AND cbm.patch = ''
				AND pm.core2_signature != ''
				AND pm.final_items_signature != ''
			GROUP BY
				pm.champion_id,
				pm.role,
				rank_bucket,
				pm.core2_signature,
				pm.core3_signature,
				pm.final_items_signature,
				pm.rune_signature,
				pm.spell_signature
			UNION ALL
			SELECT
				champion_id,
				role,
				rank_bucket,
				core2_signature,
				core3_signature,
				final_items_signature,
				rune_signature,
				spell_signature,
				toUInt64(sum(wins)) AS row_wins,
				toUInt64(sum(games)) AS row_games
			FROM patch_build_metrics FINAL
			WHERE patch = ?
				AND queue_id = ?
				AND core2_signature != ''
				AND final_items_signature != ''
			GROUP BY
				champion_id,
				role,
				rank_bucket,
				core2_signature,
				core3_signature,
				final_items_signature,
				rune_signature,
				spell_signature
		)
		GROUP BY champion_id, role, rank_bucket, core2_signature
		HAVING games > 0`,
		patch,
		queueID,
		patch,
		queueID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []championBuildVariantAnalyticsRow{}
	for rows.Next() {
		var row championBuildVariantAnalyticsRow
		if err := rows.Scan(
			&row.ChampionID,
			&row.Role,
			&row.RankBucket,
			&row.Core2Signature,
			&row.Core3Signature,
			&row.FinalItemsSignature,
			&row.RuneSignature,
			&row.SpellSignature,
			&row.Wins,
			&row.Games,
			&row.BuildCount,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) queryChampionBuildVariantSkillChoices(ctx context.Context, patch string, queueID uint16) (map[string]championBuildVariantSkillChoice, error) {
	rows, err := r.db.QueryContext(ctx, `
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
			FROM timeline_skill_events FINAL
			WHERE patch = ? AND queue_id = ? AND skill_slot BETWEEN 1 AND 4
			GROUP BY match_id, participant_id
			HAVING skill_order_signature != ''
		)
		SELECT
			pm.champion_id,
			pm.role,
			multiIf(
				s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
				pm.rank_bucket
			) AS rank_bucket,
			pm.core2_signature,
			pm.core3_signature,
			pm.final_items_signature,
			sp.skill_order_signature,
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
		WHERE pm.patch = ?
			AND pm.queue_id = ?
			AND pm.core2_signature != ''
			AND pm.final_items_signature != ''
		GROUP BY
			pm.champion_id,
			pm.role,
			rank_bucket,
			pm.core2_signature,
			pm.core3_signature,
			pm.final_items_signature,
			sp.skill_order_signature`,
		patch,
		queueID,
		patch,
		queueID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byVariant := map[string]map[string]buildVariantSkillAggregate{}
	for rows.Next() {
		var (
			championID          uint16
			role, rankBucket    string
			core2, core3, final string
			signature           string
			wins, games         int
		)
		if err := rows.Scan(&championID, &role, &rankBucket, &core2, &core3, &final, &signature, &wins, &games); err != nil {
			return nil, err
		}
		coreKey := buildVariantCoreKey(core3, final, core2)
		if coreKey == "" {
			continue
		}
		label, tags := buildVariantLabelAndTags(core2 + "-" + core3 + "-" + final)
		groupKey := buildVariantGroupKey(ChampionGuideBuildVariantRow{
			VariantKey:   coreKey,
			VariantLabel: label,
			VariantTags:  tags,
		})
		key := championBuildVariantAnalyticsKey(championID, role, rankBucket, groupKey)
		if byVariant[key] == nil {
			byVariant[key] = map[string]buildVariantSkillAggregate{}
		}
		current := byVariant[key][signature]
		current.Wins += wins
		current.Games += games
		byVariant[key][signature] = current
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := map[string]championBuildVariantSkillChoice{}
	for key, skills := range byVariant {
		bestSignature := ""
		best := buildVariantSkillAggregate{}
		for signature, aggregate := range skills {
			if aggregate.Games < buildVariantSkillOrderMinGames {
				continue
			}
			if bestSignature == "" || aggregate.Games > best.Games || (aggregate.Games == best.Games && aggregate.Wins > best.Wins) {
				bestSignature = signature
				best = aggregate
			}
		}
		if bestSignature == "" || best.Games <= 0 {
			continue
		}
		out[key] = championBuildVariantSkillChoice{
			Signature: bestSignature,
			Wins:      best.Wins,
			Games:     best.Games,
		}
	}
	return out, nil
}

func (r *Repository) insertChampionBuildVariantAnalytics(ctx context.Context, patch string, queueID uint16, rows []championBuildVariantAnalyticsRow) error {
	if len(rows) == 0 {
		return nil
	}
	for _, row := range rows {
		if _, err := r.db.ExecContext(
			ctx,
			`INSERT INTO champion_build_variant_analytics
			(patch, platform, queue_id, champion_id, role, rank_bucket, variant_key, variant_label, variant_tags, core2_signature, core3_signature, final_items_signature, rune_signature, spell_signature, skill_order_signature, skill_order_wins, skill_order_games, wins, games, build_count)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			patch,
			"ALL",
			queueID,
			row.ChampionID,
			row.Role,
			row.RankBucket,
			row.VariantKey,
			row.VariantLabel,
			row.VariantTags,
			row.Core2Signature,
			row.Core3Signature,
			row.FinalItemsSignature,
			row.RuneSignature,
			row.SpellSignature,
			row.SkillOrderSignature,
			row.SkillOrderWins,
			row.SkillOrderGames,
			row.Wins,
			row.Games,
			row.BuildCount,
		); err != nil {
			return err
		}
	}
	return nil
}

func championBuildVariantAnalyticsKey(championID uint16, role, rankBucket, variantKey string) string {
	return fmt.Sprintf("%d\x00%s\x00%s\x00%s", championID, role, rankBucket, variantKey)
}

func (r *Repository) queryChampionGuideSkillOrders(ctx context.Context, filters map[string]string, minGames, limit int) ([]ChampionGuideSkillOrderRow, error) {
	if minGames <= 0 {
		minGames = 5
	}
	if limit <= 0 {
		limit = 12
	}
	roleScope := strictAnalyticsRoleScope(filters["role"])
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
