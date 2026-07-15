package clickhouse

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"winrift/core/internal/analytics"
)

type ItemSlotRow struct {
	ChampionID         uint16
	Role               string
	OpponentChampionID uint16
	PatchBucket        string
	RankBucket         string
	ItemSlot           uint8
	ItemID             uint32
	Wins               int
	Games              int
	WinRate            float64
	Confidence         float64
}

type StartingItemLoadoutRow struct {
	ChampionID         uint16
	Role               string
	OpponentChampionID uint16
	PatchBucket        string
	RankBucket         string
	ItemSignature      string
	Wins               int
	Games              int
	WinRate            float64
	Confidence         float64
}

type ItemSlotAnalyticsContext struct {
	Key             string
	ItemIDs         []uint32
	StartingItemIDs []uint32
}

type StartingLoadoutAnalyticsContext struct {
	Key              string
	OpeningItemCosts map[uint32]uint32
}

const startingItemWindowMS uint32 = 120000
const openingPurchaseFirstWindowMS uint32 = 45000
const openingPurchaseBurstWindowMS uint32 = 20000
const openingPurchaseGoldCap uint32 = 500

func (r *Repository) QueryItemSlots(ctx context.Context, filters map[string]string, itemContext string, allowedItemIDs, startingItemIDs []uint32, minGames, limit int) ([]ItemSlotRow, error) {
	completionItemIDs := removeItemIDs(allowedItemIDs, startingItemIDs)
	if len(completionItemIDs) == 0 && len(startingItemIDs) == 0 {
		return nil, nil
	}
	if minGames <= 0 {
		minGames = 1
	}
	if limit <= 0 {
		limit = 25
	}
	itemContext = normalizedItemContext(itemContext)
	rows, err := r.queryItemSlotsSummary(ctx, filters, itemContext, minGames, limit)
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}
	hasSummary, err := r.ItemSlotAnalyticsHasData(ctx, itemContext)
	if err != nil {
		return nil, err
	}
	if hasSummary {
		return rows, nil
	}
	return r.queryItemSlotsLiveScan(ctx, filters, completionItemIDs, startingItemIDs, minGames, limit)
}

func (r *Repository) QueryStartingItemLoadouts(ctx context.Context, filters map[string]string, itemContext string, openingItemCosts map[uint32]uint32, minGames, limit int) ([]StartingItemLoadoutRow, error) {
	if len(openingItemCosts) == 0 {
		return nil, nil
	}
	if minGames <= 0 {
		minGames = 1
	}
	if limit <= 0 {
		limit = 6
	}
	itemContext = normalizedItemContext(itemContext)
	rows, err := r.queryStartingItemLoadoutsSummary(ctx, filters, itemContext, minGames, limit)
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}
	hasSummary, err := r.StartingLoadoutAnalyticsHasData(ctx, itemContext, filters["patch"])
	if err != nil {
		return nil, err
	}
	if hasSummary {
		return rows, nil
	}
	return r.queryStartingItemLoadoutsLiveScan(ctx, filters, openingItemCosts, minGames, limit)
}

