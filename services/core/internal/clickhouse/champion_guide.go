package clickhouse

import (
	"context"
	"database/sql"
	"sort"
	"strconv"
	"strings"

	"winrift/services/core/internal/analytics"
)

type ChampionGuideSummary struct {
	ChampionID    uint16
	Role          string
	PatchBucket   string
	RankBucket    string
	Wins          int
	Games         int
	WinRate       float64
	Confidence    float64
	PickRate      float64
	RoleRank      int
	RoleRankTotal int
}

type ChampionGuideMatchupRow struct {
	OpponentChampionID uint16
	Wins               int
	Games              int
	WinRate            float64
	Confidence         float64
}

type ChampionGuideSignatureRow struct {
	Signature  string
	Wins       int
	Games      int
	WinRate    float64
	Confidence float64
}

type ChampionGuideData struct {
	Summary          ChampionGuideSummary
	ToughestMatchups []ChampionGuideMatchupRow
	BestMatchups     []ChampionGuideMatchupRow
	TopRunes         []ChampionGuideSignatureRow
	TopSpells        []ChampionGuideSignatureRow
}

func (r *Repository) QueryChampionGuideIndex(ctx context.Context, filters map[string]string, minGames, limit int) ([]ChampionGuideSummary, error) {
	if minGames <= 0 {
		minGames = 1
	}
	if limit <= 0 {
		limit = 250
	}
	roleScope := analyticsRoleScope(filters["role"])
	baseSQL, baseArgs := championGuideBaseSQL(filters, roleScope, false)
	var totalRoleGames int
	if err := r.db.QueryRowContext(ctx, "SELECT count() "+baseSQL, baseArgs...).Scan(&totalRoleGames); err != nil {
		return nil, err
	}
	query := `
		SELECT
			champion_id,
			sum(win) AS wins,
			count() AS games,
			wins / games AS win_rate
		` + baseSQL + `
		GROUP BY champion_id
		HAVING games >= ?
		ORDER BY games DESC, champion_id ASC`
	args := append(append([]any{}, baseArgs...), minGames)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ChampionGuideSummary{}
	for rows.Next() {
		var row ChampionGuideSummary
		if err := rows.Scan(&row.ChampionID, &row.Wins, &row.Games, &row.WinRate); err != nil {
			return nil, err
		}
		row.Role = roleLabel(filters["role"])
		row.PatchBucket = patchBucketLabel(filters["patch"])
		row.RankBucket = rankBucketLabel(filters["rank_bucket"])
		if row.Games > 0 {
			row.Confidence = analytics.WilsonLowerBound(row.Wins, row.Games, 1.96)
		}
		if totalRoleGames > 0 {
			row.PickRate = float64(row.Games) / float64(totalRoleGames)
		}
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
		return out[i].ChampionID < out[j].ChampionID
	})
	totalRanked := len(out)
	for index := range out {
		out[index].RoleRank = index + 1
		out[index].RoleRankTotal = totalRanked
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *Repository) QueryChampionGuide(ctx context.Context, filters map[string]string, minGames, limit int) (ChampionGuideData, error) {
	if minGames <= 0 {
		minGames = 5
	}
	if limit <= 0 {
		limit = 12
	}
	summary, err := r.queryChampionGuideSummary(ctx, filters, minGames)
	if err != nil {
		return ChampionGuideData{}, err
	}
	toughest, err := r.queryChampionGuideMatchups(ctx, filters, minGames, limit, true)
	if err != nil {
		return ChampionGuideData{}, err
	}
	best, err := r.queryChampionGuideMatchups(ctx, filters, minGames, limit, false)
	if err != nil {
		return ChampionGuideData{}, err
	}
	runes, err := r.queryChampionGuideSignatures(ctx, filters, "rune_signature", minGames, limit)
	if err != nil {
		return ChampionGuideData{}, err
	}
	spells, err := r.queryChampionGuideSignatures(ctx, filters, "spell_signature", minGames, limit)
	if err != nil {
		return ChampionGuideData{}, err
	}
	return ChampionGuideData{
		Summary:          summary,
		ToughestMatchups: toughest,
		BestMatchups:     best,
		TopRunes:         runes,
		TopSpells:        spells,
	}, nil
}

