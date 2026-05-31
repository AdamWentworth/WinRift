package clickhouse

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"winrift/core/internal/analytics"
)

func (r *Repository) queryChampionGuideMatchups(ctx context.Context, filters map[string]string, minGames, limit int, toughest bool) ([]ChampionGuideMatchupRow, error) {
	rows, hasSummary, err := r.queryChampionGuideMatchupsSummary(ctx, filters, minGames, limit, toughest)
	if err != nil {
		return nil, err
	}
	if hasSummary {
		return rows, nil
	}
	return r.queryChampionGuideMatchupsLiveScan(ctx, filters, minGames, limit, toughest)
}

func (r *Repository) queryChampionGuideMatchupsSummary(ctx context.Context, filters map[string]string, minGames, limit int, toughest bool) ([]ChampionGuideMatchupRow, bool, error) {
	championID := filterValue(filters["champion_id"])
	if championID == "" {
		return nil, false, nil
	}
	if minGames <= 0 {
		minGames = 5
	}
	if limit <= 0 {
		limit = 12
	}
	roleScope := strictAnalyticsRoleScope(filters["role"])
	order := "win_rate DESC, games DESC"
	if toughest {
		order = "win_rate ASC, games DESC"
	}
	query := fmt.Sprintf(`
		SELECT
			opponent_champion_id,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games,
			wins / games AS win_rate
		FROM champion_matchup_analytics FINAL
		WHERE platform = 'ALL'
			AND queue_id = ?
			AND champion_id = ?
			AND %s
			AND opponent_champion_id > 0`,
		guideRolePredicate(roleScope),
	)
	args := []any{analytics.RankedSoloQueueID, championID}
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
		GROUP BY opponent_champion_id
		HAVING games >= ?
		ORDER BY ` + order + `
		LIMIT ?`
	args = append(args, minGames, limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []ChampionGuideMatchupRow{}
	for rows.Next() {
		var row ChampionGuideMatchupRow
		if err := rows.Scan(&row.OpponentChampionID, &row.Wins, &row.Games, &row.WinRate); err != nil {
			return nil, false, err
		}
		row.Confidence = analytics.WilsonLowerBound(row.Wins, row.Games, 1.96)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(out) > 0 {
		return out, true, nil
	}
	hasSummary, err := r.championGuideAnalyticsHasData(ctx, "champion_matchup_analytics", filters)
	if err != nil {
		return nil, false, err
	}
	return out, hasSummary, nil
}

func (r *Repository) queryChampionGuideMatchupsLiveScan(ctx context.Context, filters map[string]string, minGames, limit int, toughest bool) ([]ChampionGuideMatchupRow, error) {
	roleScope := strictAnalyticsRoleScope(filters["role"])
	baseSQL, args := championGuideBaseSQL(filters, roleScope, true)
	order := "win_rate DESC, games DESC"
	if toughest {
		order = "win_rate ASC, games DESC"
	}
	query := `
		SELECT
			opponent_champion_id,
			sum(win) AS wins,
			count() AS games,
			wins / games AS win_rate
		` + baseSQL + `
			AND opponent_champion_id > 0
		GROUP BY opponent_champion_id
		HAVING games >= ?
		ORDER BY ` + order + `
		LIMIT ?`
	args = append(args, minGames, limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChampionGuideMatchupRow{}
	for rows.Next() {
		var row ChampionGuideMatchupRow
		if err := rows.Scan(&row.OpponentChampionID, &row.Wins, &row.Games, &row.WinRate); err != nil {
			return nil, err
		}
		row.Confidence = analytics.WilsonLowerBound(row.Wins, row.Games, 1.96)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) queryChampionGuideSignatures(ctx context.Context, filters map[string]string, column string, minGames, limit int) ([]ChampionGuideSignatureRow, error) {
	if column != "rune_signature" && column != "spell_signature" {
		return nil, nil
	}
	rows, hasSummary, err := r.queryChampionGuideSignaturesSummary(ctx, filters, column, minGames, limit)
	if err != nil {
		return nil, err
	}
	if hasSummary {
		return rows, nil
	}
	return r.queryChampionGuideSignaturesLiveScan(ctx, filters, column, minGames, limit)
}

func (r *Repository) queryChampionGuideSignaturesSummary(ctx context.Context, filters map[string]string, column string, minGames, limit int) ([]ChampionGuideSignatureRow, bool, error) {
	signatureType := championGuideSignatureType(column)
	if signatureType == "" {
		return nil, false, nil
	}
	championID := filterValue(filters["champion_id"])
	if championID == "" {
		return nil, false, nil
	}
	if minGames <= 0 {
		minGames = 5
	}
	if limit <= 0 {
		limit = 12
	}
	roleScope := strictAnalyticsRoleScope(filters["role"])
	query := fmt.Sprintf(`
		SELECT
			signature,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games,
			wins / games AS win_rate
		FROM champion_signature_analytics FINAL
		WHERE platform = 'ALL'
			AND queue_id = ?
			AND champion_id = ?
			AND signature_type = ?
			AND %s
			AND signature != ''`,
		guideRolePredicate(roleScope),
	)
	args := []any{analytics.RankedSoloQueueID, championID, signatureType}
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
		GROUP BY signature
		HAVING games >= ?
		ORDER BY games DESC, win_rate DESC
		LIMIT ?`
	args = append(args, minGames, limit*4)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []ChampionGuideSignatureRow{}
	for rows.Next() {
		var row ChampionGuideSignatureRow
		if err := rows.Scan(&row.Signature, &row.Wins, &row.Games, &row.WinRate); err != nil {
			return nil, false, err
		}
		row.Confidence = analytics.WilsonLowerBound(row.Wins, row.Games, 1.96)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	out = sortChampionGuideSignatureRows(out, limit)
	if len(out) > 0 {
		return out, true, nil
	}
	hasSummary, err := r.championGuideAnalyticsHasData(ctx, "champion_signature_analytics", filters)
	if err != nil {
		return nil, false, err
	}
	return out, hasSummary, nil
}

func (r *Repository) queryChampionGuideSignaturesLiveScan(ctx context.Context, filters map[string]string, column string, minGames, limit int) ([]ChampionGuideSignatureRow, error) {
	roleScope := strictAnalyticsRoleScope(filters["role"])
	baseSQL, args := championGuideBaseSQL(filters, roleScope, true)
	query := `
		SELECT
			` + column + ` AS signature,
			sum(win) AS wins,
			count() AS games,
			wins / games AS win_rate
		` + baseSQL + `
			AND ` + column + ` != ''
		GROUP BY signature
		HAVING games >= ?
		ORDER BY games DESC, win_rate DESC
		LIMIT ?`
	args = append(args, minGames, limit*4)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChampionGuideSignatureRow{}
	for rows.Next() {
		var row ChampionGuideSignatureRow
		if err := rows.Scan(&row.Signature, &row.Wins, &row.Games, &row.WinRate); err != nil {
			return nil, err
		}
		row.Confidence = analytics.WilsonLowerBound(row.Wins, row.Games, 1.96)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sortChampionGuideSignatureRows(out, limit), nil
}

func sortChampionGuideSignatureRows(rows []ChampionGuideSignatureRow, limit int) []ChampionGuideSignatureRow {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Confidence != rows[j].Confidence {
			return rows[i].Confidence > rows[j].Confidence
		}
		if rows[i].WinRate != rows[j].WinRate {
			return rows[i].WinRate > rows[j].WinRate
		}
		if rows[i].Games != rows[j].Games {
			return rows[i].Games > rows[j].Games
		}
		return rows[i].Signature < rows[j].Signature
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

func championGuideSignatureType(column string) string {
	switch column {
	case "rune_signature":
		return "rune"
	case "spell_signature":
		return "spell"
	default:
		return ""
	}
}

func (r *Repository) championGuideAnalyticsHasData(ctx context.Context, table string, filters map[string]string) (bool, error) {
	switch table {
	case "champion_matchup_analytics", "champion_signature_analytics":
	default:
		return false, fmt.Errorf("unsupported champion guide analytics table %q", table)
	}
	query := fmt.Sprintf(`
		SELECT count()
		FROM %s FINAL
		WHERE platform = 'ALL'
			AND queue_id = ?`, table)
	args := []any{analytics.RankedSoloQueueID}
	if filterValue(filters["patch"]) != "" {
		query += " AND patch = ?"
		args = append(args, filterValue(filters["patch"]))
	}
	var count uint64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func championGuideBaseSQL(filters map[string]string, roleScope roleAnalyticsScope, includeChampion bool) (string, []any) {
	return championGuideBaseSQLInternal(filters, roleScope, includeChampion, false)
}

func championGuideBaseSQLExcludingCompiledBuilds(filters map[string]string, roleScope roleAnalyticsScope, includeChampion bool) (string, []any) {
	return championGuideBaseSQLInternal(filters, roleScope, includeChampion, true)
}

func championGuideBaseSQLInternal(filters map[string]string, roleScope roleAnalyticsScope, includeChampion bool, excludeCompiledBuilds bool) (string, []any) {
	query := `
		FROM
		(
			SELECT
				pm.match_id AS match_id,
				pm.champion_id AS champion_id,
				pm.role AS role,
				pm.opponent_champion_id AS opponent_champion_id,
				pm.patch AS patch,
				multiIf(
					s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
					pm.rank_bucket
				) AS rank_bucket,
				pm.rune_signature AS rune_signature,
				pm.spell_signature AS spell_signature,
				pm.final_items_signature AS final_items_signature,
				pm.core2_signature AS core2_signature,
				pm.core3_signature AS core3_signature,
				pm.win AS win,
				pm.kills AS kills,
				pm.deaths AS deaths,
				pm.assists AS assists,
				multiIf(pp.gold_earned > 0, pp.gold_earned, pm.gold_earned) AS gold_earned,
				multiIf(pp.gold_spent > 0, pp.gold_spent, pm.gold_spent) AS gold_spent,
				multiIf(pp.total_minions_killed > 0, pp.total_minions_killed, pm.total_minions_killed) AS total_minions_killed,
				multiIf(pp.neutral_minions_killed > 0, pp.neutral_minions_killed, pm.neutral_minions_killed) AS neutral_minions_killed,
				multiIf(pp.total_damage_dealt_to_champions > 0, pp.total_damage_dealt_to_champions, pm.total_damage_dealt_to_champions) AS total_damage_dealt_to_champions,
				multiIf(pp.physical_damage_dealt_to_champions > 0, pp.physical_damage_dealt_to_champions, pm.physical_damage_dealt_to_champions) AS physical_damage_dealt_to_champions,
				multiIf(pp.magic_damage_dealt_to_champions > 0, pp.magic_damage_dealt_to_champions, pm.magic_damage_dealt_to_champions) AS magic_damage_dealt_to_champions,
				multiIf(pp.true_damage_dealt_to_champions > 0, pp.true_damage_dealt_to_champions, pm.true_damage_dealt_to_champions) AS true_damage_dealt_to_champions,
				multiIf(pp.total_damage_taken > 0, pp.total_damage_taken, pm.total_damage_taken) AS total_damage_taken,
				multiIf(pp.damage_self_mitigated > 0, pp.damage_self_mitigated, pm.damage_self_mitigated) AS damage_self_mitigated,
				multiIf(pp.damage_dealt_to_objectives > 0, pp.damage_dealt_to_objectives, pm.damage_dealt_to_objectives) AS damage_dealt_to_objectives,
				multiIf(pp.damage_dealt_to_turrets > 0, pp.damage_dealt_to_turrets, pm.damage_dealt_to_turrets) AS damage_dealt_to_turrets,
				multiIf(pp.damage_dealt_to_buildings > 0, pp.damage_dealt_to_buildings, pm.damage_dealt_to_buildings) AS damage_dealt_to_buildings,
				multiIf(pp.vision_score > 0, pp.vision_score, pm.vision_score) AS vision_score,
				multiIf(pp.wards_placed > 0, pp.wards_placed, pm.wards_placed) AS wards_placed,
				multiIf(pp.wards_killed > 0, pp.wards_killed, pm.wards_killed) AS wards_killed,
				multiIf(pp.detector_wards_placed > 0, pp.detector_wards_placed, pm.detector_wards_placed) AS detector_wards_placed,
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
				sum(pm.kills) OVER (PARTITION BY pm.match_id, pm.team_id) AS team_kills
			FROM participant_matchups AS pm FINAL
			LEFT JOIN participant_performance AS pp FINAL
				ON pp.match_id = pm.match_id
				AND pp.platform = pm.platform
				AND pp.participant_id = pm.participant_id
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
`
	if excludeCompiledBuilds {
		query += `
			LEFT JOIN
			(
				SELECT DISTINCT patch, platform, queue_id
				FROM patch_build_metrics FINAL
			) AS cbm
				ON cbm.patch = pm.patch
				AND cbm.platform = pm.platform
				AND cbm.queue_id = pm.queue_id
			WHERE cbm.patch = ''
`
	}
	query += `
		)
		WHERE 1 = 1`
	args := []any{}
	if includeChampion && strings.TrimSpace(filters["champion_id"]) != "" {
		query += " AND champion_id = ?"
		args = append(args, filters["champion_id"])
	}
	if roleScope.whereSQL != "" {
		query += " AND " + roleScope.whereSQL
		args = append(args, roleScope.args...)
	}
	if filterValue(filters["patch"]) != "" {
		query += " AND patch = ?"
		args = append(args, filterValue(filters["patch"]))
	}
	if filterValue(filters["rank_bucket"]) != "" {
		query += " AND rank_bucket = ?"
		args = append(args, filterValue(filters["rank_bucket"]))
	}
	return query, args
}

func filterValue(value string) string {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "ALL") {
		return ""
	}
	return value
}

func patchBucketLabel(value string) string {
	value = filterValue(value)
	if value == "" {
		return "ALL"
	}
	return value
}

func rankBucketLabel(value string) string {
	value = strings.ToUpper(filterValue(value))
	if value == "" {
		return "ALL"
	}
	return value
}

func roleLabel(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "ALL"
	}
	return value
}

func parseUint16Filter(value string) (uint16, bool) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 16)
	if err != nil {
		return 0, false
	}
	return uint16(parsed), true
}
