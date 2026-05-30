package clickhouse

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"winrift/services/core/internal/analytics"
)

const buildVariantSkillOrderMinGames = 10

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
	championID := strings.TrimSpace(filters["champion_id"])
	roleScope := strictAnalyticsRoleScope(filters["role"])
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
	direct.RoleRankTotal = len(candidates)
	return direct, nil
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
		rows[index].TierScore = clampFloat(
			0.64*rows[index].WinScore+
				0.12*rows[index].SampleScore+
				0.09*rows[index].PickScore+
				0.04*rows[index].BanScore+
				0.11*rows[index].ImpactScore,
			0,
			100,
		)
	}
	return rows
}

func (r *Repository) queryChampionGuideItemPaths(ctx context.Context, filters map[string]string, minGames, limit int) ([]ChampionGuideItemPathRow, error) {
	if minGames <= 0 {
		minGames = 5
	}
	if limit <= 0 {
		limit = 12
	}
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

func (r *Repository) queryChampionGuideMatchups(ctx context.Context, filters map[string]string, minGames, limit int, toughest bool) ([]ChampionGuideMatchupRow, error) {
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
