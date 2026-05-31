package clickhouse

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"strings"
	"sync"

	"winrift/core/internal/analytics"
)

const (
	buildVariantSkillOrderMinGames = 10
	championGuideRankingMinGames   = 50
)

type ChampionGuideSummary struct {
	ChampionID                 uint16
	Role                       string
	PatchBucket                string
	RankBucket                 string
	Wins                       int
	Games                      int
	Bans                       int
	WinRate                    float64
	Confidence                 float64
	PickRate                   float64
	BanRate                    float64
	AvgKills                   float64
	AvgDeaths                  float64
	AvgAssists                 float64
	KDA                        float64
	AvgGoldEarned              float64
	AvgCS                      float64
	AvgDamageDealtToChampions  float64
	AvgDamageTaken             float64
	AvgDamageSelfMitigated     float64
	AvgDamageDealtToObjectives float64
	AvgDamageDealtToStructures float64
	AvgVisionScore             float64
	AvgTimeCCingOthers         float64
	AvgTeamUtility             float64
	AvgStructureTakedowns      float64
	AvgObjectiveTakedowns      float64
	AvgTotalTimeSpentDead      float64
	AvgTimePlayed              float64
	KillParticipation          float64
	TierScore                  float64
	WinScore                   float64
	SampleScore                float64
	PickScore                  float64
	BanScore                   float64
	ImpactScore                float64
	DamageScore                float64
	EconomyScore               float64
	VisionScore                float64
	ObjectiveScore             float64
	UtilityScore               float64
	SurvivabilityScore         float64
	RoleRank                   int
	RoleRankTotal              int
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

type ChampionGuideItemPathRow struct {
	Core3Signature      string
	FinalItemsSignature string
	Wins                int
	Games               int
	WinRate             float64
	Confidence          float64
}

type ChampionGuideBuildVariantRow struct {
	VariantKey           string
	VariantLabel         string
	VariantTags          []string
	Core2Signature       string
	Core3Signature       string
	FinalItemsSignature  string
	RuneSignature        string
	SpellSignature       string
	SkillOrderSignature  string
	SkillOrderWins       int
	SkillOrderGames      int
	SkillOrderWinRate    float64
	SkillOrderConfidence float64
	Wins                 int
	Games                int
	WinRate              float64
	Confidence           float64
	BuildCount           int
}

type ChampionGuideData struct {
	Summary          ChampionGuideSummary
	ToughestMatchups []ChampionGuideMatchupRow
	BestMatchups     []ChampionGuideMatchupRow
	TopRunes         []ChampionGuideSignatureRow
	TopSpells        []ChampionGuideSignatureRow
	TopSkillOrders   []ChampionGuideSkillOrderRow
	TopItemPaths     []ChampionGuideItemPathRow
	BuildVariants    []ChampionGuideBuildVariantRow
}

type ChampionGuideIndex struct {
	Results            []ChampionGuideSummary
	MatchCount         int
	ParticipantSamples int
}

type championGuidePerformance struct {
	Kills                      int
	Deaths                     int
	Assists                    int
	AvgGoldEarned              float64
	AvgCS                      float64
	AvgDamageDealtToChampions  float64
	AvgDamageTaken             float64
	AvgDamageSelfMitigated     float64
	AvgDamageDealtToObjectives float64
	AvgDamageDealtToStructures float64
	AvgVisionScore             float64
	AvgTimeCCingOthers         float64
	AvgTeamUtility             float64
	AvgStructureTakedowns      float64
	AvgObjectiveTakedowns      float64
	AvgTotalTimeSpentDead      float64
	AvgTimePlayed              float64
	KillParticipation          float64
}

func (performance *championGuidePerformance) scanTargets() []any {
	return []any{
		&performance.Kills,
		&performance.Deaths,
		&performance.Assists,
		&performance.AvgGoldEarned,
		&performance.AvgCS,
		&performance.AvgDamageDealtToChampions,
		&performance.AvgDamageTaken,
		&performance.AvgDamageSelfMitigated,
		&performance.AvgDamageDealtToObjectives,
		&performance.AvgDamageDealtToStructures,
		&performance.AvgVisionScore,
		&performance.AvgTimeCCingOthers,
		&performance.AvgTeamUtility,
		&performance.AvgStructureTakedowns,
		&performance.AvgObjectiveTakedowns,
		&performance.AvgTotalTimeSpentDead,
		&performance.AvgTimePlayed,
		&performance.KillParticipation,
	}
}

func (r *Repository) QueryChampionGuideIndex(ctx context.Context, filters map[string]string, minGames, limit int) (ChampionGuideIndex, error) {
	if minGames <= 0 {
		minGames = 1
	}
	if limit <= 0 {
		limit = 250
	}
	if index, ok, err := r.queryChampionGuideIndexSummary(ctx, filters, minGames, limit); err != nil {
		return ChampionGuideIndex{}, err
	} else if ok {
		return index, nil
	}
	return r.queryChampionGuideIndexRaw(ctx, filters, minGames, limit)
}

func (r *Repository) queryChampionGuideIndexRaw(ctx context.Context, filters map[string]string, minGames, limit int) (ChampionGuideIndex, error) {
	roleScope := strictAnalyticsRoleScope(filters["role"])
	baseSQL, baseArgs := championGuideBaseSQL(filters, roleScope, false)
	index := ChampionGuideIndex{}
	if err := r.db.QueryRowContext(ctx, "SELECT count(), uniqExact(match_id) "+baseSQL, baseArgs...).Scan(&index.ParticipantSamples, &index.MatchCount); err != nil {
		return ChampionGuideIndex{}, err
	}
	query := `
		SELECT
			champion_id,
			sum(win) AS wins,
			count() AS games,
			wins / games AS win_rate,
			sum(kills) AS total_kills,
			sum(deaths) AS total_deaths,
			sum(assists) AS total_assists,
			avg(gold_earned) AS avg_gold_earned,
			avg(total_minions_killed + neutral_minions_killed) AS avg_cs,
			avg(total_damage_dealt_to_champions) AS avg_damage_dealt_to_champions,
			avg(total_damage_taken) AS avg_damage_taken,
			avg(damage_self_mitigated) AS avg_damage_self_mitigated,
			avg(damage_dealt_to_objectives) AS avg_damage_dealt_to_objectives,
			avg(damage_dealt_to_turrets + damage_dealt_to_buildings) AS avg_damage_dealt_to_structures,
			avg(vision_score) AS avg_vision_score,
			avg(time_ccing_others) AS avg_time_ccing_others,
			avg(total_heal + total_heals_on_teammates + total_damage_shielded_on_teammates) AS avg_team_utility,
			avg(turret_takedowns + inhibitor_takedowns) AS avg_structure_takedowns,
			avg(dragon_kills + baron_kills + objectives_stolen) AS avg_objective_takedowns,
			avg(total_time_spent_dead) AS avg_total_time_spent_dead,
			avg(time_played) AS avg_time_played,
			avg(multiIf(team_kills > 0, toFloat64(kills + assists) / toFloat64(team_kills), 0)) AS kill_participation
		` + baseSQL + `
		GROUP BY champion_id
		HAVING games >= ?
		ORDER BY games DESC, champion_id ASC`
	args := append(append([]any{}, baseArgs...), minGames)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return ChampionGuideIndex{}, err
	}
	defer rows.Close()

	out := []ChampionGuideSummary{}
	for rows.Next() {
		var row ChampionGuideSummary
		var performance championGuidePerformance
		scanTargets := append([]any{&row.ChampionID, &row.Wins, &row.Games, &row.WinRate}, performance.scanTargets()...)
		if err := rows.Scan(scanTargets...); err != nil {
			return ChampionGuideIndex{}, err
		}
		applyPerformanceAverages(&row, performance)
		row.Role = roleLabel(filters["role"])
		row.PatchBucket = patchBucketLabel(filters["patch"])
		row.RankBucket = rankBucketLabel(filters["rank_bucket"])
		if row.Games > 0 {
			row.Confidence = analytics.WilsonLowerBound(row.Wins, row.Games, 1.96)
		}
		if index.ParticipantSamples > 0 {
			row.PickRate = float64(row.Games) / float64(index.ParticipantSamples)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return ChampionGuideIndex{}, err
	}
	out, err = r.rankChampionGuideSummaries(ctx, filters, out)
	if err != nil {
		return ChampionGuideIndex{}, err
	}
	if len(out) > limit {
		out = out[:limit]
	}
	index.Results = out
	return index, nil
}

func (r *Repository) queryChampionGuideIndexSummary(ctx context.Context, filters map[string]string, minGames, limit int) (ChampionGuideIndex, bool, error) {
	roleScope := strictAnalyticsRoleScope(filters["role"])
	index := ChampionGuideIndex{}
	scopeQuery := `
		SELECT
			toUInt64(sum(participant_samples)) AS participant_samples,
			toUInt64(sum(match_count)) AS match_count
		FROM champion_guide_scope_analytics FINAL
		WHERE platform = 'ALL'
			AND queue_id = ?
			AND role = ?
			AND rank_bucket = ?`
	scopeArgs := []any{analytics.RankedSoloQueueID, roleLabel(filters["role"]), rankBucketLabel(filters["rank_bucket"])}
	if filterValue(filters["patch"]) != "" {
		scopeQuery += " AND patch = ?"
		scopeArgs = append(scopeArgs, filterValue(filters["patch"]))
	}
	if err := r.db.QueryRowContext(ctx, scopeQuery, scopeArgs...).Scan(&index.ParticipantSamples, &index.MatchCount); err != nil {
		return ChampionGuideIndex{}, false, err
	}

	query := `
		SELECT
			champion_id,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games,
			wins / games AS win_rate,
			toUInt64(sum(kills)) AS total_kills,
			toUInt64(sum(deaths)) AS total_deaths,
			toUInt64(sum(assists)) AS total_assists,
			sum(gold_earned_sum) / games AS avg_gold_earned,
			sum(cs_sum) / games AS avg_cs,
			sum(damage_dealt_to_champions_sum) / games AS avg_damage_dealt_to_champions,
			sum(damage_taken_sum) / games AS avg_damage_taken,
			sum(damage_self_mitigated_sum) / games AS avg_damage_self_mitigated,
			sum(damage_dealt_to_objectives_sum) / games AS avg_damage_dealt_to_objectives,
			sum(damage_dealt_to_structures_sum) / games AS avg_damage_dealt_to_structures,
			sum(vision_score_sum) / games AS avg_vision_score,
			sum(time_ccing_others_sum) / games AS avg_time_ccing_others,
			sum(team_utility_sum) / games AS avg_team_utility,
			sum(structure_takedowns_sum) / games AS avg_structure_takedowns,
			sum(objective_takedowns_sum) / games AS avg_objective_takedowns,
			sum(total_time_spent_dead_sum) / games AS avg_total_time_spent_dead,
			sum(time_played_sum) / games AS avg_time_played,
			sum(kill_participation_sum) / games AS kill_participation
		FROM champion_guide_summary_analytics FINAL
		WHERE platform = 'ALL'
			AND queue_id = ?`
	args := []any{analytics.RankedSoloQueueID}
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
		GROUP BY champion_id
		HAVING games >= ?
		ORDER BY games DESC, champion_id ASC`
	args = append(args, minGames)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return ChampionGuideIndex{}, false, err
	}
	defer rows.Close()

	out := []ChampionGuideSummary{}
	for rows.Next() {
		var row ChampionGuideSummary
		var performance championGuidePerformance
		scanTargets := append([]any{&row.ChampionID, &row.Wins, &row.Games, &row.WinRate}, performance.scanTargets()...)
		if err := rows.Scan(scanTargets...); err != nil {
			return ChampionGuideIndex{}, false, err
		}
		applyPerformanceAverages(&row, performance)
		row.Role = roleLabel(filters["role"])
		row.PatchBucket = patchBucketLabel(filters["patch"])
		row.RankBucket = rankBucketLabel(filters["rank_bucket"])
		if row.Games > 0 {
			row.Confidence = analytics.WilsonLowerBound(row.Wins, row.Games, 1.96)
		}
		if index.ParticipantSamples > 0 {
			row.PickRate = float64(row.Games) / float64(index.ParticipantSamples)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return ChampionGuideIndex{}, false, err
	}
	if index.ParticipantSamples == 0 && len(out) == 0 {
		return ChampionGuideIndex{}, false, nil
	}
	out, err = r.rankChampionGuideSummaries(ctx, filters, out)
	if err != nil {
		return ChampionGuideIndex{}, false, err
	}
	if len(out) > limit {
		out = out[:limit]
	}
	index.Results = out
	return index, true, nil
}

func (r *Repository) QueryChampionGuide(ctx context.Context, filters map[string]string, minGames, limit int) (ChampionGuideData, error) {
	if minGames <= 0 {
		minGames = 5
	}
	if limit <= 0 {
		limit = 12
	}
	queryCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		summary       ChampionGuideSummary
		toughest      []ChampionGuideMatchupRow
		best          []ChampionGuideMatchupRow
		runes         []ChampionGuideSignatureRow
		spells        []ChampionGuideSignatureRow
		skills        []ChampionGuideSkillOrderRow
		itemPaths     []ChampionGuideItemPathRow
		buildVariants []ChampionGuideBuildVariantRow
		banRates      map[uint16]championBanRate
		wg            sync.WaitGroup
		mu            sync.Mutex
		firstErr      error
	)
	run := func(fn func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(queryCtx); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				mu.Unlock()
			}
		}()
	}
	run(func(ctx context.Context) error {
		var err error
		summary, err = r.queryChampionGuideSummary(ctx, filters, minGames)
		return err
	})
	run(func(ctx context.Context) error {
		var err error
		toughest, err = r.queryChampionGuideMatchups(ctx, filters, minGames, limit, true)
		return err
	})
	run(func(ctx context.Context) error {
		var err error
		best, err = r.queryChampionGuideMatchups(ctx, filters, minGames, limit, false)
		return err
	})
	run(func(ctx context.Context) error {
		var err error
		runes, err = r.queryChampionGuideSignatures(ctx, filters, "rune_signature", minGames, limit)
		return err
	})
	run(func(ctx context.Context) error {
		var err error
		spells, err = r.queryChampionGuideSignatures(ctx, filters, "spell_signature", minGames, limit)
		return err
	})
	run(func(ctx context.Context) error {
		var err error
		skills, err = r.queryChampionGuideSkillOrders(ctx, filters, minGames, limit)
		return err
	})
	run(func(ctx context.Context) error {
		var err error
		itemPaths, err = r.queryChampionGuideItemPaths(ctx, filters, minGames, limit)
		return err
	})
	run(func(ctx context.Context) error {
		var err error
		buildVariants, err = r.queryChampionGuideBuildVariants(ctx, filters, minGames, limit)
		return err
	})
	run(func(ctx context.Context) error {
		var err error
		banRates, err = r.queryChampionBanRates(ctx, filters)
		return err
	})
	wg.Wait()
	if firstErr != nil {
		return ChampionGuideData{}, firstErr
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
		TopItemPaths:     itemPaths,
		BuildVariants:    buildVariants,
	}, nil
}

func (r *Repository) queryChampionGuideSummary(ctx context.Context, filters map[string]string, minGames int) (ChampionGuideSummary, error) {
	if summary, ok, err := r.queryChampionGuideSummaryReadModel(ctx, filters, minGames); err != nil {
		return ChampionGuideSummary{}, err
	} else if ok {
		return summary, nil
	}
	return r.queryChampionGuideSummaryLiveScan(ctx, filters, minGames)
}

func (r *Repository) queryChampionGuideSummaryReadModel(ctx context.Context, filters map[string]string, minGames int) (ChampionGuideSummary, bool, error) {
	championID := strings.TrimSpace(filters["champion_id"])
	if championID == "" {
		return ChampionGuideSummary{}, false, nil
	}
	rankingMinGames := championGuideSummaryRankingMinGames(minGames)
	index, ok, err := r.queryChampionGuideIndexSummary(ctx, filters, rankingMinGames, 500)
	if err != nil {
		return ChampionGuideSummary{}, false, err
	}
	if !ok {
		return ChampionGuideSummary{}, false, nil
	}
	if parsed, ok := parseUint16Filter(championID); ok {
		for _, candidate := range index.Results {
			if candidate.ChampionID == parsed {
				return candidate, true, nil
			}
		}
	}
	direct, err := r.queryChampionGuideDirectSummaryReadModel(ctx, filters, index.ParticipantSamples)
	if err != nil {
		return ChampionGuideSummary{}, false, err
	}
	if direct.Games <= 0 {
		direct.RoleRankTotal = len(index.Results)
		return direct, true, nil
	}
	scoredCandidates := append(append([]ChampionGuideSummary{}, index.Results...), direct)
	scoredCandidates, err = r.rankChampionGuideSummaries(ctx, filters, scoredCandidates)
	if err != nil {
		return ChampionGuideSummary{}, false, err
	}
	for _, candidate := range scoredCandidates {
		if candidate.ChampionID == direct.ChampionID {
			if candidate.Games < rankingMinGames {
				candidate.RoleRank = 0
				candidate.RoleRankTotal = len(index.Results)
			}
			return candidate, true, nil
		}
	}
	direct.RoleRankTotal = len(index.Results)
	return direct, true, nil
}

func (r *Repository) queryChampionGuideSummaryLiveScan(ctx context.Context, filters map[string]string, minGames int) (ChampionGuideSummary, error) {
	championID := strings.TrimSpace(filters["champion_id"])
	roleScope := strictAnalyticsRoleScope(filters["role"])
	baseSQL, baseArgs := championGuideBaseSQL(filters, roleScope, false)
	rankingMinGames := championGuideSummaryRankingMinGames(minGames)
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
			sum(kills) AS total_kills,
			sum(deaths) AS total_deaths,
			sum(assists) AS total_assists,
			avg(gold_earned) AS avg_gold_earned,
			avg(total_minions_killed + neutral_minions_killed) AS avg_cs,
			avg(total_damage_dealt_to_champions) AS avg_damage_dealt_to_champions,
			avg(total_damage_taken) AS avg_damage_taken,
			avg(damage_self_mitigated) AS avg_damage_self_mitigated,
			avg(damage_dealt_to_objectives) AS avg_damage_dealt_to_objectives,
			avg(damage_dealt_to_turrets + damage_dealt_to_buildings) AS avg_damage_dealt_to_structures,
			avg(vision_score) AS avg_vision_score,
			avg(time_ccing_others) AS avg_time_ccing_others,
			avg(total_heal + total_heals_on_teammates + total_damage_shielded_on_teammates) AS avg_team_utility,
			avg(turret_takedowns + inhibitor_takedowns) AS avg_structure_takedowns,
			avg(dragon_kills + baron_kills + objectives_stolen) AS avg_objective_takedowns,
			avg(total_time_spent_dead) AS avg_total_time_spent_dead,
			avg(time_played) AS avg_time_played,
			avg(multiIf(team_kills > 0, toFloat64(kills + assists) / toFloat64(team_kills), 0)) AS kill_participation
		` + baseSQL + `
		GROUP BY champion_id
		HAVING games >= ?
		ORDER BY games DESC, champion_id ASC`
	rankArgs := append(append([]any{}, baseArgs...), rankingMinGames)
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
		var performance championGuidePerformance
		scanTargets := append([]any{&candidate.ChampionID, &candidate.Wins, &candidate.Games, &candidate.WinRate}, performance.scanTargets()...)
		if err := rows.Scan(scanTargets...); err != nil {
			return ChampionGuideSummary{}, err
		}
		applyPerformanceAverages(&candidate, performance)
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
	if direct.Games <= 0 {
		direct.RoleRankTotal = len(candidates)
		return direct, nil
	}
	scoredCandidates := append(append([]ChampionGuideSummary{}, candidates...), direct)
	scoredCandidates, err = r.rankChampionGuideSummaries(ctx, filters, scoredCandidates)
	if err != nil {
		return ChampionGuideSummary{}, err
	}
	for _, candidate := range scoredCandidates {
		if candidate.ChampionID == direct.ChampionID {
			if candidate.Games < rankingMinGames {
				candidate.RoleRank = 0
				candidate.RoleRankTotal = len(candidates)
			}
			return candidate, nil
		}
	}
	direct.RoleRankTotal = len(candidates)
	return direct, nil
}

func championGuideSummaryRankingMinGames(minGames int) int {
	if minGames > championGuideRankingMinGames {
		return minGames
	}
	return championGuideRankingMinGames
}

func (r *Repository) queryChampionGuideDirectSummary(ctx context.Context, filters map[string]string, totalRoleGames int) (ChampionGuideSummary, error) {
	roleScope := strictAnalyticsRoleScope(filters["role"])
	baseSQL, args := championGuideBaseSQL(filters, roleScope, true)
	query := `
		SELECT
			champion_id,
			sum(win) AS wins,
			count() AS games,
			wins / games AS win_rate,
			sum(kills) AS total_kills,
			sum(deaths) AS total_deaths,
			sum(assists) AS total_assists,
			avg(gold_earned) AS avg_gold_earned,
			avg(total_minions_killed + neutral_minions_killed) AS avg_cs,
			avg(total_damage_dealt_to_champions) AS avg_damage_dealt_to_champions,
			avg(total_damage_taken) AS avg_damage_taken,
			avg(damage_self_mitigated) AS avg_damage_self_mitigated,
			avg(damage_dealt_to_objectives) AS avg_damage_dealt_to_objectives,
			avg(damage_dealt_to_turrets + damage_dealt_to_buildings) AS avg_damage_dealt_to_structures,
			avg(vision_score) AS avg_vision_score,
			avg(time_ccing_others) AS avg_time_ccing_others,
			avg(total_heal + total_heals_on_teammates + total_damage_shielded_on_teammates) AS avg_team_utility,
			avg(turret_takedowns + inhibitor_takedowns) AS avg_structure_takedowns,
			avg(dragon_kills + baron_kills + objectives_stolen) AS avg_objective_takedowns,
			avg(total_time_spent_dead) AS avg_total_time_spent_dead,
			avg(time_played) AS avg_time_played,
			avg(multiIf(team_kills > 0, toFloat64(kills + assists) / toFloat64(team_kills), 0)) AS kill_participation
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
	var performance championGuidePerformance
	scanTargets := append([]any{&summary.ChampionID, &summary.Wins, &summary.Games, &summary.WinRate}, performance.scanTargets()...)
	err := r.db.QueryRowContext(ctx, query, args...).Scan(scanTargets...)
	if err != nil {
		if err == sql.ErrNoRows {
			return summary, nil
		}
		return ChampionGuideSummary{}, err
	}
	applyPerformanceAverages(&summary, performance)
	if summary.Games > 0 {
		summary.Confidence = analytics.WilsonLowerBound(summary.Wins, summary.Games, 1.96)
	}
	if totalRoleGames > 0 {
		summary.PickRate = float64(summary.Games) / float64(totalRoleGames)
	}
	scored := applyChampionTierScores([]ChampionGuideSummary{summary})
	return scored[0], nil
}

func (r *Repository) queryChampionGuideDirectSummaryReadModel(ctx context.Context, filters map[string]string, totalRoleGames int) (ChampionGuideSummary, error) {
	championID := strings.TrimSpace(filters["champion_id"])
	roleScope := strictAnalyticsRoleScope(filters["role"])
	query := `
		SELECT
			champion_id,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games,
			wins / games AS win_rate,
			toUInt64(sum(kills)) AS total_kills,
			toUInt64(sum(deaths)) AS total_deaths,
			toUInt64(sum(assists)) AS total_assists,
			sum(gold_earned_sum) / games AS avg_gold_earned,
			sum(cs_sum) / games AS avg_cs,
			sum(damage_dealt_to_champions_sum) / games AS avg_damage_dealt_to_champions,
			sum(damage_taken_sum) / games AS avg_damage_taken,
			sum(damage_self_mitigated_sum) / games AS avg_damage_self_mitigated,
			sum(damage_dealt_to_objectives_sum) / games AS avg_damage_dealt_to_objectives,
			sum(damage_dealt_to_structures_sum) / games AS avg_damage_dealt_to_structures,
			sum(vision_score_sum) / games AS avg_vision_score,
			sum(time_ccing_others_sum) / games AS avg_time_ccing_others,
			sum(team_utility_sum) / games AS avg_team_utility,
			sum(structure_takedowns_sum) / games AS avg_structure_takedowns,
			sum(objective_takedowns_sum) / games AS avg_objective_takedowns,
			sum(total_time_spent_dead_sum) / games AS avg_total_time_spent_dead,
			sum(time_played_sum) / games AS avg_time_played,
			sum(kill_participation_sum) / games AS kill_participation
		FROM champion_guide_summary_analytics FINAL
		WHERE platform = 'ALL'
			AND queue_id = ?
			AND champion_id = ?`
	args := []any{analytics.RankedSoloQueueID, championID}
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
		GROUP BY champion_id
		LIMIT 1`
	summary := ChampionGuideSummary{
		Role:        roleLabel(filters["role"]),
		PatchBucket: patchBucketLabel(filters["patch"]),
		RankBucket:  rankBucketLabel(filters["rank_bucket"]),
	}
	if parsed, ok := parseUint16Filter(championID); ok {
		summary.ChampionID = parsed
	}
	var performance championGuidePerformance
	scanTargets := append([]any{&summary.ChampionID, &summary.Wins, &summary.Games, &summary.WinRate}, performance.scanTargets()...)
	err := r.db.QueryRowContext(ctx, query, args...).Scan(scanTargets...)
	if err != nil {
		if err == sql.ErrNoRows {
			return summary, nil
		}
		return ChampionGuideSummary{}, err
	}
	applyPerformanceAverages(&summary, performance)
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
	banRates, err := r.queryChampionBanRates(ctx, championGuideRankingFilters(filters))
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

func championGuideRankingFilters(filters map[string]string) map[string]string {
	out := make(map[string]string, len(filters))
	for key, value := range filters {
		out[key] = value
	}
	delete(out, "champion_id")
	return out
}

func applyCombatAverages(row *ChampionGuideSummary, kills, deaths, assists int) {
	applyPerformanceAverages(row, championGuidePerformance{Kills: kills, Deaths: deaths, Assists: assists})
}

func applyPerformanceAverages(row *ChampionGuideSummary, performance championGuidePerformance) {
	if row.Games <= 0 {
		return
	}
	row.AvgKills = float64(performance.Kills) / float64(row.Games)
	row.AvgDeaths = float64(performance.Deaths) / float64(row.Games)
	row.AvgAssists = float64(performance.Assists) / float64(row.Games)
	if performance.Deaths > 0 {
		row.KDA = float64(performance.Kills+performance.Assists) / float64(performance.Deaths)
	} else {
		row.KDA = float64(performance.Kills + performance.Assists)
	}
	row.AvgGoldEarned = performance.AvgGoldEarned
	row.AvgCS = performance.AvgCS
	row.AvgDamageDealtToChampions = performance.AvgDamageDealtToChampions
	row.AvgDamageTaken = performance.AvgDamageTaken
	row.AvgDamageSelfMitigated = performance.AvgDamageSelfMitigated
	row.AvgDamageDealtToObjectives = performance.AvgDamageDealtToObjectives
	row.AvgDamageDealtToStructures = performance.AvgDamageDealtToStructures
	row.AvgVisionScore = performance.AvgVisionScore
	row.AvgTimeCCingOthers = performance.AvgTimeCCingOthers
	row.AvgTeamUtility = performance.AvgTeamUtility
	row.AvgStructureTakedowns = performance.AvgStructureTakedowns
	row.AvgObjectiveTakedowns = performance.AvgObjectiveTakedowns
	row.AvgTotalTimeSpentDead = performance.AvgTotalTimeSpentDead
	row.AvgTimePlayed = performance.AvgTimePlayed
	row.KillParticipation = performance.KillParticipation
}

type championTierReferences struct {
	KDA                    float64
	KillParticipation      float64
	GoldEarned             float64
	CS                     float64
	DamageDealtToChampions float64
	DamageAbsorption       float64
	VisionScore            float64
	ObjectiveContribution  float64
	Utility                float64
	Survivability          float64
}

func championTierReferenceValues(rows []ChampionGuideSummary) championTierReferences {
	var refs championTierReferences
	var weight float64
	for _, row := range rows {
		if row.Games <= 0 {
			continue
		}
		rowWeight := float64(row.Games)
		weight += rowWeight
		refs.KDA += row.KDA * rowWeight
		refs.KillParticipation += row.KillParticipation * rowWeight
		refs.GoldEarned += row.AvgGoldEarned * rowWeight
		refs.CS += row.AvgCS * rowWeight
		refs.DamageDealtToChampions += row.AvgDamageDealtToChampions * rowWeight
		refs.DamageAbsorption += damageAbsorptionContribution(row) * rowWeight
		refs.VisionScore += row.AvgVisionScore * rowWeight
		refs.ObjectiveContribution += objectiveContribution(row) * rowWeight
		refs.Utility += utilityContribution(row) * rowWeight
		refs.Survivability += survivabilityContribution(row) * rowWeight
	}
	if weight <= 0 {
		return refs
	}
	refs.KDA /= weight
	refs.KillParticipation /= weight
	refs.GoldEarned /= weight
	refs.CS /= weight
	refs.DamageDealtToChampions /= weight
	refs.DamageAbsorption /= weight
	refs.VisionScore /= weight
	refs.ObjectiveContribution /= weight
	refs.Utility /= weight
	refs.Survivability /= weight
	return refs
}

func applyChampionImpactScores(row *ChampionGuideSummary, refs championTierReferences) {
	kdaScore := normalizedImpactScore(row.KDA, refs.KDA)
	killParticipationScore := normalizedImpactScore(row.KillParticipation, refs.KillParticipation)
	row.DamageScore = normalizedImpactScore(row.AvgDamageDealtToChampions, refs.DamageDealtToChampions)
	damageAbsorptionScore := normalizedImpactScore(damageAbsorptionContribution(*row), refs.DamageAbsorption)
	goldScore := normalizedImpactScore(row.AvgGoldEarned, refs.GoldEarned)
	csScore := normalizedImpactScore(row.AvgCS, refs.CS)
	row.EconomyScore = 0.65*goldScore + 0.35*csScore
	row.VisionScore = normalizedImpactScore(row.AvgVisionScore, refs.VisionScore)
	row.ObjectiveScore = normalizedImpactScore(objectiveContribution(*row), refs.ObjectiveContribution)
	row.UtilityScore = normalizedImpactScore(utilityContribution(*row), refs.Utility)
	row.SurvivabilityScore = normalizedImpactScore(survivabilityContribution(*row), refs.Survivability)
	row.SurvivabilityScore = clampFloat(0.68*row.SurvivabilityScore+0.32*damageAbsorptionScore, 0, 100)

	kpKdaScore := 0.55*kdaScore + 0.45*killParticipationScore
	switch strings.ToUpper(row.Role) {
	case "UTILITY":
		row.ImpactScore = 0.22*row.VisionScore + 0.18*row.UtilityScore + 0.16*kpKdaScore + 0.12*row.SurvivabilityScore + 0.10*row.ObjectiveScore + 0.10*row.DamageScore + 0.12*row.EconomyScore
	case "JUNGLE":
		row.ImpactScore = 0.20*row.ObjectiveScore + 0.18*kpKdaScore + 0.17*row.DamageScore + 0.15*row.EconomyScore + 0.12*row.VisionScore + 0.10*row.SurvivabilityScore + 0.08*row.UtilityScore
	case "BOTTOM":
		row.ImpactScore = 0.26*row.DamageScore + 0.20*row.EconomyScore + 0.18*kpKdaScore + 0.14*row.SurvivabilityScore + 0.12*row.ObjectiveScore + 0.06*row.UtilityScore + 0.04*row.VisionScore
	case "TOP":
		row.ImpactScore = 0.18*row.DamageScore + 0.16*row.EconomyScore + 0.14*kpKdaScore + 0.15*row.ObjectiveScore + 0.17*row.SurvivabilityScore + 0.10*row.UtilityScore + 0.10*row.VisionScore
	case "MIDDLE":
		row.ImpactScore = 0.23*row.DamageScore + 0.17*row.EconomyScore + 0.18*kpKdaScore + 0.12*row.ObjectiveScore + 0.10*row.SurvivabilityScore + 0.10*row.UtilityScore + 0.10*row.VisionScore
	default:
		row.ImpactScore = 0.18*row.DamageScore + 0.16*row.EconomyScore + 0.16*kpKdaScore + 0.14*row.ObjectiveScore + 0.13*row.VisionScore + 0.12*row.UtilityScore + 0.11*row.SurvivabilityScore
	}
	row.ImpactScore = clampFloat(row.ImpactScore, 0, 100)
}

func objectiveContribution(row ChampionGuideSummary) float64 {
	return row.AvgDamageDealtToObjectives + row.AvgDamageDealtToStructures + row.AvgStructureTakedowns*750 + row.AvgObjectiveTakedowns*900
}

func utilityContribution(row ChampionGuideSummary) float64 {
	return row.AvgTimeCCingOthers*40 + row.AvgTeamUtility
}

func survivabilityContribution(row ChampionGuideSummary) float64 {
	deathShare := 0.0
	if row.AvgTimePlayed > 0 {
		deathShare = row.AvgTotalTimeSpentDead / row.AvgTimePlayed
	}
	survivalScore := (1 - clampFloat(deathShare, 0, 1)) * 100
	if row.AvgDamageTaken <= 0 {
		return survivalScore
	}
	mitigationRatio := row.AvgDamageSelfMitigated / row.AvgDamageTaken
	return survivalScore + clampFloat(mitigationRatio, 0, 2)*20
}

func damageAbsorptionContribution(row ChampionGuideSummary) float64 {
	minutes := row.AvgTimePlayed / 60
	if minutes <= 0 {
		minutes = 1
	}
	deathShare := 0.0
	if row.AvgTimePlayed > 0 {
		deathShare = row.AvgTotalTimeSpentDead / row.AvgTimePlayed
	}
	survivalFactor := 0.6 + 0.4*(1-clampFloat(deathShare, 0, 1))
	return ((row.AvgDamageTaken + row.AvgDamageSelfMitigated*0.65) / minutes) * survivalFactor
}

func normalizedPositiveScore(value, average float64) float64 {
	if average <= 0 || value <= 0 {
		return 50
	}
	return clampFloat(50+((value-average)/average)*35, 0, 100)
}

func normalizedImpactScore(value, average float64) float64 {
	return normalizedPositiveScore(value, average)
}

func normalizedShareScore(value, maxValue float64) float64 {
	if maxValue <= 0 {
		return 50
	}
	return clampFloat(math.Sqrt(value/maxValue)*100, 0, 100)
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

func applyChampionTierScores(rows []ChampionGuideSummary) []ChampionGuideSummary {
	if len(rows) == 0 {
		return rows
	}
	maxGames := 0
	maxPickRate := 0.0
	maxBanRate := 0.0
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
	}
	refs := championTierReferenceValues(rows)

	for index := range rows {
		winRateScore := clampFloat(50+((rows[index].WinRate-0.5)*500), 0, 100)
		winRateReliability := clampFloat(math.Sqrt(float64(rows[index].Games)/250), 0, 1)
		winRateScore = 50 + (winRateScore-50)*winRateReliability
		confidenceScore := clampFloat(rows[index].Confidence*100, 0, 100)
		rows[index].WinScore = clampFloat(0.40*winRateScore+0.60*confidenceScore, 0, 100)
		if maxGames > 0 {
			rows[index].SampleScore = math.Sqrt(float64(rows[index].Games)/float64(maxGames)) * 100
		}
		rows[index].PickScore = normalizedShareScore(rows[index].PickRate, maxPickRate)
		rows[index].BanScore = normalizedShareScore(rows[index].BanRate, maxBanRate)
		applyChampionImpactScores(&rows[index], refs)
		presenceScore := 0.70*rows[index].PickScore + 0.30*rows[index].BanScore
		tierScore := 0.70*rows[index].WinScore +
			0.08*rows[index].SampleScore +
			0.05*presenceScore +
			0.17*rows[index].ImpactScore
		if rows[index].WinRate < 0.50 {
			tierScore -= (0.50 - rows[index].WinRate) * 250
		}
		if rows[index].WinRate < 0.49 {
			tierScore -= (0.49 - rows[index].WinRate) * 250
		}
		if rows[index].Confidence < 0.47 {
			tierScore -= (0.47 - rows[index].Confidence) * 80
		}
		rows[index].TierScore = clampFloat(tierScore, 0, 100)
	}
	return rows
}
