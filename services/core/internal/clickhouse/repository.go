package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"

	"winrift/services/core/internal/analytics"
	"winrift/services/core/internal/config"
)

type Repository struct {
	db *sql.DB
}

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

type PatchSnapshot struct {
	Patch            string
	Platform         string
	QueueID          uint16
	Status           string
	Matches          uint64
	Participants     uint64
	RawRetainedUntil time.Time
}

type roleAnalyticsScope struct {
	selectExpr string
	whereSQL   string
	args       []any
}

// analyticsRoleScope intentionally merges solo lanes for build advice, where
// matchup-specific items usually transfer between top and mid.
func analyticsRoleScope(role string) roleAnalyticsScope {
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case "TOP":
		return roleAnalyticsScope{selectExpr: "'TOP'", whereSQL: "role IN ('TOP', 'MIDDLE')"}
	case "MIDDLE":
		return roleAnalyticsScope{selectExpr: "'MIDDLE'", whereSQL: "role IN ('TOP', 'MIDDLE')"}
	case "JUNGLE":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"JUNGLE"}}
	case "BOTTOM":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"BOTTOM"}}
	case "UTILITY":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"UTILITY"}}
	default:
		return roleAnalyticsScope{selectExpr: "'ALL'"}
	}
}

func analyticsOpponentBucketExpr(filters map[string]string) string {
	if strings.TrimSpace(filters["opponent_champion_id"]) != "" {
		return "opponent_champion_id"
	}
	return "toUInt16(0)"
}

// strictAnalyticsRoleScope keeps champion strength and guide rankings in their
// selected role so mid-lane picks do not leak into top-lane tier lists.
func strictAnalyticsRoleScope(role string) roleAnalyticsScope {
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case "TOP":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"TOP"}}
	case "MIDDLE":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"MIDDLE"}}
	case "JUNGLE":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"JUNGLE"}}
	case "BOTTOM":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"BOTTOM"}}
	case "UTILITY":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"UTILITY"}}
	default:
		return roleAnalyticsScope{selectExpr: "'ALL'"}
	}
}

func NewRepository(cfg config.Config) (*Repository, error) {
	dsn := fmt.Sprintf("clickhouse://%s:%d/%s?username=%s&password=%s", cfg.ClickHouseHost, cfg.ClickHousePort, cfg.ClickHouseDatabase, cfg.ClickHouseUser, cfg.ClickHousePassword)
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	repo := &Repository{db: db}
	if err := repo.EnsureRuntimeSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repo, nil
}

func (r *Repository) MatchExists(ctx context.Context, matchID string) (bool, error) {
	var count uint64
	err := r.db.QueryRowContext(ctx, "SELECT count() FROM raw_matches FINAL WHERE match_id = ?", matchID).Scan(&count)
	return count > 0, err
}