func (r *Repository) queryStartingItemLoadoutsLiveScan(ctx context.Context, filters map[string]string, openingItemCosts map[uint32]uint32, minGames, limit int) ([]StartingItemLoadoutRow, error) {
	openingItemIDs := uint32MapKeysSorted(openingItemCosts)
	itemList := uint32ListSQL(openingItemIDs)
	itemCostExpr := itemCostExpressionSQL("tie.item_id", openingItemCosts)
	patchBucketExpr := "'ALL'"
	rankBucketExpr := "'ALL'"
	opponentBucketExpr := analyticsOpponentBucketExpr(filters)
	if filters["patch"] != "" {
		patchBucketExpr = "patch_value"
	}
	if filters["rank_bucket"] != "" {
		rankBucketExpr = "rank_value"
	}
	roleScope := analyticsRoleScope(filters["role"])
	rawFilters := ""
	args := []any{}
	if filters["champion_id"] != "" {
		rawFilters += " AND pm.champion_id = ?"
		args = append(args, filters["champion_id"])
	}
	if roleScope.whereSQL != "" {
		rawFilters += " AND " + qualifyRoleWhereSQL(roleScope.whereSQL, "pm.role")
		args = append(args, roleScope.args...)
	}
	if filters["opponent_champion_id"] != "" {
		rawFilters += " AND pm.opponent_champion_id = ?"
		args = append(args, filters["opponent_champion_id"])
	}
	if filters["patch"] != "" {
		rawFilters += " AND pm.patch = ?"
		args = append(args, filters["patch"])
	}
	query := fmt.Sprintf(`
		WITH raw_opening_item_events AS
		(
			SELECT
				pm.match_id AS match_id,
				pm.participant_id AS participant_id,
				pm.champion_id AS champion_id,
				pm.role AS role,
				pm.opponent_champion_id AS opponent_champion_id,
				pm.patch AS patch_value,
				multiIf(
					s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
					pm.rank_bucket
				) AS rank_value,
				pm.win AS win,
				tie.timestamp_ms AS timestamp_ms,
				tie.item_id AS item_id,
				%s AS item_cost
			FROM participant_matchups AS pm FINAL
			INNER JOIN timeline_item_events AS tie FINAL
				ON pm.match_id = tie.match_id
				AND pm.participant_id = tie.participant_id
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
			WHERE tie.event_type = 'ITEM_PURCHASED'
				AND tie.timestamp_ms <= ?
				AND tie.item_id IN (%s)
				%s
		),
		first_opening_purchases AS
		(
			SELECT
				match_id,
				participant_id,
				min(timestamp_ms) AS first_purchase_ms
			FROM raw_opening_item_events
			GROUP BY
				match_id,
				participant_id
		),
		opening_purchases AS
		(
			SELECT
				e.match_id AS match_id,
				e.participant_id AS participant_id,
				e.champion_id AS champion_id,
				e.role AS role,
				e.opponent_champion_id AS opponent_champion_id,
				e.patch_value AS patch_value,
				e.rank_value AS rank_value,
				e.win AS win,
				arraySort(groupArray(toUInt32(e.item_id))) AS item_ids,
				sum(toUInt32(e.item_cost)) AS item_gold
			FROM raw_opening_item_events AS e
			INNER JOIN first_opening_purchases AS f
				ON e.match_id = f.match_id
				AND e.participant_id = f.participant_id
			WHERE e.timestamp_ms <= f.first_purchase_ms + ?
			GROUP BY
				e.match_id,
				e.participant_id,
				e.champion_id,
				e.role,
				e.opponent_champion_id,
				e.patch_value,
				e.rank_value,
				e.win
			HAVING length(item_ids) > 0 AND item_gold <= ?
		)
		SELECT
			champion_id,
			%s AS role_bucket,
			%s AS opponent_champion_id,
			%s AS patch_bucket,
			%s AS rank_bucket,
			arrayStringConcat(arrayMap(item -> toString(item), item_ids), '-') AS item_signature,
			toUInt64(sum(win)) AS wins,
			toUInt64(count()) AS games,
			wins / games AS win_rate
		FROM opening_purchases
		WHERE length(item_ids) > 0`, itemCostExpr, itemList, rawFilters, roleScope.selectExpr, opponentBucketExpr, patchBucketExpr, rankBucketExpr)
	queryArgs := append([]any{openingPurchaseFirstWindowMS}, args...)
	queryArgs = append(queryArgs, openingPurchaseBurstWindowMS, openingPurchaseGoldCap)
	if filters["rank_bucket"] != "" {
		query += " AND rank_value = ?"
		queryArgs = append(queryArgs, filters["rank_bucket"])
	}
	query += `
		GROUP BY champion_id, role_bucket, opponent_champion_id, patch_bucket, rank_bucket, item_signature
		HAVING games >= ?
		ORDER BY win_rate DESC, games DESC`
	queryArgs = append(queryArgs, minGames)
	return r.scanStartingItemLoadoutRows(ctx, query, queryArgs, limit)
}

func (r *Repository) queryStartingItemLoadoutsSummary(ctx context.Context, filters map[string]string, itemContext string, minGames, limit int) ([]StartingItemLoadoutRow, error) {
	patchBucketExpr := "'ALL'"
	rankBucketExpr := "'ALL'"
	opponentBucketExpr := analyticsOpponentBucketExpr(filters)
	if filters["patch"] != "" {
		patchBucketExpr = "patch"
	}
	if filters["rank_bucket"] != "" {
		rankBucketExpr = "rank_bucket"
	}
	roleScope := analyticsRoleScope(filters["role"])
	query := fmt.Sprintf(`
		SELECT
			champion_id,
			%s AS role_bucket,
			%s AS opponent_champion_id,
			%s AS patch_bucket,
			%s AS rank_bucket,
			item_signature,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games,
			wins / games AS win_rate
		FROM starting_loadout_analytics FINAL
		WHERE item_context = ?
			AND item_signature != ''`, roleScope.selectExpr, opponentBucketExpr, patchBucketExpr, rankBucketExpr)
	args := []any{itemContext}
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
		GROUP BY champion_id, role_bucket, opponent_champion_id, patch_bucket, rank_bucket, item_signature
		HAVING games >= ?
		ORDER BY win_rate DESC, games DESC`
	args = append(args, minGames)
	return r.scanStartingItemLoadoutRows(ctx, query, args, limit)
}

func (r *Repository) StartingLoadoutAnalyticsHasData(ctx context.Context, itemContext, patch string) (bool, error) {
	query := "SELECT count() FROM starting_loadout_analytics WHERE item_context = ?"
	args := []any{normalizedItemContext(itemContext)}
	if strings.TrimSpace(patch) != "" {
		query += " AND patch = ?"
		args = append(args, patch)
	}
	query += " LIMIT 1"
	var count uint64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count > 0, err
}

func (r *Repository) queryItemSlotsSummary(ctx context.Context, filters map[string]string, itemContext string, minGames, limit int) ([]ItemSlotRow, error) {
	patchBucketExpr := "'ALL'"
	rankBucketExpr := "'ALL'"
	opponentBucketExpr := analyticsOpponentBucketExpr(filters)
	if filters["patch"] != "" {
		patchBucketExpr = "patch"
	}
	if filters["rank_bucket"] != "" {
		rankBucketExpr = "rank_bucket"
	}
	roleScope := analyticsRoleScope(filters["role"])
	query := fmt.Sprintf(`
		SELECT
			champion_id,
			%s AS role_bucket,
			%s AS opponent_champion_id,
			%s AS patch_bucket,
			%s AS rank_bucket,
			item_slot,
			item_id,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games,
			wins / games AS win_rate
		FROM item_slot_analytics FINAL
		WHERE item_context = ?
			AND item_id > 0`, roleScope.selectExpr, opponentBucketExpr, patchBucketExpr, rankBucketExpr)
	args := []any{itemContext}
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
		GROUP BY champion_id, role_bucket, opponent_champion_id, patch_bucket, rank_bucket, item_slot, item_id
		HAVING games >= ?
		ORDER BY item_slot ASC, win_rate DESC, games DESC`
	args = append(args, minGames)
	return r.scanItemSlotRows(ctx, query, args, limit)
}

