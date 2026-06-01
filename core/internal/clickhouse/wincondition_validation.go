package clickhouse

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
)

type WinConditionValidationFilters struct {
	QueueID                uint16
	Patch                  string
	Platform               string
	RankBucket             string
	MinGames               int
	WeakSignalWinRate      float64
	Limit                  int
	SynergyMinGames        int
	SynergyLimit           int
	SynergyParentLimit     int
	SynergyMinParentSignal float64
}

type WinConditionValidation struct {
	Patch                  string                              `json:"patch"`
	Platform               string                              `json:"platform"`
	QueueID                uint16                              `json:"queueId"`
	RankBucket             string                              `json:"rankBucket"`
	MinGames               int                                 `json:"minGames"`
	WeakSignalWinRate      float64                             `json:"weakSignalWinRate"`
	SynergyMinGames        int                                 `json:"synergyMinGames"`
	SynergyMinParentSignal float64                             `json:"synergyMinParentSignal"`
	Teams                  int                                 `json:"teams"`
	Matches                int                                 `json:"matches"`
	RatingOutcomes         []WinConditionRatingOutcome         `json:"ratingOutcomes"`
	ScoreDeltaOutcomes     []WinConditionScoreDeltaOutcome     `json:"scoreDeltaOutcomes"`
	PrimaryMatchups        []WinConditionPrimaryMatchupOutcome `json:"primaryMatchups"`
	PrimaryMarginOutcomes  []WinConditionPrimaryMarginOutcome  `json:"primaryMarginOutcomes"`
	SynergyResiduals       []WinConditionSynergyResidual       `json:"synergyResiduals"`
	WeakSignalWarnings     []WinConditionWeakSignalWarning     `json:"weakSignalWarnings"`
	Findings               []WinConditionValidationFinding     `json:"findings"`
}

type WinConditionRatingOutcome struct {
	Axis       string  `json:"axis"`
	Rating     string  `json:"rating"`
	Games      int     `json:"games"`
	Wins       int     `json:"wins"`
	WinRate    float64 `json:"winRate"`
	Confidence float64 `json:"confidence"`
	AvgScore   float64 `json:"avgScore"`
	MinScore   int     `json:"minScore"`
	MaxScore   int     `json:"maxScore"`
}

type WinConditionScoreDeltaOutcome struct {
	Axis       string  `json:"axis"`
	Bucket     string  `json:"bucket"`
	Games      int     `json:"games"`
	Wins       int     `json:"wins"`
	WinRate    float64 `json:"winRate"`
	Confidence float64 `json:"confidence"`
	AvgDelta   float64 `json:"avgDelta"`
}

type WinConditionPrimaryMatchupOutcome struct {
	Condition         string  `json:"condition"`
	Rating            string  `json:"rating"`
	OpponentCondition string  `json:"opponentCondition"`
	OpponentRating    string  `json:"opponentRating"`
	Games             int     `json:"games"`
	Wins              int     `json:"wins"`
	WinRate           float64 `json:"winRate"`
	Confidence        float64 `json:"confidence"`
	Edge              float64 `json:"edge"`
	WilsonLow         float64 `json:"wilsonLow"`
	WilsonHigh        float64 `json:"wilsonHigh"`
	Signal            float64 `json:"signal"`
	Direction         string  `json:"direction"`
}

type WinConditionPrimaryMarginOutcome struct {
	Bucket     string  `json:"bucket"`
	Games      int     `json:"games"`
	Wins       int     `json:"wins"`
	WinRate    float64 `json:"winRate"`
	Confidence float64 `json:"confidence"`
	AvgMargin  float64 `json:"avgMargin"`
}

type WinConditionWeakSignalWarning struct {
	Condition         string  `json:"condition"`
	Rating            string  `json:"rating"`
	OpponentCondition string  `json:"opponentCondition"`
	OpponentRating    string  `json:"opponentRating"`
	TeamPrimary       uint8   `json:"teamPrimary"`
	Games             int     `json:"games"`
	Wins              int     `json:"wins"`
	WinRate           float64 `json:"winRate"`
	Confidence        float64 `json:"confidence"`
	Note              string  `json:"note"`
}

type WinConditionSynergyResidual struct {
	PairType          string  `json:"pairType"`
	ChampionID1       uint16  `json:"championId1"`
	ChampionID2       uint16  `json:"championId2"`
	Condition         string  `json:"condition"`
	Rating            string  `json:"rating"`
	OpponentCondition string  `json:"opponentCondition"`
	OpponentRating    string  `json:"opponentRating"`
	ParentGames       int     `json:"parentGames"`
	ParentWinRate     float64 `json:"parentWinRate"`
	ParentSignal      float64 `json:"parentSignal"`
	Games             int     `json:"games"`
	Wins              int     `json:"wins"`
	WinRate           float64 `json:"winRate"`
	WilsonLow         float64 `json:"wilsonLow"`
	WilsonHigh        float64 `json:"wilsonHigh"`
	Residual          float64 `json:"residual"`
	Signal            float64 `json:"signal"`
	Direction         string  `json:"direction"`
}

