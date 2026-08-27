package clickhouse

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"winrift/core/internal/analytics"
)

func (r *Repository) queryChampionGuideItemPaths(ctx context.Context, filters map[string]string, minGames, limit int) ([]ChampionGuideItemPathRow, error) {
	if minGames <= 0 {
		minGames = 5
	}
	if limit <= 0 {
		limit = 12
	}
	rows, hasSummary, err := r.queryChampionGuideItemPathsSummary(ctx, filters, minGames, limit)
	if err != nil {
		return nil, err
	}
	if hasSummary {
		return rows, nil
	}
	return r.queryChampionGuideItemPathsLiveScan(ctx, filters, minGames, limit)
}

func (r *Repository) queryChampionGuideItemPathsSummary(ctx context.Context, filters map[string]string, minGames, limit int) ([]ChampionGuideItemPathRow, bool, error) {
	championID := filterValue(filters["champion_id"])
	if championID == "" {
		return nil, false, nil
	}
	sourceSQL, hasSummary, err := r.buildSignatureSummarySource(ctx, filterValue(filters["patch"]))
	if err != nil {
		return nil, false, err
	}
	if !hasSummary {
		return nil, false, nil
	}
	roleScope := strictAnalyticsRoleScope(filters["role"])
	query := `
		SELECT
			core3_signature,
			final_items_signature,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games,
			wins / games AS win_rate
		FROM (` + sourceSQL + `)
		WHERE champion_id = ?
			AND core3_signature != ''
			AND final_items_signature != ''
			AND length(splitByChar('-', core3_signature)) >= 3
			AND length(splitByChar('-', final_items_signature)) >= 3`
	args := []any{championID}
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
	query += `
		GROUP BY core3_signature, final_items_signature
		HAVING games >= ?
		ORDER BY games DESC, win_rate DESC
		LIMIT ?`
	args = append(args, minGames, limit*5)
	rows, err := r.scanChampionGuideItemPathRows(ctx, query, args, limit)
	return rows, true, err
}

func (r *Repository) queryChampionGuideItemPathsLiveScan(ctx context.Context, filters map[string]string, minGames, limit int) ([]ChampionGuideItemPathRow, error) {
	roleScope := strictAnalyticsRoleScope(filters["role"])
	rawSQL, rawArgs := championGuideBaseSQLExcludingCompiledBuilds(filters, roleScope, true)
	compiledWhere := `
		FROM patch_build_metrics FINAL
		WHERE champion_id = ?
			AND core3_signature != ''
			AND final_items_signature != ''
			AND length(splitByChar('-', core3_signature)) >= 3
			AND length(splitByChar('-', final_items_signature)) >= 3`
	compiledArgs := []any{filters["champion_id"]}
	if roleScope.whereSQL != "" {
		compiledWhere += " AND " + roleScope.whereSQL
		compiledArgs = append(compiledArgs, roleScope.args...)
	}
	if filterValue(filters["patch"]) != "" {
		compiledWhere += " AND patch = ?"
		compiledArgs = append(compiledArgs, filterValue(filters["patch"]))
	}
	if filterValue(filters["rank_bucket"]) != "" {
		compiledWhere += " AND rank_bucket = ?"
		compiledArgs = append(compiledArgs, filterValue(filters["rank_bucket"]))
	}
	query := `
		SELECT
			core3_signature,
			final_items_signature,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games,
			wins / games AS win_rate
		FROM
		(
			SELECT
				core3_signature,
				final_items_signature,
				toUInt64(sum(win)) AS wins,
				toUInt64(count()) AS games
			` + rawSQL + `
				AND core3_signature != ''
				AND final_items_signature != ''
				AND length(splitByChar('-', core3_signature)) >= 3
				AND length(splitByChar('-', final_items_signature)) >= 3
			GROUP BY core3_signature, final_items_signature
			UNION ALL
			SELECT
				core3_signature,
				final_items_signature,
				toUInt64(sum(wins)) AS wins,
				toUInt64(sum(games)) AS games
			` + compiledWhere + `
			GROUP BY core3_signature, final_items_signature
		)
		GROUP BY core3_signature, final_items_signature
		HAVING games >= ?
		ORDER BY games DESC, win_rate DESC
		LIMIT ?`
	args := append([]any{}, rawArgs...)
	args = append(args, compiledArgs...)
	args = append(args, minGames, limit*5)
	return r.scanChampionGuideItemPathRows(ctx, query, args, limit)
}