func (r *Repository) scanStartingItemLoadoutRows(ctx context.Context, query string, args []any, limit int) ([]StartingItemLoadoutRow, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StartingItemLoadoutRow
	for rows.Next() {
		var row StartingItemLoadoutRow
		if err := rows.Scan(&row.ChampionID, &row.Role, &row.OpponentChampionID, &row.PatchBucket, &row.RankBucket, &row.ItemSignature, &row.Wins, &row.Games, &row.WinRate); err != nil {
			return nil, err
		}
		row.Confidence = analytics.WilsonLowerBound(row.Wins, row.Games, 1.96)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return trimStartingItemLoadoutRows(out, limit), nil
}

func (r *Repository) queryItemSlotsLiveScan(ctx context.Context, filters map[string]string, allowedItemIDs, startingItemIDs []uint32, minGames, limit int) ([]ItemSlotRow, error) {
	if len(allowedItemIDs) == 0 && len(startingItemIDs) == 0 {
		return nil, nil
	}
	itemList := uint32ListSQL(allowedItemIDs)
	startingItemList := uint32ListSQL(startingItemIDs)
	patchBucketExpr := "'ALL'"
	rankBucketExpr := "'ALL'"
	opponentBucketExpr := analyticsOpponentBucketExpr(filters)
	if filters["patch"] != "" {
		patchBucketExpr = "patch_value"
	}
	if filters["rank_bucket"] != "" {
		rankBucketExpr = "rank_value"
	}
	roleScope := analyticsRoleScope(filters["role"])
	rawFilters := ""
	compiledFilters := ""
	args := []any{}
	if filters["champion_id"] != "" {
		rawFilters += " AND pm.champion_id = ?"
		compiledFilters += " AND champion_id = ?"
		args = append(args, filters["champion_id"])
	}
	if roleScope.whereSQL != "" {
		rawFilters += " AND " + qualifyRoleWhereSQL(roleScope.whereSQL, "pm.role")
		compiledFilters += " AND " + roleScope.whereSQL
		args = append(args, roleScope.args...)
	}
	if filters["opponent_champion_id"] != "" {
		rawFilters += " AND pm.opponent_champion_id = ?"
		compiledFilters += " AND opponent_champion_id = ?"
		args = append(args, filters["opponent_champion_id"])
	}
	if filters["patch"] != "" {
		rawFilters += " AND pm.patch = ?"
		compiledFilters += " AND patch = ?"
		args = append(args, filters["patch"])
	}
	rawArgs := append([]any{}, args...)
	compiledArgs := append([]any{}, args...)
	query := fmt.Sprintf(`
		WITH raw_starting_items AS
		(
			SELECT
				pm.match_id AS match_id,
				pm.participant_id AS participant_id,
				pm.champion_id AS champion_id,
				pm.role AS role,
				pm.opponent_champion_id AS opponent_champion_id,
				pm.patch AS patch_value,
				multiIf(
					s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
					pm.rank_bucket
				) AS rank_value,
				pm.win AS win,
				tie.item_id AS item_id
			FROM participant_matchups AS pm FINAL
			INNER JOIN timeline_item_events AS tie FINAL
				ON pm.match_id = tie.match_id
				AND pm.participant_id = tie.participant_id
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
			WHERE tie.event_type = 'ITEM_PURCHASED'
				AND tie.timestamp_ms <= ?
				AND tie.item_id IN (%s)
				%s
			GROUP BY
				pm.match_id,
				pm.participant_id,
				pm.champion_id,
				pm.role,
				pm.opponent_champion_id,
				pm.patch,
				rank_value,
				pm.win,
				tie.item_id
		),
		raw_item_purchases AS
		(
			SELECT
				pm.match_id AS match_id,
				pm.participant_id AS participant_id,
				pm.champion_id AS champion_id,
				pm.role AS role,
				pm.opponent_champion_id AS opponent_champion_id,
				pm.patch AS patch_value,
				multiIf(
					s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
					pm.rank_bucket
				) AS rank_value,
				pm.win AS win,
				tie.item_id AS item_id,
				min(tie.timestamp_ms) AS first_purchase_ms
			FROM participant_matchups AS pm FINAL
			INNER JOIN timeline_item_events AS tie FINAL
				ON pm.match_id = tie.match_id
				AND pm.participant_id = tie.participant_id
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
			WHERE tie.event_type = 'ITEM_PURCHASED'
				AND tie.item_id IN (%s)
				%s
			GROUP BY
				pm.match_id,
				pm.participant_id,
				pm.champion_id,
				pm.role,
				pm.opponent_champion_id,
				pm.patch,
				rank_value,
				pm.win,
				tie.item_id
		),
		raw_item_slots AS
		(
			SELECT
				champion_id,
				role,
				opponent_champion_id,
				patch_value,
				rank_value,
				toUInt8(row_number() OVER (PARTITION BY match_id, participant_id ORDER BY first_purchase_ms, item_id)) AS item_slot,
				item_id,
				toUInt64(win) AS wins,
				toUInt64(1) AS games
			FROM raw_item_purchases
		),
		compiled_builds AS
		(
			SELECT
				champion_id,
				role,
				opponent_champion_id,
				patch AS patch_value,
				rank_bucket AS rank_value,
				arrayFilter(item -> toUInt32OrZero(item) IN (%s), splitByChar('-', final_items_signature)) AS items,
				wins,
				games
			FROM patch_build_metrics FINAL
			WHERE 1 = 1
				%s
		),
		compiled_item_slots AS
		(
			SELECT
				champion_id,
				role,
				opponent_champion_id,
				patch_value,
				rank_value,
				toUInt8(tupleElement(item_tuple, 1)) AS item_slot,
				toUInt32OrZero(tupleElement(item_tuple, 2)) AS item_id,
				wins,
				games
			FROM compiled_builds
			ARRAY JOIN arrayZip(arrayEnumerate(items), items) AS item_tuple
		)
		SELECT
			champion_id,
			%s AS role_bucket,
			%s AS opponent_champion_id,
			%s AS patch_bucket,
			%s AS rank_bucket,
			item_slot,
			item_id,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games,
			wins / games AS win_rate
		FROM
		(
			SELECT
				champion_id,
				role,
				opponent_champion_id,
				patch_value,
				rank_value,
				toUInt8(0) AS item_slot,
				item_id,
				toUInt64(win) AS wins,
				toUInt64(1) AS games
			FROM raw_starting_items
			UNION ALL
			SELECT * FROM raw_item_slots WHERE item_slot <= 6
			UNION ALL
			SELECT * FROM compiled_item_slots WHERE item_slot <= 6
		)
		WHERE item_id > 0`, startingItemList, rawFilters, itemList, rawFilters, itemList, compiledFilters, roleScope.selectExpr, opponentBucketExpr, patchBucketExpr, rankBucketExpr)
	args = append([]any{startingItemWindowMS}, rawArgs...)
	args = append(args, rawArgs...)
	args = append(args, compiledArgs...)
	if filters["rank_bucket"] != "" {
		query += " AND rank_value = ?"
		args = append(args, filters["rank_bucket"])
	}
	query += `
		GROUP BY champion_id, role_bucket, opponent_champion_id, patch_bucket, rank_bucket, item_slot, item_id
		HAVING games >= ?
		ORDER BY item_slot ASC, win_rate DESC, games DESC`
	args = append(args, minGames)
	return r.scanItemSlotRows(ctx, query, args, limit)
}

func (r *Repository) scanItemSlotRows(ctx context.Context, query string, args []any, limit int) ([]ItemSlotRow, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ItemSlotRow
	for rows.Next() {
		var row ItemSlotRow
		if err := rows.Scan(&row.ChampionID, &row.Role, &row.OpponentChampionID, &row.PatchBucket, &row.RankBucket, &row.ItemSlot, &row.ItemID, &row.Wins, &row.Games, &row.WinRate); err != nil {
			return nil, err
		}
		row.Confidence = analytics.WilsonLowerBound(row.Wins, row.Games, 1.96)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return trimItemSlotRows(out, limit), nil
}

func (r *Repository) ItemSlotAnalyticsHasData(ctx context.Context, itemContext string) (bool, error) {
	var count uint64
	err := r.db.QueryRowContext(ctx, "SELECT count() FROM item_slot_analytics WHERE item_context = ? LIMIT 1", normalizedItemContext(itemContext)).Scan(&count)
	return count > 0, err
}

func (r *Repository) RefreshItemSlotAnalytics(ctx context.Context, patch string, queueID uint16, contexts []ItemSlotAnalyticsContext) error {
	patch = strings.TrimSpace(patch)
	if patch == "" {
		return fmt.Errorf("patch is required")
	}
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	platforms, err := r.rawMatchPlatforms(ctx, patch, queueID)
	if err != nil {
		return err
	}
	for _, itemContext := range contexts {
		key := normalizedItemContext(itemContext.Key)
		completionItemIDs := removeItemIDs(itemContext.ItemIDs, itemContext.StartingItemIDs)
		if len(completionItemIDs) == 0 && len(itemContext.StartingItemIDs) == 0 {
			continue
		}
		compiledAt := time.Now().UTC().Truncate(time.Second)
		itemList := uint32ListSQL(completionItemIDs)
		startingItemList := uint32ListSQL(itemContext.StartingItemIDs)
		for _, platform := range platforms {
			_, err := r.db.ExecContext(
				ctx,
				fmt.Sprintf(`
				INSERT INTO item_slot_analytics
				(patch, platform, queue_id, item_context, champion_id, role, opponent_champion_id, rank_bucket, item_slot, item_id, wins, games, compiled_at)
				WITH target_match_ids AS
				(
					SELECT DISTINCT match_id
					FROM raw_matches
					PREWHERE patch = ? AND queue_id = ?
					WHERE platform = ?
				),
				raw_starting_items AS
				(
					SELECT
						pm.patch AS patch,
						? AS platform,
						pm.queue_id AS queue_id,
						? AS item_context,
						pm.champion_id AS champion_id,
						pm.role AS role,
						pm.opponent_champion_id AS opponent_champion_id,
						multiIf(
							s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
							pm.rank_bucket
						) AS rank_bucket,
						toUInt8(0) AS item_slot,
						tie.item_id AS item_id,
						pm.win AS win
					FROM participant_matchups AS pm FINAL
					INNER JOIN timeline_item_events AS tie FINAL
						ON pm.match_id = tie.match_id
						AND pm.participant_id = tie.participant_id
					LEFT JOIN
					(
						SELECT
							platform,
							puuid,
							argMax(rank_bucket, fetched_at) AS snapshot_rank_bucket
						FROM summoner_rank_snapshots FINAL
						WHERE queue_type = 'RANKED_SOLO_5x5'
							AND platform = ?
						GROUP BY platform, puuid
					) AS s
						ON s.platform = pm.platform AND s.puuid = pm.puuid
					WHERE pm.patch = ?
						AND pm.queue_id = ?
						AND pm.platform = ?
						AND tie.patch = ?
						AND tie.queue_id = ?
						AND tie.platform = ?
						AND pm.match_id IN (SELECT match_id FROM target_match_ids)
						AND tie.match_id IN (SELECT match_id FROM target_match_ids)
						AND tie.event_type = 'ITEM_PURCHASED'
						AND tie.timestamp_ms <= ?
						AND tie.item_id IN (%s)
					GROUP BY
						pm.patch,
						pm.queue_id,
						pm.champion_id,
						pm.role,
						pm.opponent_champion_id,
						rank_bucket,
						pm.win,
						tie.item_id
				),
				raw_item_purchases AS
				(
					SELECT
						pm.match_id AS match_id,
						pm.participant_id AS participant_id,
						pm.patch AS patch,
						pm.queue_id AS queue_id,
						pm.champion_id AS champion_id,
						pm.role AS role,
						pm.opponent_champion_id AS opponent_champion_id,
						multiIf(
							s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
							pm.rank_bucket
						) AS rank_bucket,
						pm.win AS win,
						tie.item_id AS item_id,
						min(tie.timestamp_ms) AS first_purchase_ms
					FROM participant_matchups AS pm FINAL
					INNER JOIN timeline_item_events AS tie FINAL
						ON pm.match_id = tie.match_id
						AND pm.participant_id = tie.participant_id
					LEFT JOIN
					(
						SELECT
							platform,
							puuid,
							argMax(rank_bucket, fetched_at) AS snapshot_rank_bucket
						FROM summoner_rank_snapshots FINAL
						WHERE queue_type = 'RANKED_SOLO_5x5'
							AND platform = ?
						GROUP BY platform, puuid
					) AS s
						ON s.platform = pm.platform AND s.puuid = pm.puuid
					WHERE pm.patch = ?
						AND pm.queue_id = ?
						AND pm.platform = ?
						AND tie.patch = ?
						AND tie.queue_id = ?
						AND tie.platform = ?
						AND pm.match_id IN (SELECT match_id FROM target_match_ids)
						AND tie.match_id IN (SELECT match_id FROM target_match_ids)
						AND tie.event_type = 'ITEM_PURCHASED'
						AND tie.item_id IN (%s)
					GROUP BY
						pm.match_id,
						pm.participant_id,
						pm.patch,
						pm.queue_id,
						pm.champion_id,
						pm.role,
						pm.opponent_champion_id,
						rank_bucket,
						pm.win,
						tie.item_id
				),
				raw_item_slots AS
				(
					SELECT
						patch,
						? AS platform,
						queue_id,
						? AS item_context,
						champion_id,
						role,
						opponent_champion_id,
						rank_bucket,
						toUInt8(row_number() OVER (PARTITION BY match_id, participant_id ORDER BY first_purchase_ms, item_id)) AS item_slot,
						item_id,
						win
					FROM raw_item_purchases
				)
				SELECT
					patch,
					platform,
					queue_id,
					item_context,
					champion_id,
					role,
					opponent_champion_id,
					rank_bucket,
					item_slot,
					item_id,
					toUInt64(sum(win)) AS wins,
					toUInt64(count()) AS games,
					? AS compiled_at
				FROM raw_item_slots
				WHERE item_slot <= 6
				GROUP BY
					patch,
					platform,
					queue_id,
					item_context,
					champion_id,
					role,
					opponent_champion_id,
					rank_bucket,
					item_slot,
					item_id
				UNION ALL
				SELECT
					patch,
					platform,
					queue_id,
					item_context,
					champion_id,
					role,
					opponent_champion_id,
					rank_bucket,
					item_slot,
					item_id,
					toUInt64(sum(win)) AS wins,
					toUInt64(count()) AS games,
					? AS compiled_at
				FROM raw_starting_items
				GROUP BY
					patch,
					platform,
					queue_id,
					item_context,
					champion_id,
					role,
					opponent_champion_id,
					rank_bucket,
					item_slot,
					item_id
				SETTINGS join_algorithm = 'grace_hash'`, startingItemList, itemList),
				patch,
				queueID,
				platform,
				platform,
				key,
				platform,
				patch,
				queueID,
				platform,
				patch,
				queueID,
				platform,
				startingItemWindowMS,
				platform,
				patch,
				queueID,
				platform,
				patch,
				queueID,
				platform,
				platform,
				key,
				compiledAt,
				compiledAt,
			)
			if err != nil {
				return fmt.Errorf("item slot analytics platform %s context %s: %w", platform, key, err)
			}
			log.Printf("item slot analytics progress patch=%s context=%s platform=%s", patch, key, platform)
		}
		if err := r.aggregateItemSlotAnalyticsPlatforms(ctx, patch, queueID, key, compiledAt, platforms); err != nil {
			return err
		}
		if err := r.cleanupOldItemSlotAnalytics(ctx, patch, queueID, key, compiledAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) RefreshStartingLoadoutAnalytics(ctx context.Context, patch string, queueID uint16, contexts []StartingLoadoutAnalyticsContext) error {
	patch = strings.TrimSpace(patch)
	if patch == "" {
		return fmt.Errorf("patch is required")
	}
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	platforms, err := r.rawMatchPlatforms(ctx, patch, queueID)
	if err != nil {
		return err
	}
	for _, context := range contexts {
		key := normalizedItemContext(context.Key)
		if len(context.OpeningItemCosts) == 0 {
			continue
		}
		compiledAt := time.Now().UTC().Truncate(time.Second)
		openingItemIDs := uint32MapKeysSorted(context.OpeningItemCosts)
		itemList := uint32ListSQL(openingItemIDs)
		itemCostExpr := itemCostExpressionSQL("tie.item_id", context.OpeningItemCosts)
		for _, platform := range platforms {
			_, err := r.db.ExecContext(
				ctx,
				fmt.Sprintf(`
				INSERT INTO starting_loadout_analytics
				(patch, platform, queue_id, item_context, champion_id, role, opponent_champion_id, rank_bucket, item_signature, wins, games, compiled_at)
				WITH target_match_ids AS
				(
					SELECT DISTINCT match_id
					FROM raw_matches
					PREWHERE patch = ? AND queue_id = ?
					WHERE platform = ?
				),
				raw_opening_item_events AS
				(
					SELECT
						pm.match_id AS match_id,
						pm.participant_id AS participant_id,
						pm.patch AS patch,
						pm.queue_id AS queue_id,
						pm.champion_id AS champion_id,
						pm.role AS role,
						pm.opponent_champion_id AS opponent_champion_id,
						multiIf(
							s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
							pm.rank_bucket
						) AS rank_bucket,
						pm.win AS win,
						tie.timestamp_ms AS timestamp_ms,
						tie.item_id AS item_id,
						%s AS item_cost
					FROM participant_matchups AS pm FINAL
					INNER JOIN timeline_item_events AS tie FINAL
						ON pm.match_id = tie.match_id
						AND pm.participant_id = tie.participant_id
					LEFT JOIN
					(
						SELECT
							platform,
							puuid,
							argMax(rank_bucket, fetched_at) AS snapshot_rank_bucket
						FROM summoner_rank_snapshots FINAL
						WHERE queue_type = 'RANKED_SOLO_5x5'
							AND platform = ?
						GROUP BY platform, puuid
					) AS s
						ON s.platform = pm.platform AND s.puuid = pm.puuid
					WHERE pm.patch = ?
						AND pm.queue_id = ?
						AND pm.platform = ?
						AND tie.patch = ?
						AND tie.queue_id = ?
						AND tie.platform = ?
						AND pm.match_id IN (SELECT match_id FROM target_match_ids)
						AND tie.match_id IN (SELECT match_id FROM target_match_ids)
						AND tie.event_type = 'ITEM_PURCHASED'
						AND tie.timestamp_ms <= ?
						AND tie.item_id IN (%s)
				),
				first_opening_purchases AS
				(
					SELECT
						match_id,
						participant_id,
						min(timestamp_ms) AS first_purchase_ms
					FROM raw_opening_item_events
					GROUP BY match_id, participant_id
				),
				opening_purchases AS
				(
					SELECT
						e.match_id AS match_id,
						e.participant_id AS participant_id,
						e.patch AS patch,
						e.queue_id AS queue_id,
						e.champion_id AS champion_id,
						e.role AS role,
						e.opponent_champion_id AS opponent_champion_id,
						e.rank_bucket AS rank_bucket,
						e.win AS win,
						arraySort(groupArray(toUInt32(e.item_id))) AS item_ids,
						sum(toUInt32(e.item_cost)) AS item_gold
					FROM raw_opening_item_events AS e
					INNER JOIN first_opening_purchases AS f
						ON e.match_id = f.match_id
						AND e.participant_id = f.participant_id
					WHERE e.timestamp_ms <= f.first_purchase_ms + ?
					GROUP BY
						e.match_id,
						e.participant_id,
						e.patch,
						e.queue_id,
						e.champion_id,
						e.role,
						e.opponent_champion_id,
						e.rank_bucket,
						e.win
					HAVING length(item_ids) > 0 AND item_gold <= ?
				)
				SELECT
					patch,
					? AS platform,
					queue_id,
					? AS item_context,
					champion_id,
					role,
					opponent_champion_id,
					rank_bucket,
					arrayStringConcat(arrayMap(item -> toString(item), item_ids), '-') AS item_signature,
					toUInt64(sum(win)) AS wins,
					toUInt64(count()) AS games,
					? AS compiled_at
				FROM opening_purchases
				GROUP BY
					patch,
					queue_id,
					champion_id,
					role,
					opponent_champion_id,
					rank_bucket,
					item_signature
				SETTINGS join_algorithm = 'grace_hash'`, itemCostExpr, itemList),
				patch,
				queueID,
				platform,
				platform,
				patch,
				queueID,
				platform,
				patch,
				queueID,
				platform,
				openingPurchaseFirstWindowMS,
				openingPurchaseBurstWindowMS,
				openingPurchaseGoldCap,
				platform,
				key,
				compiledAt,
			)
			if err != nil {
				return fmt.Errorf("starting loadout analytics platform %s context %s: %w", platform, key, err)
			}
			log.Printf("starting loadout analytics progress patch=%s context=%s platform=%s", patch, key, platform)
		}
		if err := r.aggregateStartingLoadoutAnalyticsPlatforms(ctx, patch, queueID, key, compiledAt, platforms); err != nil {
			return err
		}
		if err := r.cleanupOldStartingLoadoutAnalytics(ctx, patch, queueID, key, compiledAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) aggregateStartingLoadoutAnalyticsPlatforms(
	ctx context.Context,
	patch string,
	queueID uint16,
	itemContext string,
	compiledAt time.Time,
	platforms []string,
) error {
	if len(platforms) == 0 {
		return nil
	}
	platformFilter, platformArgs := rawPayloadMatchFilter(platforms)
	args := make([]any, 0, len(platformArgs)+5)
	args = append(args, compiledAt, patch, queueID, normalizedItemContext(itemContext), compiledAt)
	args = append(args, platformArgs...)
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO starting_loadout_analytics
		(patch, platform, queue_id, item_context, champion_id, role, opponent_champion_id, rank_bucket, item_signature, wins, games, compiled_at)
		SELECT
			patch,
			'ALL' AS platform,
			queue_id,
			item_context,
			champion_id,
			role,
			opponent_champion_id,
			rank_bucket,
			item_signature,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games,
			? AS compiled_at
		FROM starting_loadout_analytics
		WHERE patch = ?
			AND queue_id = ?
			AND item_context = ?
			AND compiled_at = ?
			AND platform IN (%s)
		GROUP BY
			patch,
			queue_id,
			item_context,
			champion_id,
			role,
			opponent_champion_id,
			rank_bucket,
			item_signature`, platformFilter), args...)
	if err != nil {
		return fmt.Errorf("aggregate starting loadout analytics context %s: %w", itemContext, err)
	}
	return nil
}

func (r *Repository) aggregateItemSlotAnalyticsPlatforms(
	ctx context.Context,
	patch string,
	queueID uint16,
	itemContext string,
	compiledAt time.Time,
	platforms []string,
) error {
	if len(platforms) == 0 {
		return nil
	}
	platformFilter, platformArgs := rawPayloadMatchFilter(platforms)
	args := make([]any, 0, len(platformArgs)+5)
	args = append(args, compiledAt, patch, queueID, normalizedItemContext(itemContext), compiledAt)
	args = append(args, platformArgs...)
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO item_slot_analytics
		(patch, platform, queue_id, item_context, champion_id, role, opponent_champion_id, rank_bucket, item_slot, item_id, wins, games, compiled_at)
		SELECT
			patch,
			'ALL' AS platform,
			queue_id,
			item_context,
			champion_id,
			role,
			opponent_champion_id,
			rank_bucket,
			item_slot,
			item_id,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games,
			? AS compiled_at
		FROM item_slot_analytics
		WHERE patch = ?
			AND queue_id = ?
			AND item_context = ?
			AND compiled_at = ?
			AND platform IN (%s)
		GROUP BY
			patch,
			queue_id,
			item_context,
			champion_id,
			role,
			opponent_champion_id,
			rank_bucket,
			item_slot,
			item_id`, platformFilter), args...)
	if err != nil {
		return fmt.Errorf("aggregate item slot analytics context %s: %w", itemContext, err)
	}
	return nil
}

func (r *Repository) cleanupOldItemSlotAnalytics(ctx context.Context, patch string, queueID uint16, itemContext string, compiledAt time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`ALTER TABLE item_slot_analytics DELETE WHERE patch = ? AND queue_id = ? AND item_context = ? AND compiled_at < ? SETTINGS mutations_sync = 2`,
		patch,
		queueID,
		normalizedItemContext(itemContext),
		compiledAt,
	)
	return err
}

func (r *Repository) cleanupOldStartingLoadoutAnalytics(ctx context.Context, patch string, queueID uint16, itemContext string, compiledAt time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`ALTER TABLE starting_loadout_analytics DELETE WHERE patch = ? AND queue_id = ? AND item_context = ? AND compiled_at < ? SETTINGS mutations_sync = 2`,
		patch,
		queueID,
		normalizedItemContext(itemContext),
		compiledAt,
	)
	return err
}

func uint32ListSQL(values []uint32) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		parts = append(parts, strconv.FormatUint(uint64(value), 10))
	}
	if len(parts) == 0 {
		return "0"
	}
	return strings.Join(parts, ",")
}