type WinConditionValidationFinding struct {
	Severity string `json:"severity"`
	Topic    string `json:"topic"`
	Summary  string `json:"summary"`
}

type winConditionPrimaryMatchupKey struct {
	condition         string
	rating            string
	opponentCondition string
	opponentRating    string
}

type winConditionSynergyKey struct {
	parent      winConditionPrimaryMatchupKey
	pairType    string
	championID1 uint16
	championID2 uint16
}

type winConditionSynergyCounts struct {
	wins  uint64
	games uint64
}

func (r *Repository) QueryWinConditionValidation(ctx context.Context, filters WinConditionValidationFilters) (WinConditionValidation, error) {
	normalized := normalizeWinConditionValidationFilters(filters)
	platformFilter := normalized.Platform
	if platformFilter == "ALL" {
		platformFilter = ""
	}
	rankFilter := normalized.RankBucket
	if rankFilter == "ALL" {
		rankFilter = ""
	}
	where, args := winConditionDiagnosticsWhere(normalized.QueueID, normalized.Patch, platformFilter, rankFilter)
	diagnostics := WinConditionDiagnostics{
		Platform:   normalized.Platform,
		QueueID:    normalized.QueueID,
		RankBucket: normalized.RankBucket,
	}
	if err := r.queryWinConditionDiagnosticsSummary(ctx, where, args, &diagnostics); err != nil {
		return WinConditionValidation{}, err
	}

	validation := WinConditionValidation{
		Patch:                  diagnostics.Patch,
		Platform:               diagnostics.Platform,
		QueueID:                diagnostics.QueueID,
		RankBucket:             diagnostics.RankBucket,
		MinGames:               normalized.MinGames,
		WeakSignalWinRate:      normalized.WeakSignalWinRate,
		SynergyMinGames:        normalized.SynergyMinGames,
		SynergyMinParentSignal: normalized.SynergyMinParentSignal,
		Teams:                  diagnostics.Teams,
		Matches:                diagnostics.Matches,
	}
	var err error
	if validation.RatingOutcomes, err = r.queryWinConditionRatingOutcomes(ctx, where, args); err != nil {
		return WinConditionValidation{}, err
	}
	if validation.ScoreDeltaOutcomes, err = r.queryWinConditionScoreDeltaOutcomes(ctx, where, args); err != nil {
		return WinConditionValidation{}, err
	}
	if validation.PrimaryMatchups, err = r.queryWinConditionPrimaryMatchupOutcomes(ctx, where, args, normalized.MinGames, normalized.Limit); err != nil {
		return WinConditionValidation{}, err
	}
	if validation.PrimaryMarginOutcomes, err = r.queryWinConditionPrimaryMarginOutcomes(ctx, where, args); err != nil {
		return WinConditionValidation{}, err
	}
	if validation.SynergyResiduals, err = r.queryWinConditionSynergyResiduals(ctx, where, args, validation.PrimaryMatchups, normalized); err != nil {
		return WinConditionValidation{}, err
	}
	if validation.WeakSignalWarnings, err = r.queryWinConditionWeakSignalWarnings(ctx, normalized, validation.Patch); err != nil {
		return WinConditionValidation{}, err
	}
	validation.Findings = winConditionValidationFindings(validation)
	return validation, nil
}

func normalizeWinConditionValidationFilters(filters WinConditionValidationFilters) WinConditionValidationFilters {
	if filters.QueueID == 0 {
		filters.QueueID = 420
	}
	filters.Patch = strings.TrimSpace(filters.Patch)
	filters.Platform = strings.ToUpper(strings.TrimSpace(filters.Platform))
	if filters.Platform == "" {
		filters.Platform = "ALL"
	}
	filters.RankBucket = strings.ToUpper(strings.TrimSpace(filters.RankBucket))
	if filters.RankBucket == "" {
		filters.RankBucket = "ALL"
	}
	if filters.MinGames <= 0 {
		filters.MinGames = 50
	}
	if filters.WeakSignalWinRate <= 0 {
		filters.WeakSignalWinRate = 55
	}
	if filters.Limit <= 0 {
		filters.Limit = 25
	}
	if filters.SynergyMinGames <= 0 {
		filters.SynergyMinGames = 25
	}
	if filters.SynergyLimit <= 0 {
		filters.SynergyLimit = 25
	}
	if filters.SynergyParentLimit <= 0 {
		filters.SynergyParentLimit = 6
	}
	if filters.SynergyMinParentSignal <= 0 {
		filters.SynergyMinParentSignal = 1
	}
	return filters
}

