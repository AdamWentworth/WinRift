package clickhouse

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type WinConditionValidationFilters struct {
	QueueID           uint16
	Patch             string
	Platform          string
	RankBucket        string
	MinGames          int
	WeakSignalWinRate float64
	Limit             int
}

type WinConditionValidation struct {
	Patch                 string                              `json:"patch"`
	Platform              string                              `json:"platform"`
	QueueID               uint16                              `json:"queueId"`
	RankBucket            string                              `json:"rankBucket"`
	MinGames              int                                 `json:"minGames"`
	WeakSignalWinRate     float64                             `json:"weakSignalWinRate"`
	Teams                 int                                 `json:"teams"`
	Matches               int                                 `json:"matches"`
	RatingOutcomes        []WinConditionRatingOutcome         `json:"ratingOutcomes"`
	ScoreDeltaOutcomes    []WinConditionScoreDeltaOutcome     `json:"scoreDeltaOutcomes"`
	PrimaryMatchups       []WinConditionPrimaryMatchupOutcome `json:"primaryMatchups"`
	PrimaryMarginOutcomes []WinConditionPrimaryMarginOutcome  `json:"primaryMarginOutcomes"`
	WeakSignalWarnings    []WinConditionWeakSignalWarning     `json:"weakSignalWarnings"`
	Findings              []WinConditionValidationFinding     `json:"findings"`
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

type WinConditionValidationFinding struct {
	Severity string `json:"severity"`
	Topic    string `json:"topic"`
	Summary  string `json:"summary"`
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
		Patch:             diagnostics.Patch,
		Platform:          diagnostics.Platform,
		QueueID:           diagnostics.QueueID,
		RankBucket:        diagnostics.RankBucket,
		MinGames:          normalized.MinGames,
		WeakSignalWinRate: normalized.WeakSignalWinRate,
		Teams:             diagnostics.Teams,
		Matches:           diagnostics.Matches,
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
		ORDER BY games DESC, abs((wins / games) - 0.5) DESC
		LIMIT ?`
	queryArgs := append(repeatArgs(args, 2), minGames, limit)
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
		out = append(out, row)
	}
	return out, rows.Err()
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
			primary_rating
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
