package clickhouse

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"winrift/core/internal/analytics"
)

type BuildRow struct {
	ChampionID          uint16
	Role                string
	OpponentChampionID  uint16
	PatchBucket         string
	RankBucket          string
	FinalItemsSignature string
	Core2Signature      string
	Core3Signature      string
	RuneSignature       string
	SpellSignature      string
	Wins                int
	Games               int
	WinRate             float64
	Confidence          float64
}

func (r *Repository) QueryBuilds(ctx context.Context, filters map[string]string, minGames, limit int) ([]BuildRow, error) {
	if minGames <= 0 {
		minGames = 1
	}
	if limit <= 0 {
		limit = 25
	}
	rows, hasSummary, err := r.queryBuildsSummary(ctx, filters, minGames, limit)
	if err != nil {
		return nil, err
	}
	if hasSummary {
		return rows, nil
	}
	return r.queryBuildsLiveScan(ctx, filters, minGames, limit)
}

func (r *Repository) queryBuildsSummary(ctx context.Context, filters map[string]string, minGames, limit int) ([]BuildRow, bool, error) {
	hasSummary, err := r.BuildSignatureAnalyticsHasData(ctx, filters["patch"])
	if err != nil {
		return nil, false, err
	}
	if !hasSummary {
		return nil, false, nil
	}
	roleScope := analyticsRoleScope(filters["role"])
	opponentBucketExpr := analyticsOpponentBucketExpr(filters)
	query := fmt.Sprintf(`
		SELECT
			champion_id,
			%s AS role_bucket,
			%s AS opponent_champion_id,
			patch AS patch_bucket,
			rank_bucket,
			final_items_signature,
			core2_signature,
			core3_signature,
			rune_signature,
			spell_signature,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games,
			wins / games AS win_rate
		FROM (`+buildSignatureSummarySourceSQL()+`)
		WHERE 1 = 1`, roleScope.selectExpr, opponentBucketExpr)
	args := []any{}
	if filters["champion_id"] != "" {
		query += " AND champion_id = ?"
		args = append(args, filters["champion_id"])
	}
	if roleScope.whereSQL != "" {
		query += " AND " + roleScope.whereSQL
		args = append(args, roleScope.args...)
	}
	if filters["opponent_champion_id"] != "" {
		query += " AND opponent_champion_id = ?"
		args = append(args, filters["opponent_champion_id"])
	}
	if filters["patch"] != "" {
		query += " AND patch = ?"
		args = append(args, filters["patch"])
	}
	if filters["rank_bucket"] != "" {
		query += " AND rank_bucket = ?"
		args = append(args, filters["rank_bucket"])
	}
	query += `
		GROUP BY champion_id, role_bucket, opponent_champion_id, patch_bucket, rank_bucket, final_items_signature, core2_signature, core3_signature, rune_signature, spell_signature
		HAVING games >= ?
		ORDER BY games DESC, win_rate DESC
		LIMIT ?`
	args = append(args, minGames, limit*5)
	rows, err := r.scanBuildRows(ctx, query, args, limit)
	return rows, true, err
}

func (r *Repository) BuildSignatureAnalyticsHasData(ctx context.Context, patch string) (bool, error) {
	patch = strings.TrimSpace(patch)
	query := "SELECT count() FROM build_signature_analytics WHERE 1 = 1"
	args := []any{}
	if patch != "" {
		query += " AND patch = ?"
		args = append(args, patch)
	}
	query += " LIMIT 1"
	var count uint64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	query = "SELECT count() FROM patch_build_metrics WHERE 1 = 1"
	args = []any{}
	if patch != "" {
		query += " AND patch = ?"
		args = append(args, patch)
	}
	query += " LIMIT 1"
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count > 0, err
}

func buildSignatureSummarySourceSQL() string {
	return `
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
			wins,
			games
		FROM build_signature_analytics FINAL
		UNION ALL
		SELECT
			pbm.patch,
			pbm.platform,
			pbm.queue_id,
			pbm.champion_id,
			pbm.role,
			pbm.opponent_champion_id,
			pbm.rank_bucket,
			pbm.final_items_signature,
			pbm.core2_signature,
			pbm.core3_signature,
			pbm.rune_signature,
			pbm.spell_signature,
			pbm.wins,
			pbm.games
		FROM patch_build_metrics AS pbm FINAL
		LEFT JOIN
		(
			SELECT DISTINCT patch, queue_id
			FROM build_signature_analytics FINAL
		) AS bsa
			ON bsa.patch = pbm.patch AND bsa.queue_id = pbm.queue_id
		WHERE bsa.patch = ''`
}