func (r *Repository) queryWinConditionRatingOutcomes(ctx context.Context, where string, args []any) ([]WinConditionRatingOutcome, error) {
	selects := []string{
		"SELECT 'SplitPush' AS axis, splitpush_rating AS rating, toUInt8(splitpush_score) AS score, win FROM match_team_win_conditions FINAL " + where,
		"SELECT 'Pick' AS axis, pick_rating AS rating, toUInt8(pick_score) AS score, win FROM match_team_win_conditions FINAL " + where,
		"SELECT 'Siege' AS axis, siege_rating AS rating, toUInt8(siege_score) AS score, win FROM match_team_win_conditions FINAL " + where,
		"SELECT 'Control' AS axis, control_rating AS rating, toUInt8(control_score) AS score, win FROM match_team_win_conditions FINAL " + where,
		"SELECT 'TeamFight' AS axis, teamfight_rating AS rating, toUInt8(teamfight_score) AS score, win FROM match_team_win_conditions FINAL " + where,
	}
	query := `
		SELECT
			axis,
			rating,
			toUInt64(count()) AS games,
			toUInt64(sum(win)) AS wins,
			avg(score) AS avg_score,
			toUInt8(min(score)) AS min_score,
			toUInt8(max(score)) AS max_score
		FROM (` + strings.Join(selects, " UNION ALL ") + `)
		GROUP BY axis, rating`
	rows, err := r.db.QueryContext(ctx, query, repeatArgs(args, len(selects))...)
	if err != nil {
		return nil, fmt.Errorf("query win condition validation rating outcomes: %w", err)
	}
	defer rows.Close()
	out := []WinConditionRatingOutcome{}
	for rows.Next() {
		var row WinConditionRatingOutcome
		var wins, games uint64
		var minScore, maxScore uint8
		if err := rows.Scan(&row.Axis, &row.Rating, &games, &wins, &row.AvgScore, &minScore, &maxScore); err != nil {
			return nil, fmt.Errorf("scan win condition validation rating outcome: %w", err)
		}
		row.Games = int(games)
		row.Wins = int(wins)
		row.WinRate = winConditionWinRatePercent(wins, games)
		row.Confidence = winConditionConfidencePercent(wins, games)
		row.AvgScore = winConditionRoundPercent(row.AvgScore)
		row.MinScore = int(minScore)
		row.MaxScore = int(maxScore)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Axis != out[j].Axis {
			return winConditionAxisOrder(out[i].Axis) < winConditionAxisOrder(out[j].Axis)
		}
		return winConditionRatingOrder(out[i].Rating) > winConditionRatingOrder(out[j].Rating)
	})
	return out, nil
}

func (r *Repository) queryWinConditionScoreDeltaOutcomes(ctx context.Context, where string, args []any) ([]WinConditionScoreDeltaOutcome, error) {
	teamSelect := winConditionScopedTeamSelect(where)
	paired := `
		SELECT
			t.win AS win,
			t.splitpush_score AS team_splitpush_score,
			o.splitpush_score AS opponent_splitpush_score,
			t.pick_score AS team_pick_score,
			o.pick_score AS opponent_pick_score,
			t.siege_score AS team_siege_score,
			o.siege_score AS opponent_siege_score,
			t.control_score AS team_control_score,
			o.control_score AS opponent_control_score,
			t.teamfight_score AS team_teamfight_score,
			o.teamfight_score AS opponent_teamfight_score
		FROM (` + teamSelect + `) AS t
		INNER JOIN (` + teamSelect + `) AS o
			ON t.match_id = o.match_id
		WHERE t.team_id != o.team_id`
	query := `
		SELECT
			axis,
			delta_bucket,
			toUInt64(count()) AS games,
			toUInt64(sum(win)) AS wins,
			avg(delta) AS avg_delta
		FROM
		(
			SELECT
				tupleElement(axis_tuple, 1) AS axis,
				tupleElement(axis_tuple, 2) - tupleElement(axis_tuple, 3) AS delta,
				multiIf(
					delta <= -8, '<=-8',
					delta <= -4, '-7..-4',
					delta <= -1, '-3..-1',
					delta = 0, '0',
					delta <= 3, '1..3',
					delta <= 7, '4..7',
					'8+'
				) AS delta_bucket,
				win
			FROM
			(
				SELECT
					win,
					arrayJoin([
						tuple('SplitPush', team_splitpush_score, opponent_splitpush_score),
						tuple('Pick', team_pick_score, opponent_pick_score),
						tuple('Siege', team_siege_score, opponent_siege_score),
						tuple('Control', team_control_score, opponent_control_score),
						tuple('TeamFight', team_teamfight_score, opponent_teamfight_score)
					]) AS axis_tuple
				FROM (` + paired + `)
			)
		)
		GROUP BY axis, delta_bucket`
	rows, err := r.db.QueryContext(ctx, query, repeatArgs(args, 2)...)
	if err != nil {
		return nil, fmt.Errorf("query win condition score deltas: %w", err)
	}
	defer rows.Close()
	out := []WinConditionScoreDeltaOutcome{}
	for rows.Next() {
		var row WinConditionScoreDeltaOutcome
		var wins, games uint64
		if err := rows.Scan(&row.Axis, &row.Bucket, &games, &wins, &row.AvgDelta); err != nil {
			return nil, fmt.Errorf("scan win condition score delta: %w", err)
		}
		row.Games = int(games)
		row.Wins = int(wins)
		row.WinRate = winConditionWinRatePercent(wins, games)
		row.Confidence = winConditionConfidencePercent(wins, games)
		row.AvgDelta = winConditionRoundPercent(row.AvgDelta)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Axis != out[j].Axis {
			return winConditionAxisOrder(out[i].Axis) < winConditionAxisOrder(out[j].Axis)
		}
		return winConditionDeltaBucketOrder(out[i].Bucket) < winConditionDeltaBucketOrder(out[j].Bucket)
	})
	return out, nil
}

