package clickhouse

import (
	"context"
	"database/sql"
	"math"
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
	Bans          int
	WinRate       float64
	Confidence    float64
	PickRate      float64
	BanRate       float64
	AvgKills      float64
	AvgDeaths     float64
	AvgAssists    float64
	KDA           float64
	TierScore     float64
	WinScore      float64
	SampleScore   float64
	PickScore     float64
	BanScore      float64
	ImpactScore   float64
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

type ChampionGuideSkillOrderRow struct {
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
	TopSkillOrders   []ChampionGuideSkillOrderRow
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
			wins / games AS win_rate,
			sum(kills) AS kills,
			sum(deaths) AS deaths,
			sum(assists) AS assists
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
		var kills, deaths, assists int
		if err := rows.Scan(&row.ChampionID, &row.Wins, &row.Games, &row.WinRate, &kills, &deaths, &assists); err != nil {
			return nil, err
		}
		applyCombatAverages(&row, kills, deaths, assists)
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
	out, err = r.rankChampionGuideSummaries(ctx, filters, out)
	if err != nil {
		return nil, err
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
	skills, err := r.queryChampionGuideSkillOrders(ctx, filters, minGames, limit)
	if err != nil {
		return ChampionGuideData{}, err
	}
	banRates, err := r.queryChampionBanRates(ctx, filters)
	if err != nil {
		return ChampionGuideData{}, err
	}
	if banRate, ok := banRates[summary.ChampionID]; ok {
		summary.Bans = banRate.Bans
		summary.BanRate = banRate.BanRate
	}
	return ChampionGuideData{
		Summary:          summary,
		ToughestMatchups: toughest,
		BestMatchups:     best,
		TopRunes:         runes,
		TopSpells:        spells,
		TopSkillOrders:   skills,
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
			wins / games AS win_rate,
			sum(kills) AS kills,
			sum(deaths) AS deaths,
			sum(assists) AS assists
		` + baseSQL + `
		GROUP BY champion_id
		HAVING games >= ?
		ORDER BY games DESC, champion_id ASC`
	rankArgs := append(append([]any{}, baseArgs...), minGames)
	rows, err := r.db.QueryContext(ctx, rankQuery, rankArgs...)
	if err != nil {
		return ChampionGuideSummary{}, err
	}
	defer rows.Close()

	candidates := []ChampionGuideSummary{}
	summary := ChampionGuideSummary{
		PatchBucket: patchBucketLabel(filters["patch"]),
		RankBucket:  rankBucketLabel(filters["rank_bucket"]),
	}
	if parsed, ok := parseUint16Filter(championID); ok {
		summary.ChampionID = parsed
	}
	for rows.Next() {
		var candidate ChampionGuideSummary
		var kills, deaths, assists int
		if err := rows.Scan(&candidate.ChampionID, &candidate.Wins, &candidate.Games, &candidate.WinRate, &kills, &deaths, &assists); err != nil {
			return ChampionGuideSummary{}, err
		}
		applyCombatAverages(&candidate, kills, deaths, assists)
		if candidate.Games > 0 {
			candidate.Confidence = analytics.WilsonLowerBound(candidate.Wins, candidate.Games, 1.96)
		}
		if totalRoleGames > 0 {
			candidate.PickRate = float64(candidate.Games) / float64(totalRoleGames)
		}
		candidate.Role = roleLabel(filters["role"])
		candidate.PatchBucket = summary.PatchBucket
		candidate.RankBucket = summary.RankBucket
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return ChampionGuideSummary{}, err
	}
	candidates, err = r.rankChampionGuideSummaries(ctx, filters, candidates)
	if err != nil {
		return ChampionGuideSummary{}, err
	}
	for _, candidate := range candidates {
		if candidate.ChampionID == summary.ChampionID {
			return candidate, nil
		}
	}
	direct, err := r.queryChampionGuideDirectSummary(ctx, filters, totalRoleGames)
	if err != nil {
		return ChampionGuideSummary{}, err
	}
	direct.RoleRankTotal = len(candidates)
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
			wins / games AS win_rate,
			sum(kills) AS kills,
			sum(deaths) AS deaths,
			sum(assists) AS assists
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
	var kills, deaths, assists int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&summary.ChampionID, &summary.Wins, &summary.Games, &summary.WinRate, &kills, &deaths, &assists)
	if err != nil {
		if err == sql.ErrNoRows {
			return summary, nil
		}
		return ChampionGuideSummary{}, err
	}
	applyCombatAverages(&summary, kills, deaths, assists)
	if summary.Games > 0 {
		summary.Confidence = analytics.WilsonLowerBound(summary.Wins, summary.Games, 1.96)
	}
	if totalRoleGames > 0 {
		summary.PickRate = float64(summary.Games) / float64(totalRoleGames)
	}
	scored := applyChampionTierScores([]ChampionGuideSummary{summary})
	return scored[0], nil
}

func (r *Repository) rankChampionGuideSummaries(ctx context.Context, filters map[string]string, rows []ChampionGuideSummary) ([]ChampionGuideSummary, error) {
	if len(rows) == 0 {
		return rows, nil
	}
	banRates, err := r.queryChampionBanRates(ctx, filters)
	if err != nil {
		return nil, err
	}
	for index := range rows {
		if banRate, ok := banRates[rows[index].ChampionID]; ok {
			rows[index].Bans = banRate.Bans
			rows[index].BanRate = banRate.BanRate
		}
	}
	rows = applyChampionTierScores(rows)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TierScore != rows[j].TierScore {
			return rows[i].TierScore > rows[j].TierScore
		}
		if rows[i].WinScore != rows[j].WinScore {
			return rows[i].WinScore > rows[j].WinScore
		}
		if rows[i].Confidence != rows[j].Confidence {
			return rows[i].Confidence > rows[j].Confidence
		}
		if rows[i].WinRate != rows[j].WinRate {
			return rows[i].WinRate > rows[j].WinRate
		}
		if rows[i].Games != rows[j].Games {
			return rows[i].Games > rows[j].Games
		}
		return rows[i].ChampionID < rows[j].ChampionID
	})
	totalRanked := len(rows)
	for index := range rows {
		rows[index].RoleRank = index + 1
		rows[index].RoleRankTotal = totalRanked
	}
	return rows, nil
}

func applyCombatAverages(row *ChampionGuideSummary, kills, deaths, assists int) {
	if row.Games <= 0 {
		return
	}
	row.AvgKills = float64(kills) / float64(row.Games)
	row.AvgDeaths = float64(deaths) / float64(row.Games)
	row.AvgAssists = float64(assists) / float64(row.Games)
	if deaths > 0 {
		row.KDA = float64(kills+assists) / float64(deaths)
		return
	}
	row.KDA = float64(kills + assists)
}

func applyChampionTierScores(rows []ChampionGuideSummary) []ChampionGuideSummary {
	if len(rows) == 0 {
		return rows
	}
	maxGames := 0
	maxPickRate := 0.0
	maxBanRate := 0.0
	totalKDAWeight := 0
	weightedKDA := 0.0
	for _, row := range rows {
		if row.Games > maxGames {
			maxGames = row.Games
		}
		if row.PickRate > maxPickRate {
			maxPickRate = row.PickRate
		}
		if row.BanRate > maxBanRate {
			maxBanRate = row.BanRate
		}
		if row.Games > 0 && row.KDA > 0 {
			totalKDAWeight += row.Games
			weightedKDA += row.KDA * float64(row.Games)
		}
	}
	averageKDA := 0.0
	if totalKDAWeight > 0 {
		averageKDA = weightedKDA / float64(totalKDAWeight)
	}

	for index := range rows {
		winRateScore := clampFloat(50+((rows[index].WinRate-0.5)*500), 0, 100)
		confidenceScore := clampFloat(rows[index].Confidence*100, 0, 100)
		rows[index].WinScore = clampFloat(0.65*winRateScore+0.35*confidenceScore, 0, 100)
		if maxGames > 0 {
			rows[index].SampleScore = math.Sqrt(float64(rows[index].Games)/float64(maxGames)) * 100
		}
		rows[index].PickScore = normalizedShareScore(rows[index].PickRate, maxPickRate)
		rows[index].BanScore = normalizedShareScore(rows[index].BanRate, maxBanRate)
		rows[index].ImpactScore = normalizedImpactScore(rows[index].KDA, averageKDA)
		rows[index].TierScore = clampFloat(
			0.58*rows[index].WinScore+
				0.14*rows[index].SampleScore+
				0.12*rows[index].PickScore+
				0.08*rows[index].BanScore+
				0.08*rows[index].ImpactScore,
			0,
			100,
		)
	}
	return rows
}

func normalizedShareScore(value, maxValue float64) float64 {
	if maxValue <= 0 {
		return 50
	}
	return clampFloat(math.Sqrt(value/maxValue)*100, 0, 100)
}

func normalizedImpactScore(kda, averageKDA float64) float64 {
	if averageKDA <= 0 || kda <= 0 {
		return 50
	}
	return clampFloat(50+((kda-averageKDA)/averageKDA)*35, 0, 100)
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
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
				pm.win,
				pm.kills,
				pm.deaths,
				pm.assists
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