func (r *Repository) scanChampionGuideItemPathRows(ctx context.Context, query string, args []any, limit int) ([]ChampionGuideItemPathRow, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChampionGuideItemPathRow{}
	for rows.Next() {
		var row ChampionGuideItemPathRow
		if err := rows.Scan(&row.Core3Signature, &row.FinalItemsSignature, &row.Wins, &row.Games, &row.WinRate); err != nil {
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
		if out[i].WinRate != out[j].WinRate {
			return out[i].WinRate > out[j].WinRate
		}
		if out[i].Games != out[j].Games {
			return out[i].Games > out[j].Games
		}
		if out[i].Core3Signature != out[j].Core3Signature {
			return out[i].Core3Signature < out[j].Core3Signature
		}
		return out[i].FinalItemsSignature < out[j].FinalItemsSignature
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *Repository) queryChampionGuideBuildVariants(ctx context.Context, filters map[string]string, minGames, limit int) ([]ChampionGuideBuildVariantRow, error) {
	if minGames <= 0 {
		minGames = 5
	}
	if limit <= 0 {
		limit = 12
	}
	rows, hasSummary, err := r.queryChampionGuideBuildVariantsSummary(ctx, filters, minGames, limit)
	if err != nil {
		return nil, err
	}
	if hasSummary {
		return rows, nil
	}
	return r.queryChampionGuideBuildVariantsLiveScan(ctx, filters, minGames, limit)
}

func (r *Repository) queryChampionGuideBuildVariantsSummary(ctx context.Context, filters map[string]string, minGames, limit int) ([]ChampionGuideBuildVariantRow, bool, error) {
	championID := filterValue(filters["champion_id"])
	if championID == "" {
		return nil, false, nil
	}
	roleScope := strictAnalyticsRoleScope(filters["role"])
	query := `
		SELECT
			variant_key,
			variant_label,
			variant_tags,
			core2_signature,
			core3_signature,
			final_items_signature,
			rune_signature,
			spell_signature,
			skill_order_signature,
			skill_order_wins,
			skill_order_games,
			wins,
			games,
			build_count
		FROM champion_build_variant_analytics FINAL
		WHERE platform = 'ALL'
			AND champion_id = ?`
	args := []any{championID}
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
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	type variantAggregate struct {
		row                 ChampionGuideBuildVariantRow
		representativeGames int
		skills              map[string]buildVariantSkillAggregate
	}
	scanned := false
	variants := map[string]*variantAggregate{}
	for rows.Next() {
		scanned = true
		var row ChampionGuideBuildVariantRow
		if err := rows.Scan(
			&row.VariantKey,
			&row.VariantLabel,
			&row.VariantTags,
			&row.Core2Signature,
			&row.Core3Signature,
			&row.FinalItemsSignature,
			&row.RuneSignature,
			&row.SpellSignature,
			&row.SkillOrderSignature,
			&row.SkillOrderWins,
			&row.SkillOrderGames,
			&row.Wins,
			&row.Games,
			&row.BuildCount,
		); err != nil {
			return nil, false, err
		}
		aggregate := variants[row.VariantKey]
		if aggregate == nil {
			aggregate = &variantAggregate{
				row:                 row,
				representativeGames: row.Games,
				skills:              map[string]buildVariantSkillAggregate{},
			}
			aggregate.row.SkillOrderSignature = ""
			aggregate.row.SkillOrderWins = 0
			aggregate.row.SkillOrderGames = 0
			variants[row.VariantKey] = aggregate
		} else {
			aggregate.row.Wins += row.Wins
			aggregate.row.Games += row.Games
			aggregate.row.BuildCount += row.BuildCount
			aggregate.row.VariantTags = mergeBuildVariantTags(aggregate.row.VariantTags, row.VariantTags)
			if row.Games > aggregate.representativeGames {
				aggregate.row.VariantLabel = row.VariantLabel
				aggregate.row.Core2Signature = row.Core2Signature
				aggregate.row.Core3Signature = row.Core3Signature
				aggregate.row.FinalItemsSignature = row.FinalItemsSignature
				aggregate.row.RuneSignature = row.RuneSignature
				aggregate.row.SpellSignature = row.SpellSignature
				aggregate.representativeGames = row.Games
			}
		}
		if row.SkillOrderSignature != "" && row.SkillOrderGames > 0 {
			current := aggregate.skills[row.SkillOrderSignature]
			current.Wins += row.SkillOrderWins
			current.Games += row.SkillOrderGames
			aggregate.skills[row.SkillOrderSignature] = current
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if !scanned {
		return nil, false, nil
	}
	skillMinGames := minGames
	if skillMinGames < buildVariantSkillOrderMinGames {
		skillMinGames = buildVariantSkillOrderMinGames
	}
	out := []ChampionGuideBuildVariantRow{}
	for _, aggregate := range variants {
		row := aggregate.row
		if row.Games < minGames {
			continue
		}
		row.WinRate = float64(row.Wins) / float64(row.Games)
		row.Confidence = analytics.WilsonLowerBound(row.Wins, row.Games, 1.96)
		bestSignature := ""
		best := buildVariantSkillAggregate{}
		for signature, skill := range aggregate.skills {
			if skill.Games < skillMinGames {
				continue
			}
			if bestSignature == "" || skill.Games > best.Games || (skill.Games == best.Games && skill.Wins > best.Wins) {
				bestSignature = signature
				best = skill
			}
		}
		if bestSignature != "" && best.Games > 0 {
			row.SkillOrderSignature = bestSignature
			row.SkillOrderWins = best.Wins
			row.SkillOrderGames = best.Games
			row.SkillOrderWinRate = float64(best.Wins) / float64(best.Games)
			row.SkillOrderConfidence = analytics.WilsonLowerBound(best.Wins, best.Games, 1.96)
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Games != out[j].Games {
			return out[i].Games > out[j].Games
		}
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		if out[i].WinRate != out[j].WinRate {
			return out[i].WinRate > out[j].WinRate
		}
		return out[i].VariantKey < out[j].VariantKey
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, true, nil
}

func (r *Repository) queryChampionGuideBuildVariantsLiveScan(ctx context.Context, filters map[string]string, minGames, limit int) ([]ChampionGuideBuildVariantRow, error) {
	championID := filterValue(filters["champion_id"])
	if championID == "" {
		return nil, nil
	}
	roleScope := strictAnalyticsRoleScope(filters["role"])
	rawSQL, rawArgs := championGuideBaseSQLExcludingCompiledBuilds(filters, roleScope, true)
	compiledWhere := `
		FROM patch_build_metrics FINAL
		WHERE champion_id = ?
			AND core2_signature != ''
			AND final_items_signature != ''`
	compiledArgs := []any{championID}
	if roleScope.whereSQL != "" {
		compiledWhere += " AND " + roleScope.whereSQL
		compiledArgs = append(compiledArgs, roleScope.args...)
	}
	if filterValue(filters["patch"]) != "" {
		compiledWhere += " AND patch = ?"
		compiledArgs = append(compiledArgs, filterValue(filters["patch"]))
	}
	if filterValue(filters["rank_bucket"]) != "" {
		compiledWhere += " AND rank_bucket = ?"
		compiledArgs = append(compiledArgs, filterValue(filters["rank_bucket"]))
	}

	query := `
		SELECT
			core2_signature,
			argMax(core3_signature, row_games) AS core3_signature,
			argMax(final_items_signature, row_games) AS final_items_signature,
			argMax(rune_signature, row_games) AS rune_signature,
			argMax(spell_signature, row_games) AS spell_signature,
			toUInt64(sum(row_wins)) AS wins,
			toUInt64(sum(row_games)) AS games,
			wins / games AS win_rate,
			count() AS build_count
		FROM
		(
			SELECT
				core2_signature,
				core3_signature,
				final_items_signature,
				rune_signature,
				spell_signature,
				toUInt64(sum(win)) AS row_wins,
				toUInt64(count()) AS row_games
			` + rawSQL + `
				AND core2_signature != ''
				AND final_items_signature != ''
			GROUP BY core2_signature, core3_signature, final_items_signature, rune_signature, spell_signature
			UNION ALL
			SELECT
				core2_signature,
				core3_signature,
				final_items_signature,
				rune_signature,
				spell_signature,
				toUInt64(sum(wins)) AS row_wins,
				toUInt64(sum(games)) AS row_games
			` + compiledWhere + `
			GROUP BY core2_signature, core3_signature, final_items_signature, rune_signature, spell_signature
		)
		GROUP BY core2_signature
		HAVING games >= ?
		ORDER BY games DESC, win_rate DESC
		LIMIT ?`
	args := append([]any{}, rawArgs...)
	args = append(args, compiledArgs...)
	args = append(args, minGames, limit*12)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type variantAggregate struct {
		row                 ChampionGuideBuildVariantRow
		representativeGames int
	}
	variants := map[string]*variantAggregate{}
	for rows.Next() {
		var row ChampionGuideBuildVariantRow
		if err := rows.Scan(&row.Core2Signature, &row.Core3Signature, &row.FinalItemsSignature, &row.RuneSignature, &row.SpellSignature, &row.Wins, &row.Games, &row.WinRate, &row.BuildCount); err != nil {
			return nil, err
		}
		key := buildVariantCoreKey(row.Core3Signature, row.FinalItemsSignature, row.Core2Signature)
		if key == "" {
			continue
		}
		aggregate := variants[key]
		if aggregate == nil {
			aggregate = &variantAggregate{
				row: ChampionGuideBuildVariantRow{
					VariantKey:          key,
					Core2Signature:      key,
					Core3Signature:      row.Core3Signature,
					FinalItemsSignature: row.FinalItemsSignature,
					RuneSignature:       row.RuneSignature,
					SpellSignature:      row.SpellSignature,
				},
				representativeGames: row.Games,
			}
			variants[key] = aggregate
		}
		aggregate.row.Wins += row.Wins
		aggregate.row.Games += row.Games
		aggregate.row.BuildCount += row.BuildCount
		if row.Games > aggregate.representativeGames {
			aggregate.row.Core3Signature = row.Core3Signature
			aggregate.row.FinalItemsSignature = row.FinalItemsSignature
			aggregate.row.RuneSignature = row.RuneSignature
			aggregate.row.SpellSignature = row.SpellSignature
			aggregate.representativeGames = row.Games
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	labelGroups := map[string]*variantAggregate{}
	for _, aggregate := range variants {
		row := aggregate.row
		if row.Games < minGames {
			continue
		}
		row.VariantLabel, row.VariantTags = buildVariantLabelAndTags(row.Core2Signature + "-" + row.Core3Signature + "-" + row.FinalItemsSignature)
		groupKey := buildVariantGroupKey(row)
		group := labelGroups[groupKey]
		if group == nil {
			row.VariantKey = groupKey
			group = &variantAggregate{
				row:                 row,
				representativeGames: aggregate.representativeGames,
			}
			labelGroups[groupKey] = group
			continue
		}
		group.row.Wins += row.Wins
		group.row.Games += row.Games
		group.row.BuildCount += row.BuildCount
		group.row.VariantTags = mergeBuildVariantTags(group.row.VariantTags, row.VariantTags)
		if aggregate.representativeGames > group.representativeGames {
			group.row.Core2Signature = row.Core2Signature
			group.row.Core3Signature = row.Core3Signature
			group.row.FinalItemsSignature = row.FinalItemsSignature
			group.row.RuneSignature = row.RuneSignature
			group.row.SpellSignature = row.SpellSignature
			group.representativeGames = aggregate.representativeGames
		}
	}
	out := []ChampionGuideBuildVariantRow{}
	for _, aggregate := range labelGroups {
		row := aggregate.row
		if row.Games < minGames {
			continue
		}
		row.WinRate = float64(row.Wins) / float64(row.Games)
		row.Confidence = analytics.WilsonLowerBound(row.Wins, row.Games, 1.96)
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Games != out[j].Games {
			return out[i].Games > out[j].Games
		}
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		if out[i].WinRate != out[j].WinRate {
			return out[i].WinRate > out[j].WinRate
		}
		return out[i].VariantKey < out[j].VariantKey
	})
	if len(out) > limit {
		out = out[:limit]
	}
	if err := r.attachBuildVariantSkillOrders(ctx, filters, out, minGames); err != nil {
		return nil, err
	}
	return out, nil
}

type buildVariantSkillAggregate struct {
	Wins  int
	Games int
}

func (r *Repository) attachBuildVariantSkillOrders(ctx context.Context, filters map[string]string, variants []ChampionGuideBuildVariantRow, minGames int) error {
	if len(variants) == 0 {
		return nil
	}
	skillMinGames := minGames
	if skillMinGames < buildVariantSkillOrderMinGames {
		skillMinGames = buildVariantSkillOrderMinGames
	}
	wanted := map[string]int{}
	for index, variant := range variants {
		wanted[variant.VariantKey] = index
	}
	roleScope := strictAnalyticsRoleScope(filters["role"])
	query := `
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
			WHERE skill_slot BETWEEN 1 AND 4
			GROUP BY match_id, participant_id
			HAVING skill_order_signature != ''
		)
		SELECT
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
		WHERE pm.champion_id = ?
			AND pm.core2_signature != ''
			AND pm.final_items_signature != ''`
	args := []any{filterValue(filters["champion_id"])}
	if roleScope.whereSQL != "" {
		query += " AND " + qualifyRoleWhereSQL(roleScope.whereSQL, "pm.role")
		args = append(args, roleScope.args...)
	}
	if filterValue(filters["patch"]) != "" {
		query += " AND pm.patch = ?"
		args = append(args, filterValue(filters["patch"]))
	}
	if filterValue(filters["rank_bucket"]) != "" {
		query += " AND multiIf(s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket, pm.rank_bucket) = ?"
		args = append(args, filterValue(filters["rank_bucket"]))
	}
	query += `
		GROUP BY
			pm.core2_signature,
			pm.core3_signature,
			pm.final_items_signature,
			sp.skill_order_signature
		HAVING games >= ?
		ORDER BY games DESC`
	args = append(args, skillMinGames)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	type skillRow struct {
		core2, core3, final, signature string
		wins, games                    int
	}
	byVariant := map[string]map[string]buildVariantSkillAggregate{}
	for rows.Next() {
		var row skillRow
		if err := rows.Scan(&row.core2, &row.core3, &row.final, &row.signature, &row.wins, &row.games); err != nil {
			return err
		}
		key := buildVariantCoreKey(row.core3, row.final, row.core2)
		if key == "" {
			continue
		}
		label, tags := buildVariantLabelAndTags(row.core2 + "-" + row.core3 + "-" + row.final)
		groupKey := buildVariantGroupKey(ChampionGuideBuildVariantRow{
			VariantKey:   key,
			VariantLabel: label,
			VariantTags:  tags,
		})
		if _, ok := wanted[groupKey]; !ok {
			continue
		}
		if byVariant[groupKey] == nil {
			byVariant[groupKey] = map[string]buildVariantSkillAggregate{}
		}
		current := byVariant[groupKey][row.signature]
		current.Wins += row.wins
		current.Games += row.games
		byVariant[groupKey][row.signature] = current
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for variantKey, skills := range byVariant {
		index, ok := wanted[variantKey]
		if !ok {
			continue
		}
		bestSignature := ""
		best := buildVariantSkillAggregate{}
		for signature, aggregate := range skills {
			if aggregate.Games < skillMinGames {
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
		variants[index].SkillOrderSignature = bestSignature
		variants[index].SkillOrderWins = best.Wins
		variants[index].SkillOrderGames = best.Games
		variants[index].SkillOrderWinRate = float64(best.Wins) / float64(best.Games)
		variants[index].SkillOrderConfidence = analytics.WilsonLowerBound(best.Wins, best.Games, 1.96)
	}
	return nil
}

func buildVariantCoreKey(signatures ...string) string {
	items := []int{}
	seen := map[int]bool{}
	for _, signature := range signatures {
		for _, itemID := range parseItemSignature(signature) {
			if seen[itemID] || buildVariantIgnoredItemID(itemID) {
				continue
			}
			seen[itemID] = true
			items = append(items, itemID)
			if len(items) == 2 {
				return joinItemSignature(items)
			}
		}
	}
	if len(items) < 2 {
		return ""
	}
	return joinItemSignature(items)
}

func buildVariantGroupKey(row ChampionGuideBuildVariantRow) string {
	label := strings.TrimSpace(row.VariantLabel)
	if label == "" {
		return "core:" + row.VariantKey
	}
	return "label:" + strings.ToLower(strings.Join(strings.Fields(label), "-"))
}

func mergeBuildVariantTags(existing, next []string) []string {
	if len(existing) == 0 {
		return next
	}
	seen := map[string]bool{}
	merged := make([]string, 0, len(existing)+len(next))
	for _, tag := range append(existing, next...) {
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		merged = append(merged, tag)
	}
	return merged
}

func parseItemSignature(signature string) []int {
	if signature == "" {
		return nil
	}
	parts := strings.Split(signature, "-")
	items := make([]int, 0, len(parts))
	for _, part := range parts {
		itemID, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && itemID > 0 {
			items = append(items, itemID)
		}
	}
	return items
}

func joinItemSignature(items []int) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, itemID := range items {
		parts = append(parts, strconv.Itoa(itemID))
	}
	return strings.Join(parts, "-")
}

func buildVariantIgnoredItemID(itemID int) bool {
	if itemID >= 1101 && itemID <= 1103 {
		return true
	}
	if itemID >= 3865 && itemID <= 3877 {
		return true
	}
	switch itemID {
	case 1001, 2422, 3006, 3009, 3020, 3047, 3111, 3117, 3158, 3173,
		1004, 1006, 1011, 1026, 1027, 1028, 1029, 1031, 1033, 1035, 1036, 1037, 1038,
		1042, 1043, 1052, 1053, 1054, 1055, 1056, 1057, 1058, 1082, 1083,
		2003, 2010, 2015, 2021, 2022, 2031, 2033, 2055, 2420, 2421, 2423,
		3010, 3024, 3044, 3051, 3066, 3067, 3070, 3076, 3082, 3086, 3105, 3108,
		3113, 3114, 3123, 3133, 3134, 3140, 3145, 3155, 3211, 3801, 3802, 3916,
		4630, 4632, 4642, 6029, 6660, 6670, 6677, 6690:
		return true
	default:
		return false
	}
}

func buildVariantLabelAndTags(signature string) (string, []string) {
	scores := map[string]int{}
	for _, itemID := range parseItemSignature(signature) {
		if buildVariantIgnoredItemID(itemID) {
			continue
		}
		for _, tag := range buildVariantItemTags(itemID) {
			scores[tag]++
		}
	}
	tags := buildVariantSortedTags(scores)
	if scores["enchanter"] >= 2 {
		return "Enchanter", tags
	}
	if scores["support-tank"] >= 2 || (scores["support-tank"] >= 1 && scores["tank"] >= 1) {
		return "Support Tank", tags
	}
	if scores["tank"] >= 2 || (scores["tank"] >= 1 && scores["health"] >= 2 && scores["damage"] == 0) {
		return "Tank", tags
	}
	if scores["on-hit"] >= 2 || (scores["on-hit"] >= 1 && scores["attack-speed"] >= 2) {
		return "On Hit", tags
	}
	if scores["crit"] >= 2 {
		return "Crit", tags
	}
	if scores["lethality"] >= 2 || (scores["lethality"] >= 1 && scores["ad"] >= 2) {
		return "Lethality", tags
	}
	if scores["ad-bruiser"] >= 2 || (scores["ad-bruiser"] >= 1 && scores["health"] >= 1) {
		return "AD Bruiser", tags
	}
	if scores["ap-bruiser"] >= 1 && (scores["health"] >= 1 || scores["tank"] >= 1) {
		return "AP Bruiser", tags
	}
	if scores["ap"] >= 2 || scores["burst-ap"] >= 1 {
		return "AP Burst", tags
	}
	if scores["ad"] >= 2 {
		return "AD", tags
	}
	if scores["ap"] >= 1 && scores["ad"] >= 1 {
		return "Hybrid", tags
	}
	return "", tags
}

func buildVariantItemTags(itemID int) []string {
	switch itemID {
	case 3100, 3115, 3146, 4645, 3089, 3157, 3135, 6655, 6653, 2503, 4628:
		return []string{"ap", "burst-ap", "damage"}
	case 4633, 6665, 6657, 3152:
		return []string{"ap", "ap-bruiser", "health", "damage"}
	case 3124, 3153, 3091, 3302, 6672:
		return []string{"ad", "on-hit", "attack-speed", "damage"}
	case 6675, 3085:
		return []string{"crit", "on-hit", "attack-speed", "damage"}
	case 3031, 3033, 3036, 3094, 3508, 6676, 3032:
		return []string{"ad", "crit", "damage"}
	case 3142, 3814, 6694, 6696, 6701, 6692:
		return []string{"ad", "lethality", "damage"}
	case 3078, 3071, 3074, 3748, 3053, 3161, 6631, 6610, 3181, 6333:
		return []string{"ad", "ad-bruiser", "health", "damage"}
	case 3084, 3068, 6662, 3143, 3065, 3075, 4401, 8020, 2502, 2504, 6664, 3001, 2501:
		return []string{"tank", "health"}
	case 3190, 3002:
		return []string{"support-tank", "tank", "health"}
	case 6617, 3504, 6616, 6620, 3222, 2065, 3011, 4643:
		return []string{"enchanter", "utility"}
	default:
		return nil
	}
}

func buildVariantSortedTags(scores map[string]int) []string {
	tags := make([]string, 0, len(scores))
	for tag, score := range scores {
		if score > 0 {
			tags = append(tags, tag)
		}
	}
	sort.Slice(tags, func(i, j int) bool {
		if scores[tags[i]] != scores[tags[j]] {
			return scores[tags[i]] > scores[tags[j]]
		}
		return tags[i] < tags[j]
	})
	if len(tags) > 6 {
		return tags[:6]
	}
	return tags
}