func (r *Repository) queryWinConditionPrimaryMatchupOutcomes(ctx context.Context, where string, args []any, minGames, limit int) ([]WinConditionPrimaryMatchupOutcome, error) {
	teamSelect := winConditionScopedTeamSelect(where)
	query := `
		SELECT
			t.primary_condition,
			t.primary_rating,
			o.primary_condition AS opponent_primary_condition,
			o.primary_rating AS opponent_primary_rating,
			toUInt64(count()) AS games,
			toUInt64(sum(t.win)) AS wins
		FROM (` + teamSelect + `) AS t
		INNER JOIN (` + teamSelect + `) AS o
			ON t.match_id = o.match_id
		WHERE t.team_id != o.team_id
		GROUP BY t.primary_condition, t.primary_rating, opponent_primary_condition, opponent_primary_rating
		HAVING games >= ?
		ORDER BY games DESC`
	queryArgs := append(repeatArgs(args, 2), minGames)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query win condition primary matchups: %w", err)
	}
	defer rows.Close()
	out := []WinConditionPrimaryMatchupOutcome{}
	for rows.Next() {
		var row WinConditionPrimaryMatchupOutcome
		var wins, games uint64
		if err := rows.Scan(&row.Condition, &row.Rating, &row.OpponentCondition, &row.OpponentRating, &games, &wins); err != nil {
			return nil, fmt.Errorf("scan win condition primary matchup: %w", err)
		}
		row.Games = int(games)
		row.Wins = int(wins)
		row.WinRate = winConditionWinRatePercent(wins, games)
		row.Confidence = winConditionConfidencePercent(wins, games)
		row.Edge = winConditionRoundPercent(row.WinRate - 50)
		row.WilsonLow, row.WilsonHigh = winConditionWilsonIntervalPercent(wins, games)
		row.Direction, row.Signal = winConditionDirectionalSignal(row.WilsonLow, row.WilsonHigh)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Signal != out[j].Signal {
			return out[i].Signal > out[j].Signal
		}
		leftEdge := math.Abs(out[i].Edge)
		rightEdge := math.Abs(out[j].Edge)
		if leftEdge != rightEdge {
			return leftEdge > rightEdge
		}
		if out[i].Games != out[j].Games {
			return out[i].Games > out[j].Games
		}
		if out[i].Condition != out[j].Condition {
			return winConditionAxisOrder(out[i].Condition) < winConditionAxisOrder(out[j].Condition)
		}
		if out[i].OpponentCondition != out[j].OpponentCondition {
			return winConditionAxisOrder(out[i].OpponentCondition) < winConditionAxisOrder(out[j].OpponentCondition)
		}
		return winConditionRatingOrder(out[i].Rating) > winConditionRatingOrder(out[j].Rating)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *Repository) queryWinConditionPrimaryMarginOutcomes(ctx context.Context, where string, args []any) ([]WinConditionPrimaryMarginOutcome, error) {
	query := `
		SELECT
			margin_bucket,
			toUInt64(count()) AS games,
			toUInt64(sum(win)) AS wins,
			avg(primary_margin) AS avg_margin
		FROM
		(
			SELECT
				win,
				primary_margin,
				multiIf(
					primary_margin = 0, 'TIE',
					primary_margin = 1, '1',
					primary_margin <= 3, '2-3',
					primary_margin <= 6, '4-6',
					'7+'
				) AS margin_bucket
			FROM
			(
				SELECT
					win,
					toInt16(arrayElement(arrayReverseSort([splitpush_score, pick_score, siege_score, control_score, teamfight_score]), 1))
						- toInt16(arrayElement(arrayReverseSort([splitpush_score, pick_score, siege_score, control_score, teamfight_score]), 2)) AS primary_margin
				FROM match_team_win_conditions FINAL
				` + where + `
			)
		)
		GROUP BY margin_bucket`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query win condition primary margin outcomes: %w", err)
	}
	defer rows.Close()
	out := []WinConditionPrimaryMarginOutcome{}
	for rows.Next() {
		var row WinConditionPrimaryMarginOutcome
		var wins, games uint64
		if err := rows.Scan(&row.Bucket, &games, &wins, &row.AvgMargin); err != nil {
			return nil, fmt.Errorf("scan win condition primary margin outcome: %w", err)
		}
		row.Games = int(games)
		row.Wins = int(wins)
		row.WinRate = winConditionWinRatePercent(wins, games)
		row.Confidence = winConditionConfidencePercent(wins, games)
		row.AvgMargin = winConditionRoundPercent(row.AvgMargin)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		return winConditionMarginBucketOrder(out[i].Bucket) < winConditionMarginBucketOrder(out[j].Bucket)
	})
	return out, nil
}