func (r *Repository) InsertNormalized(ctx context.Context, normalized analytics.NormalizedMatch) error {
	if err := r.insertRawMatch(ctx, normalized.RawMatch); err != nil {
		return err
	}
	if err := r.insertRawTimeline(ctx, normalized.RawTimeline); err != nil {
		return err
	}
	for _, row := range normalized.Participants {
		if err := r.insertParticipant(ctx, row); err != nil {
			return err
		}
		if err := r.insertParticipantPerformance(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.Matchups {
		if err := r.insertMatchup(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.TimelineParticipantFrames {
		if err := r.insertTimelineParticipantFrame(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.TimelineItemEvents {
		if err := r.insertTimelineItemEvent(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.TimelineSkillEvents {
		if err := r.insertTimelineSkillEvent(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.TimelineCombatEvents {
		if err := r.insertTimelineCombatEvent(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.TimelineObjectiveEvents {
		if err := r.insertTimelineObjectiveEvent(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.ChampionBans {
		if err := r.insertChampionBan(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) QueryBuilds(ctx context.Context, filters map[string]string, minGames, limit int) ([]BuildRow, error) {
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
	for _, itemContext := range contexts {
		key := normalizedItemContext(itemContext.Key)
		completionItemIDs := removeItemIDs(itemContext.ItemIDs, itemContext.StartingItemIDs)
		if len(completionItemIDs) == 0 && len(itemContext.StartingItemIDs) == 0 {
			continue
		}
		if _, err := r.db.ExecContext(
			ctx,
			`ALTER TABLE item_slot_analytics DELETE WHERE patch = ? AND queue_id = ? AND item_context = ? SETTINGS mutations_sync = 2`,
			patch,
			queueID,
			key,
		); err != nil {
			return err
		}
		itemList := uint32ListSQL(completionItemIDs)
		startingItemList := uint32ListSQL(itemContext.StartingItemIDs)
		_, err := r.db.ExecContext(
			ctx,
			fmt.Sprintf(`
				INSERT INTO item_slot_analytics
				(patch, platform, queue_id, item_context, champion_id, role, opponent_champion_id, rank_bucket, item_slot, item_id, wins, games)
				WITH raw_starting_items AS
				(
					SELECT
						pm.patch AS patch,
						'ALL' AS platform,
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
						GROUP BY platform, puuid
					) AS s
						ON s.platform = pm.platform AND s.puuid = pm.puuid
					WHERE pm.patch = ?
						AND pm.queue_id = ?
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
						GROUP BY platform, puuid
					) AS s
						ON s.platform = pm.platform AND s.puuid = pm.puuid
					WHERE pm.patch = ?
						AND pm.queue_id = ?
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
						'ALL' AS platform,
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
					toUInt64(count()) AS games
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
					toUInt64(count()) AS games
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
					item_id`, startingItemList, itemList),
			key,
			patch,
			queueID,
			startingItemWindowMS,
			patch,
			queueID,
			key,
		)
		if err != nil {
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
	for _, context := range contexts {
		key := normalizedItemContext(context.Key)
		if len(context.OpeningItemCosts) == 0 {
			continue
		}
		if _, err := r.db.ExecContext(
			ctx,
			`ALTER TABLE starting_loadout_analytics DELETE WHERE patch = ? AND queue_id = ? AND item_context = ? SETTINGS mutations_sync = 2`,
			patch,
			queueID,
			key,
		); err != nil {
			return err
		}
		openingItemIDs := uint32MapKeysSorted(context.OpeningItemCosts)
		itemList := uint32ListSQL(openingItemIDs)
		itemCostExpr := itemCostExpressionSQL("tie.item_id", context.OpeningItemCosts)
		_, err := r.db.ExecContext(
			ctx,
			fmt.Sprintf(`
				INSERT INTO starting_loadout_analytics
				(patch, platform, queue_id, item_context, champion_id, role, opponent_champion_id, rank_bucket, item_signature, wins, games)
				WITH raw_opening_item_events AS
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
						GROUP BY platform, puuid
					) AS s
						ON s.platform = pm.platform AND s.puuid = pm.puuid
					WHERE pm.patch = ?
						AND pm.queue_id = ?
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
					'ALL' AS platform,
					queue_id,
					? AS item_context,
					champion_id,
					role,
					opponent_champion_id,
					rank_bucket,
					arrayStringConcat(arrayMap(item -> toString(item), item_ids), '-') AS item_signature,
					toUInt64(sum(win)) AS wins,
					toUInt64(count()) AS games
				FROM opening_purchases
				GROUP BY
					patch,
					queue_id,
					champion_id,
					role,
					opponent_champion_id,
					rank_bucket,
					item_signature`, itemCostExpr, itemList),
			patch,
			queueID,
			openingPurchaseFirstWindowMS,
			openingPurchaseBurstWindowMS,
			openingPurchaseGoldCap,
			key,
		)
		if err != nil {
			return err
		}
	}
	return nil
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

func (r *Repository) insertRawMatch(ctx context.Context, row analytics.RawMatch) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO raw_matches (match_id, platform, queue_id, patch, game_creation, game_start_timestamp, game_end_timestamp, duration_seconds, raw_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, row.MatchID, row.Platform, row.QueueID, row.Patch, row.GameCreation, row.GameStartTimestamp, row.GameEndTimestamp, row.DurationSeconds, row.RawJSON)
	return err
}

func (r *Repository) insertRawTimeline(ctx context.Context, row analytics.RawTimeline) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO raw_timelines (match_id, platform, patch, queue_id, raw_json) VALUES (?, ?, ?, ?, ?)`, row.MatchID, row.Platform, row.Patch, row.QueueID, row.RawJSON)
	return err
}

func (r *Repository) insertParticipant(ctx context.Context, row analytics.ParticipantRow) error {
	_, err := r.db.ExecContext(ctx, participantInsertSQL, participantArgs(row)...)
	return err
}

func (r *Repository) insertParticipantPerformance(ctx context.Context, row analytics.ParticipantRow) error {
	_, err := r.db.ExecContext(ctx, participantPerformanceInsertSQL, participantPerformanceArgs(row)...)
	return err
}

func (r *Repository) insertMatchup(ctx context.Context, row analytics.MatchupRow) error {
	args := participantArgs(row.ParticipantRow)
	args = append(args, row.OpponentParticipantID, row.OpponentChampionID, row.OpponentRole)
	_, err := r.db.ExecContext(ctx, matchupInsertSQL, args...)
	return err
}

func (r *Repository) insertTimelineParticipantFrame(ctx context.Context, row analytics.TimelineParticipantFrameRow) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO timeline_participant_frames (match_id, platform, patch, queue_id, timestamp_ms, participant_id, level, xp, current_gold, total_gold, minions_killed, jungle_minions_killed, position_x, position_y, total_damage_done_to_champions, total_damage_taken) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.ParticipantID, row.Level, row.XP, row.CurrentGold, row.TotalGold, row.MinionsKilled, row.JungleMinionsKilled, row.PositionX, row.PositionY, row.TotalDamageDoneToChampions, row.TotalDamageTaken,
	)
	return err
}

func (r *Repository) insertTimelineItemEvent(ctx context.Context, row analytics.TimelineItemEventRow) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO timeline_item_events (match_id, platform, patch, queue_id, timestamp_ms, participant_id, event_type, item_id, before_id, after_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.ParticipantID, row.EventType, row.ItemID, row.BeforeID, row.AfterID,
	)
	return err
}

func (r *Repository) insertTimelineSkillEvent(ctx context.Context, row analytics.TimelineSkillEventRow) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO timeline_skill_events (match_id, platform, patch, queue_id, timestamp_ms, participant_id, skill_slot, skill_order, level_up_type) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.ParticipantID, row.SkillSlot, row.SkillOrder, row.LevelUpType,
	)
	return err
}

func (r *Repository) insertTimelineCombatEvent(ctx context.Context, row analytics.TimelineCombatEventRow) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO timeline_combat_events (match_id, platform, patch, queue_id, timestamp_ms, killer_id, victim_id, assisting_participant_ids, bounty, shutdown_bounty, position_x, position_y) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.KillerID, row.VictimID, row.AssistingParticipantIDs, row.Bounty, row.ShutdownBounty, row.PositionX, row.PositionY,
	)
	return err
}

func (r *Repository) insertTimelineObjectiveEvent(ctx context.Context, row analytics.TimelineObjectiveEventRow) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO timeline_objective_events (match_id, platform, patch, queue_id, timestamp_ms, event_type, killer_id, team_id, monster_type, monster_sub_type, building_type, tower_type, lane_type, position_x, position_y) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.EventType, row.KillerID, row.TeamID, row.MonsterType, row.MonsterSubType, row.BuildingType, row.TowerType, row.LaneType, row.PositionX, row.PositionY,
	)
	return err
}

func (r *Repository) insertChampionBan(ctx context.Context, row analytics.ChampionBanRow) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO champion_bans (match_id, platform, patch, queue_id, team_id, champion_id, pick_turn) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		row.MatchID, row.Platform, row.Patch, row.QueueID, row.TeamID, row.ChampionID, row.PickTurn,
	)
	return err
}

const participantColumns = `match_id, platform, patch, queue_id, participant_id, puuid, team_id, champion_id, champion_name, role, win, kills, deaths, assists, gold_earned, gold_spent, total_minions_killed, neutral_minions_killed, total_damage_dealt_to_champions, physical_damage_dealt_to_champions, magic_damage_dealt_to_champions, true_damage_dealt_to_champions, total_damage_taken, damage_self_mitigated, damage_dealt_to_objectives, damage_dealt_to_turrets, damage_dealt_to_buildings, vision_score, wards_placed, wards_killed, detector_wards_placed, time_ccing_others, total_heal, total_heals_on_teammates, total_damage_shielded_on_teammates, turret_takedowns, inhibitor_takedowns, dragon_kills, baron_kills, objectives_stolen, total_time_spent_dead, time_played, item0, item1, item2, item3, item4, item5, trinket_item, summoner_spell1, summoner_spell2, primary_rune_tree, secondary_rune_tree, keystone, rune_signature, spell_signature, final_items_signature, core2_signature, core3_signature, rank_bucket`

const participantPlaceholders = `?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?`

const participantInsertSQL = `INSERT INTO participants (` + participantColumns + `) VALUES (` + participantPlaceholders + `)`

const matchupInsertSQL = `INSERT INTO participant_matchups (` + participantColumns + `, opponent_participant_id, opponent_champion_id, opponent_role) VALUES (` + participantPlaceholders + `, ?, ?, ?)`

const participantPerformanceColumns = `match_id, platform, patch, queue_id, participant_id, champion_id, role, gold_earned, gold_spent, total_minions_killed, neutral_minions_killed, total_damage_dealt_to_champions, physical_damage_dealt_to_champions, magic_damage_dealt_to_champions, true_damage_dealt_to_champions, total_damage_taken, damage_self_mitigated, damage_dealt_to_objectives, damage_dealt_to_turrets, damage_dealt_to_buildings, vision_score, wards_placed, wards_killed, detector_wards_placed, time_ccing_others, total_heal, total_heals_on_teammates, total_damage_shielded_on_teammates, turret_takedowns, inhibitor_takedowns, dragon_kills, baron_kills, objectives_stolen, total_time_spent_dead, time_played`

const participantPerformancePlaceholders = `?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?`

const participantPerformanceInsertSQL = `INSERT INTO participant_performance (` + participantPerformanceColumns + `) VALUES (` + participantPerformancePlaceholders + `)`

func participantArgs(row analytics.ParticipantRow) []any {
	return []any{row.MatchID, row.Platform, row.Patch, row.QueueID, row.ParticipantID, row.PUUID, row.TeamID, row.ChampionID, row.ChampionName, row.Role, row.Win, row.Kills, row.Deaths, row.Assists, row.GoldEarned, row.GoldSpent, row.TotalMinionsKilled, row.NeutralMinionsKilled, row.TotalDamageDealtToChampions, row.PhysicalDamageDealtToChampions, row.MagicDamageDealtToChampions, row.TrueDamageDealtToChampions, row.TotalDamageTaken, row.DamageSelfMitigated, row.DamageDealtToObjectives, row.DamageDealtToTurrets, row.DamageDealtToBuildings, row.VisionScore, row.WardsPlaced, row.WardsKilled, row.DetectorWardsPlaced, row.TimeCCingOthers, row.TotalHeal, row.TotalHealsOnTeammates, row.TotalDamageShieldedOnTeammates, row.TurretTakedowns, row.InhibitorTakedowns, row.DragonKills, row.BaronKills, row.ObjectivesStolen, row.TotalTimeSpentDead, row.TimePlayed, row.Item0, row.Item1, row.Item2, row.Item3, row.Item4, row.Item5, row.TrinketItem, row.SummonerSpell1, row.SummonerSpell2, row.PrimaryRuneTree, row.SecondaryRuneTree, row.Keystone, row.RuneSignature, row.SpellSignature, row.FinalItemsSignature, row.Core2Signature, row.Core3Signature, row.RankBucket}
}

func participantPerformanceArgs(row analytics.ParticipantRow) []any {
	return []any{
		row.MatchID,
		row.Platform,
		row.Patch,
		row.QueueID,
		row.ParticipantID,
		row.ChampionID,
		row.Role,
		row.GoldEarned,
		row.GoldSpent,
		row.TotalMinionsKilled,
		row.NeutralMinionsKilled,
		row.TotalDamageDealtToChampions,
		row.PhysicalDamageDealtToChampions,
		row.MagicDamageDealtToChampions,
		row.TrueDamageDealtToChampions,
		row.TotalDamageTaken,
		row.DamageSelfMitigated,
		row.DamageDealtToObjectives,
		row.DamageDealtToTurrets,
		row.DamageDealtToBuildings,
		row.VisionScore,
		row.WardsPlaced,
		row.WardsKilled,
		row.DetectorWardsPlaced,
		row.TimeCCingOthers,
		row.TotalHeal,
		row.TotalHealsOnTeammates,
		row.TotalDamageShieldedOnTeammates,
		row.TurretTakedowns,
		row.InhibitorTakedowns,
		row.DragonKills,
		row.BaronKills,
		row.ObjectivesStolen,
		row.TotalTimeSpentDead,
		row.TimePlayed,
	}
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