func (r *Repository) queryBuildsLiveScan(ctx context.Context, filters map[string]string, minGames, limit int) ([]BuildRow, error) {
	roleScope := analyticsRoleScope(filters["role"])
	opponentBucketExpr := analyticsOpponentBucketExpr(filters)
	query := fmt.Sprintf(`
		SELECT
			champion_id,
			%s AS role_bucket,
			%s AS opponent_champion_id,
			patch_bucket,
			rank_bucket,
			final_items_signature,
			core2_signature,
			core3_signature,
			rune_signature,
			spell_signature,
			sum(wins) AS wins,
			sum(games) AS games,
			wins / games AS win_rate
		FROM
		(
			SELECT
				champion_id,
				role,
				opponent_champion_id,
				patch AS patch_bucket,
				rank_bucket,
				final_items_signature,
				core2_signature,
				core3_signature,
				rune_signature,
				spell_signature,
				toUInt64(sum(win)) AS wins,
				toUInt64(count()) AS games
			FROM
			(
				SELECT
					pm.champion_id,
					pm.role,
					pm.opponent_champion_id,
					pm.patch AS patch,
					multiIf(
						s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
						pm.rank_bucket
					) AS rank_bucket,
					pm.final_items_signature,
					pm.core2_signature,
					pm.core3_signature,
					pm.rune_signature,
					pm.spell_signature,
					pm.win
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
				WHERE cbm.patch = ''
			)
			GROUP BY
				champion_id,
				role,
				opponent_champion_id,
				patch_bucket,
				rank_bucket,
				final_items_signature,
				core2_signature,
				core3_signature,
				rune_signature,
				spell_signature
			UNION ALL
			SELECT
				champion_id,
				role,
				opponent_champion_id,
				patch AS patch_bucket,
				rank_bucket,
				final_items_signature,
				core2_signature,
				core3_signature,
				rune_signature,
				spell_signature,
				wins,
				games
			FROM patch_build_metrics FINAL
		)
		WHERE 1 = 1`, roleScope.selectExpr, opponentBucketExpr)
	args := []any{}
	if filters["champion_id"] != "" {
		query += " AND champion_id = ?"
		args = append(args, filters["champion_id"])
	}
	if roleScope.whereSQL != "" {
		query += " AND " + roleScope.whereSQL
		args = append(args, roleScope.args...)
	}
	if filters["opponent_champion_id"] != "" {
		query += " AND opponent_champion_id = ?"
		args = append(args, filters["opponent_champion_id"])
	}
	if filters["patch"] != "" {
		query += " AND patch_bucket = ?"
		args = append(args, filters["patch"])
	}
	if filters["rank_bucket"] != "" {
		query += " AND rank_bucket = ?"
		args = append(args, filters["rank_bucket"])
	}
	query += ` GROUP BY champion_id, role_bucket, opponent_champion_id, patch_bucket, rank_bucket, final_items_signature, core2_signature, core3_signature, rune_signature, spell_signature HAVING games >= ? ORDER BY games DESC, win_rate DESC LIMIT ?`
	args = append(args, minGames, limit*5)

	return r.scanBuildRows(ctx, query, args, limit)
}

func (r *Repository) scanBuildRows(ctx context.Context, query string, args []any, limit int) ([]BuildRow, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BuildRow
	for rows.Next() {
		var row BuildRow
		if err := rows.Scan(&row.ChampionID, &row.Role, &row.OpponentChampionID, &row.PatchBucket, &row.RankBucket, &row.FinalItemsSignature, &row.Core2Signature, &row.Core3Signature, &row.RuneSignature, &row.SpellSignature, &row.Wins, &row.Games, &row.WinRate); err != nil {
			return nil, err
		}
		row.Confidence = analytics.WilsonLowerBound(row.Wins, row.Games, 1.96)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		if out[i].Games != out[j].Games {
			return out[i].Games > out[j].Games
		}
		return out[i].WinRate > out[j].WinRate
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