func (r *Repository) queryWinConditionSynergyResiduals(ctx context.Context, where string, args []any, primaryMatchups []WinConditionPrimaryMatchupOutcome, filters WinConditionValidationFilters) ([]WinConditionSynergyResidual, error) {
	parents := winConditionSynergyParents(primaryMatchups, filters.SynergyMinParentSignal, filters.SynergyParentLimit)
	if len(parents) == 0 {
		return []WinConditionSynergyResidual{}, nil
	}
	parentByKey := make(map[winConditionPrimaryMatchupKey]WinConditionPrimaryMatchupOutcome, len(parents))
	clauses := make([]string, 0, len(parents))
	queryArgs := repeatArgs(args, 2)
	for _, parent := range parents {
		key := winConditionPrimaryKey(parent)
		parentByKey[key] = parent
		clauses = append(clauses, "(t.primary_condition = ? AND t.primary_rating = ? AND o.primary_condition = ? AND o.primary_rating = ?)")
		queryArgs = append(queryArgs, parent.Condition, parent.Rating, parent.OpponentCondition, parent.OpponentRating)
	}

	teamSelect := winConditionScopedTeamSelect(where)
	query := `
		SELECT
			t.primary_condition,
			t.primary_rating,
			o.primary_condition AS opponent_primary_condition,
			o.primary_rating AS opponent_primary_rating,
			toUInt8(t.win) AS win,
			t.champion_ids AS team_champion_ids,
			o.champion_ids AS opponent_champion_ids
		FROM (` + teamSelect + `) AS t
		INNER JOIN (` + teamSelect + `) AS o
			ON t.match_id = o.match_id
		WHERE t.team_id != o.team_id
			AND (` + strings.Join(clauses, " OR ") + `)`
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query win condition synergy residuals: %w", err)
	}
	defer rows.Close()

	countsByKey := map[winConditionSynergyKey]winConditionSynergyCounts{}
	for rows.Next() {
		var condition, rating, opponentCondition, opponentRating string
		var win uint8
		var teamChampionIDs, opponentChampionIDs []uint16
		if err := rows.Scan(&condition, &rating, &opponentCondition, &opponentRating, &win, &teamChampionIDs, &opponentChampionIDs); err != nil {
			return nil, fmt.Errorf("scan win condition synergy residual: %w", err)
		}
		parent := winConditionPrimaryMatchupKey{
			condition:         condition,
			rating:            rating,
			opponentCondition: opponentCondition,
			opponentRating:    opponentRating,
		}
		if _, ok := parentByKey[parent]; !ok {
			continue
		}
		addWinConditionSynergyPairs(countsByKey, parent, "teammate", teamChampionIDs, win)
		addWinConditionSynergyPairs(countsByKey, parent, "opponent", opponentChampionIDs, win)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]WinConditionSynergyResidual, 0, len(countsByKey))
	for key, counts := range countsByKey {
		if int(counts.games) < filters.SynergyMinGames {
			continue
		}
		parent := parentByKey[key.parent]
		row := WinConditionSynergyResidual{
			PairType:          key.pairType,
			ChampionID1:       key.championID1,
			ChampionID2:       key.championID2,
			Condition:         parent.Condition,
			Rating:            parent.Rating,
			OpponentCondition: parent.OpponentCondition,
			OpponentRating:    parent.OpponentRating,
			ParentGames:       parent.Games,
			ParentWinRate:     parent.WinRate,
			ParentSignal:      parent.Signal,
			Games:             int(counts.games),
			Wins:              int(counts.wins),
			WinRate:           winConditionWinRatePercent(counts.wins, counts.games),
		}
		row.WilsonLow, row.WilsonHigh = winConditionWilsonIntervalPercent(counts.wins, counts.games)
		row.Residual = winConditionRoundPercent(row.WinRate - parent.WinRate)
		row.Direction, row.Signal = winConditionResidualSignal(row.WilsonLow, row.WilsonHigh, parent.WinRate)
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Signal != out[j].Signal {
			return out[i].Signal > out[j].Signal
		}
		leftResidual := math.Abs(out[i].Residual)
		rightResidual := math.Abs(out[j].Residual)
		if leftResidual != rightResidual {
			return leftResidual > rightResidual
		}
		if out[i].Games != out[j].Games {
			return out[i].Games > out[j].Games
		}
		if out[i].PairType != out[j].PairType {
			return out[i].PairType < out[j].PairType
		}
		if out[i].ChampionID1 != out[j].ChampionID1 {
			return out[i].ChampionID1 < out[j].ChampionID1
		}
		return out[i].ChampionID2 < out[j].ChampionID2
	})
	if filters.SynergyLimit > 0 && len(out) > filters.SynergyLimit {
		out = out[:filters.SynergyLimit]
	}
	return out, nil
}