func uint32MapKeysSorted(values map[uint32]uint32) []uint32 {
	keys := make([]uint32, 0, len(values))
	for key := range values {
		if key != 0 {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}

func itemCostExpressionSQL(column string, costs map[uint32]uint32) string {
	keys := uint32MapKeysSorted(costs)
	parts := make([]string, 0, len(keys)*2+1)
	for _, key := range keys {
		parts = append(parts,
			fmt.Sprintf("%s = %d", column, key),
			fmt.Sprintf("toUInt32(%d)", costs[key]),
		)
	}
	parts = append(parts, "toUInt32(0)")
	return fmt.Sprintf("multiIf(%s)", strings.Join(parts, ", "))
}

func removeItemIDs(values, removed []uint32) []uint32 {
	if len(removed) == 0 {
		return values
	}
	removedSet := make(map[uint32]bool, len(removed))
	for _, value := range removed {
		removedSet[value] = true
	}
	out := make([]uint32, 0, len(values))
	for _, value := range values {
		if value != 0 && !removedSet[value] {
			out = append(out, value)
		}
	}
	return out
}

func normalizedItemContext(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "JUNGLE":
		return "JUNGLE"
	case "SUPPORT", "UTILITY":
		return "SUPPORT"
	default:
		return "DEFAULT"
	}
}

func qualifyRoleWhereSQL(whereSQL, qualifiedColumn string) string {
	return strings.ReplaceAll(whereSQL, "role", qualifiedColumn)
}

func trimItemSlotRows(rows []ItemSlotRow, limit int) []ItemSlotRow {
	if limit <= 0 || len(rows) == 0 {
		return rows
	}
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]
		if left.ItemSlot != right.ItemSlot {
			return left.ItemSlot < right.ItemSlot
		}
		leftScore := itemSlotRecommendationScore(left)
		rightScore := itemSlotRecommendationScore(right)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if left.Confidence != right.Confidence {
			return left.Confidence > right.Confidence
		}
		if left.WinRate != right.WinRate {
			return left.WinRate > right.WinRate
		}
		if left.Games != right.Games {
			return left.Games > right.Games
		}
		return left.ItemID < right.ItemID
	})
	countsBySlot := map[uint8]int{}
	out := make([]ItemSlotRow, 0, len(rows))
	for _, row := range rows {
		if countsBySlot[row.ItemSlot] >= limit {
			continue
		}
		countsBySlot[row.ItemSlot]++
		out = append(out, row)
	}
	return out
}

func trimStartingItemLoadoutRows(rows []StartingItemLoadoutRow, limit int) []StartingItemLoadoutRow {
	if limit <= 0 || len(rows) == 0 {
		return rows
	}
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]
		leftScore := startingLoadoutRecommendationScore(left)
		rightScore := startingLoadoutRecommendationScore(right)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if left.Confidence != right.Confidence {
			return left.Confidence > right.Confidence
		}
		if left.WinRate != right.WinRate {
			return left.WinRate > right.WinRate
		}
		if left.Games != right.Games {
			return left.Games > right.Games
		}
		return left.ItemSignature < right.ItemSignature
	})
	if len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func itemSlotRecommendationScore(row ItemSlotRow) float64 {
	reliability := math.Sqrt(float64(row.Games) / 200)
	if reliability > 1 {
		reliability = 1
	}
	return row.Confidence * reliability
}

func startingLoadoutRecommendationScore(row StartingItemLoadoutRow) float64 {
	reliability := math.Sqrt(float64(row.Games) / 150)
	if reliability > 1 {
		reliability = 1
	}
	return row.Confidence * reliability
}