func (r *Repository) queryChampionGuideSummary(ctx context.Context, filters map[string]string, minGames int) (ChampionGuideSummary, error) {
	championID := strings.TrimSpace(filters["champion_id"])
	roleScope := analyticsRoleScope(filters["role"])
	baseSQL, baseArgs := championGuideBaseSQL(filters, roleScope, false)
	var totalRoleGames int
	if err := r.db.QueryRowContext(ctx, "SELECT count() "+baseSQL, baseArgs...).Scan(&totalRoleGames); err != nil {
		return ChampionGuideSummary{}, err
	}

	rankQuery := `
		SELECT
			champion_id,
			sum(win) AS wins,
			count() AS games,
			wins / games AS win_rate
		` + baseSQL + `
		GROUP BY champion_id
		HAVING games >= ?
		ORDER BY win_rate DESC, games DESC, champion_id ASC`
	rankArgs := append(append([]any{}, baseArgs...), minGames)
	rows, err := r.db.QueryContext(ctx, rankQuery, rankArgs...)
	if err != nil {
		return ChampionGuideSummary{}, err
	}
	defer rows.Close()

	summary := ChampionGuideSummary{
		PatchBucket: patchBucketLabel(filters["patch"]),
		RankBucket:  rankBucketLabel(filters["rank_bucket"]),
	}
	if parsed, ok := parseUint16Filter(championID); ok {
		summary.ChampionID = parsed
	}
	rank := 0
	for rows.Next() {
		rank++
		var candidate ChampionGuideSummary
		if err := rows.Scan(&candidate.ChampionID, &candidate.Wins, &candidate.Games, &candidate.WinRate); err != nil {
			return ChampionGuideSummary{}, err
		}
		if candidate.Games > 0 {
			candidate.Confidence = analytics.WilsonLowerBound(candidate.Wins, candidate.Games, 1.96)
		}
		if totalRoleGames > 0 {
			candidate.PickRate = float64(candidate.Games) / float64(totalRoleGames)
		}
		candidate.Role = roleLabel(filters["role"])
		candidate.PatchBucket = summary.PatchBucket
		candidate.RankBucket = summary.RankBucket
		candidate.RoleRank = rank
		if candidate.ChampionID == summary.ChampionID {
			summary = candidate
		}
	}
	if err := rows.Err(); err != nil {
		return ChampionGuideSummary{}, err
	}
	summary.RoleRankTotal = rank
	if summary.Games > 0 {
		return summary, nil
	}
	direct, err := r.queryChampionGuideDirectSummary(ctx, filters, totalRoleGames)
	if err != nil {
		return ChampionGuideSummary{}, err
	}
	direct.RoleRankTotal = rank
	return direct, nil
}

func (r *Repository) queryChampionGuideDirectSummary(ctx context.Context, filters map[string]string, totalRoleGames int) (ChampionGuideSummary, error) {
	roleScope := analyticsRoleScope(filters["role"])
	baseSQL, args := championGuideBaseSQL(filters, roleScope, true)
	query := `
		SELECT
			champion_id,
			sum(win) AS wins,
			count() AS games,
			wins / games AS win_rate
		` + baseSQL + `
		GROUP BY champion_id
		LIMIT 1`
	summary := ChampionGuideSummary{
		Role:        roleLabel(filters["role"]),
		PatchBucket: patchBucketLabel(filters["patch"]),
		RankBucket:  rankBucketLabel(filters["rank_bucket"]),
	}
	if parsed, ok := parseUint16Filter(filters["champion_id"]); ok {
		summary.ChampionID = parsed
	}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&summary.ChampionID, &summary.Wins, &summary.Games, &summary.WinRate)
	if err != nil {
		if err == sql.ErrNoRows {
			return summary, nil
		}
		return ChampionGuideSummary{}, err
	}
	if summary.Games > 0 {
		summary.Confidence = analytics.WilsonLowerBound(summary.Wins, summary.Games, 1.96)
	}
	if totalRoleGames > 0 {
		summary.PickRate = float64(summary.Games) / float64(totalRoleGames)
	}
	return summary, nil
}

func (r *Repository) queryChampionGuideMatchups(ctx context.Context, filters map[string]string, minGames, limit int, toughest bool) ([]ChampionGuideMatchupRow, error) {
	roleScope := analyticsRoleScope(filters["role"])
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
	roleScope := analyticsRoleScope(filters["role"])
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
		return out[i].Signature < out[j].Signature
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func championGuideBaseSQL(filters map[string]string, roleScope roleAnalyticsScope, includeChampion bool) (string, []any) {
	query := `
		FROM
		(
			SELECT
				pm.champion_id,
				pm.role,
				pm.opponent_champion_id,
				pm.patch,
				multiIf(
					s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
					pm.rank_bucket
				) AS rank_bucket,
				pm.rune_signature,
				pm.spell_signature,
				pm.win
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