func (r *Repository) queryWinConditionWeakSignalWarnings(ctx context.Context, filters WinConditionValidationFilters, resolvedPatch string) ([]WinConditionWeakSignalWarning, error) {
	patch := resolvedPatch
	if patch == "" {
		patch = filters.Patch
	}
	rows, err := r.QueryWinConditionMetrics(ctx, WinConditionMetricFilters{
		QueueID:    filters.QueueID,
		Patch:      patch,
		Platform:   filters.Platform,
		RankBucket: filters.RankBucket,
	})
	if err != nil {
		return nil, err
	}
	warnings := []WinConditionWeakSignalWarning{}
	for _, row := range rows {
		if row.GameLengthBucket != "ALL" || row.Games < filters.MinGames || row.WinRate < filters.WeakSignalWinRate || !winConditionLowRating(row.TeamRating) {
			continue
		}
		warnings = append(warnings, WinConditionWeakSignalWarning{
			Condition:         row.TeamCondition,
			Rating:            row.TeamRating,
			OpponentCondition: row.OpponentCondition,
			OpponentRating:    row.OpponentRating,
			TeamPrimary:       row.TeamPrimary,
			Games:             row.Games,
			Wins:              row.Wins,
			WinRate:           row.WinRate,
			Confidence:        row.Confidence,
			Note:              "Low-rated strategy row with high winrate; treat as correlation until timeline/objective evidence shows teams actually played through this angle.",
		})
	}
	sort.Slice(warnings, func(i, j int) bool {
		if warnings[i].Confidence != warnings[j].Confidence {
			return warnings[i].Confidence > warnings[j].Confidence
		}
		if warnings[i].Games != warnings[j].Games {
			return warnings[i].Games > warnings[j].Games
		}
		return warnings[i].WinRate > warnings[j].WinRate
	})
	if len(warnings) > filters.Limit {
		warnings = warnings[:filters.Limit]
	}
	return warnings, nil
}

func winConditionScopedTeamSelect(where string) string {
	return `
		SELECT
			match_id,
			team_id,
			win,
			toInt16(splitpush_score) AS splitpush_score,
			toInt16(pick_score) AS pick_score,
			toInt16(siege_score) AS siege_score,
			toInt16(control_score) AS control_score,
			toInt16(teamfight_score) AS teamfight_score,
			primary_condition,
			primary_rating,
			champion_ids
		FROM match_team_win_conditions FINAL
		` + where
}

func winConditionValidationFindings(validation WinConditionValidation) []WinConditionValidationFinding {
	findings := []WinConditionValidationFinding{}
	deltaByAxis := map[string]struct {
		positiveWins  int
		positiveGames int
		negativeWins  int
		negativeGames int
	}{}
	for _, row := range validation.ScoreDeltaOutcomes {
		bucket := deltaByAxis[row.Axis]
		switch row.Bucket {
		case "1..3", "4..7", "8+":
			bucket.positiveWins += row.Wins
			bucket.positiveGames += row.Games
		case "<=-8", "-7..-4", "-3..-1":
			bucket.negativeWins += row.Wins
			bucket.negativeGames += row.Games
		}
		deltaByAxis[row.Axis] = bucket
	}
	axes := make([]string, 0, len(deltaByAxis))
	for axis := range deltaByAxis {
		axes = append(axes, axis)
	}
	sort.Slice(axes, func(i, j int) bool {
		return winConditionAxisOrder(axes[i]) < winConditionAxisOrder(axes[j])
	})
	for _, axis := range axes {
		bucket := deltaByAxis[axis]
		if bucket.positiveGames < validation.MinGames || bucket.negativeGames < validation.MinGames {
			continue
		}
		positiveWR := winConditionWinRatePercent(uint64(bucket.positiveWins), uint64(bucket.positiveGames))
		negativeWR := winConditionWinRatePercent(uint64(bucket.negativeWins), uint64(bucket.negativeGames))
		delta := winConditionRoundPercent(positiveWR - negativeWR)
		severity := "watch"
		summary := fmt.Sprintf("%s score advantages are only %.2f pts above score disadvantages (%d positive-side games, %d negative-side games).", axis, delta, bucket.positiveGames, bucket.negativeGames)
		if delta >= 3 {
			severity = "supportive"
			summary = fmt.Sprintf("%s score advantages are %.2f pts above score disadvantages (%d positive-side games, %d negative-side games).", axis, delta, bucket.positiveGames, bucket.negativeGames)
		} else if delta <= 0 {
			severity = "risk"
			summary = fmt.Sprintf("%s score advantages are not outperforming score disadvantages yet (%.2f pt gap across %d positive-side games and %d negative-side games).", axis, delta, bucket.positiveGames, bucket.negativeGames)
		}
		findings = append(findings, WinConditionValidationFinding{
			Severity: severity,
			Topic:    axis + " score delta",
			Summary:  summary,
		})
	}
	signaledPrimaryRows := 0
	var strongestPrimary WinConditionPrimaryMatchupOutcome
	for _, row := range validation.PrimaryMatchups {
		if row.Signal < 1 {
			continue
		}
		signaledPrimaryRows++
		if strongestPrimary.Signal == 0 || row.Signal > strongestPrimary.Signal {
			strongestPrimary = row
		}
	}
	if signaledPrimaryRows > 0 {
		findings = append(findings, WinConditionValidationFinding{
			Severity: "supportive",
			Topic:    "Primary strategy matchups",
			Summary: fmt.Sprintf(
				"%d returned primary-vs-primary rows have a Wilson interval at least 1 point away from 50%%; strongest signal is %s %s into %s %s at %.2f%% over %d games (%s, %.2f pt signal).",
				signaledPrimaryRows,
				strongestPrimary.Condition,
				strongestPrimary.Rating,
				strongestPrimary.OpponentCondition,
				strongestPrimary.OpponentRating,
				strongestPrimary.WinRate,
				strongestPrimary.Games,
				strongestPrimary.Direction,
				strongestPrimary.Signal,
			),
		})
	} else if len(validation.PrimaryMatchups) > 0 {
		findings = append(findings, WinConditionValidationFinding{
			Severity: "watch",
			Topic:    "Primary strategy matchups",
			Summary:  "No returned primary-vs-primary row has a Wilson interval at least 1 point away from 50%; keep these reads exploratory until more data or stronger segmentation produces stable edges.",
		})
	}
	signaledSynergyRows := 0
	var strongestSynergy WinConditionSynergyResidual
	for _, row := range validation.SynergyResiduals {
		if row.Signal < 1 {
			continue
		}
		signaledSynergyRows++
		if strongestSynergy.Signal == 0 || row.Signal > strongestSynergy.Signal {
			strongestSynergy = row
		}
	}
	if signaledSynergyRows > 0 {
		findings = append(findings, WinConditionValidationFinding{
			Severity: "watch",
			Topic:    "Champion-pair residuals",
			Summary: fmt.Sprintf(
				"%d champion-pair residuals cleared a 1-point Wilson signal against their parent strategy matchup; strongest is %s pair %d/%d in %s %s into %s %s (%+.2f pts over parent, %s, %.2f pt signal).",
				signaledSynergyRows,
				strongestSynergy.PairType,
				strongestSynergy.ChampionID1,
				strongestSynergy.ChampionID2,
				strongestSynergy.Condition,
				strongestSynergy.Rating,
				strongestSynergy.OpponentCondition,
				strongestSynergy.OpponentRating,
				strongestSynergy.Residual,
				strongestSynergy.Direction,
				strongestSynergy.Signal,
			),
		})
	} else if len(validation.SynergyResiduals) > 0 {
		findings = append(findings, WinConditionValidationFinding{
			Severity: "watch",
			Topic:    "Champion-pair residuals",
			Summary:  fmt.Sprintf("%d champion-pair residual rows met the %d-game threshold, but none cleared a 1-point Wilson signal against their parent strategy matchup.", len(validation.SynergyResiduals), validation.SynergyMinGames),
		})
	} else if len(validation.PrimaryMatchups) > 0 {
		hasSynergyParents := false
		for _, row := range validation.PrimaryMatchups {
			if row.Signal >= validation.SynergyMinParentSignal {
				hasSynergyParents = true
				break
			}
		}
		if hasSynergyParents {
			findings = append(findings, WinConditionValidationFinding{
				Severity: "watch",
				Topic:    "Champion-pair residuals",
				Summary:  fmt.Sprintf("No champion-pair residual rows met the %d-game threshold inside the returned high-signal primary matchups. That means no obvious repeated pair artifact yet, not proof that synergy is absent.", validation.SynergyMinGames),
			})
		}
	}
	if len(validation.WeakSignalWarnings) > 0 {
		findings = append(findings, WinConditionValidationFinding{
			Severity: "watch",
			Topic:    "Low-rated high-winrate rows",
			Summary:  fmt.Sprintf("%d low-rated strategy rows cleared the high-winrate warning threshold; these are likely correlation-first reads until timeline validation exists.", len(validation.WeakSignalWarnings)),
		})
	}
	if len(findings) == 0 {
		findings = append(findings, WinConditionValidationFinding{
			Severity: "watch",
			Topic:    "Sample size",
			Summary:  "No validation finding cleared the minimum sample threshold yet. Keep collecting or lower minGames for exploratory local analysis.",
		})
	}
	return findings
}

func winConditionAxisOrder(axis string) int {
	switch axis {
	case "SplitPush":
		return 1
	case "Pick":
		return 2
	case "Siege":
		return 3
	case "Control":
		return 4
	case "TeamFight":
		return 5
	default:
		return 99
	}
}

func winConditionRatingOrder(rating string) int {
	switch strings.ToUpper(strings.TrimSpace(rating)) {
	case "D-":
		return 1
	case "D":
		return 2
	case "D+":
		return 3
	case "C-":
		return 4
	case "C":
		return 5
	case "C+":
		return 6
	case "B-":
		return 7
	case "B":
		return 8
	case "B+":
		return 9
	case "A-":
		return 10
	case "A":
		return 11
	case "A+":
		return 12
	case "S-":
		return 13
	case "S":
		return 14
	case "S+":
		return 15
	default:
		return 0
	}
}

func winConditionDeltaBucketOrder(bucket string) int {
	switch bucket {
	case "<=-8":
		return 1
	case "-7..-4":
		return 2
	case "-3..-1":
		return 3
	case "0":
		return 4
	case "1..3":
		return 5
	case "4..7":
		return 6
	case "8+":
		return 7
	default:
		return 99
	}
}

func winConditionMarginBucketOrder(bucket string) int {
	switch bucket {
	case "TIE":
		return 1
	case "1":
		return 2
	case "2-3":
		return 3
	case "4-6":
		return 4
	case "7+":
		return 5
	default:
		return 99
	}
}

func winConditionLowRating(rating string) bool {
	return winConditionRatingOrder(rating) > 0 && winConditionRatingOrder(rating) <= winConditionRatingOrder("C")
}

func winConditionPrimaryKey(row WinConditionPrimaryMatchupOutcome) winConditionPrimaryMatchupKey {
	return winConditionPrimaryMatchupKey{
		condition:         row.Condition,
		rating:            row.Rating,
		opponentCondition: row.OpponentCondition,
		opponentRating:    row.OpponentRating,
	}
}

func winConditionSynergyParents(rows []WinConditionPrimaryMatchupOutcome, minSignal float64, limit int) []WinConditionPrimaryMatchupOutcome {
	out := make([]WinConditionPrimaryMatchupOutcome, 0, len(rows))
	for _, row := range rows {
		if row.Signal < minSignal {
			continue
		}
		out = append(out, row)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func addWinConditionSynergyPairs(countsByKey map[winConditionSynergyKey]winConditionSynergyCounts, parent winConditionPrimaryMatchupKey, pairType string, championIDs []uint16, win uint8) {
	for _, pair := range winConditionChampionPairs(championIDs) {
		key := winConditionSynergyKey{
			parent:      parent,
			pairType:    pairType,
			championID1: pair[0],
			championID2: pair[1],
		}
		counts := countsByKey[key]
		counts.games++
		if win > 0 {
			counts.wins++
		}
		countsByKey[key] = counts
	}
}

func winConditionChampionPairs(championIDs []uint16) [][2]uint16 {
	if len(championIDs) < 2 {
		return nil
	}
	ids := append([]uint16(nil), championIDs...)
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	pairs := make([][2]uint16, 0, len(ids)*(len(ids)-1)/2)
	seen := map[[2]uint16]bool{}
	for i := 0; i < len(ids); i++ {
		if ids[i] == 0 {
			continue
		}
		for j := i + 1; j < len(ids); j++ {
			if ids[j] == 0 || ids[i] == ids[j] {
				continue
			}
			pair := [2]uint16{ids[i], ids[j]}
			if seen[pair] {
				continue
			}
			seen[pair] = true
			pairs = append(pairs, pair)
		}
	}
	return pairs
}

func winConditionWilsonIntervalPercent(wins, games uint64) (float64, float64) {
	if games == 0 {
		return 0, 0
	}
	const z = 1.96
	n := float64(games)
	p := float64(wins) / n
	z2 := z * z
	denominator := 1 + z2/n
	center := (p + z2/(2*n)) / denominator
	margin := (z / denominator) * math.Sqrt((p*(1-p)+z2/(4*n))/n)
	low := math.Max(0, center-margin) * 100
	high := math.Min(1, center+margin) * 100
	return winConditionRoundPercent(low), winConditionRoundPercent(high)
}

func winConditionDirectionalSignal(wilsonLow, wilsonHigh float64) (string, float64) {
	switch {
	case wilsonLow > 50:
		return "favorable", winConditionRoundPercent(wilsonLow - 50)
	case wilsonHigh < 50:
		return "unfavorable", winConditionRoundPercent(50 - wilsonHigh)
	default:
		return "mixed", 0
	}
}

func winConditionResidualSignal(wilsonLow, wilsonHigh, parentWinRate float64) (string, float64) {
	switch {
	case wilsonLow > parentWinRate:
		return "overperforming", winConditionRoundPercent(wilsonLow - parentWinRate)
	case wilsonHigh < parentWinRate:
		return "underperforming", winConditionRoundPercent(parentWinRate - wilsonHigh)
	default:
		return "mixed", 0
	}
}
