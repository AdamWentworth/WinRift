package clickhouse

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"winrift/core/internal/analytics"
	"winrift/core/internal/winconditions"
)

type TeamCompositionFilters struct {
	QueueID  uint16
	Patch    string
	Platform string
	MaxRows  int
}

type TeamCompositionRow struct {
	MatchID         string
	Platform        string
	Patch           string
	QueueID         uint16
	TeamID          uint16
	Win             bool
	DurationSeconds uint32
	ChampionIDs     []uint16
	RankBuckets     []string
}

type WinConditionMetricFilters struct {
	QueueID    uint16
	Patch      string
	Platform   string
	RankBucket string
}

type WinConditionMetricRow struct {
	Patch             string
	Platform          string
	QueueID           uint16
	RankBucket        string
	TeamCondition     string
	TeamRating        string
	OpponentCondition string
	OpponentRating    string
	TeamPrimary       uint8
	GameLengthBucket  string
	Wins              int
	Games             int
	WinRate           float64
	Confidence        float64
}

type WinConditionDiagnosticsFilters struct {
	QueueID    uint16
	Patch      string
	Platform   string
	RankBucket string
}

type WinConditionDiagnostics struct {
	Patch                string                             `json:"patch"`
	Platform             string                             `json:"platform"`
	QueueID              uint16                             `json:"queueId"`
	RankBucket           string                             `json:"rankBucket"`
	Teams                int                                `json:"teams"`
	Matches              int                                `json:"matches"`
	AxisRatings          []WinConditionAxisRatingDiagnostic `json:"axisRatings"`
	PrimaryConditions    []WinConditionPrimaryDiagnostic    `json:"primaryConditions"`
	PrimaryMarginBuckets []WinConditionMarginDiagnostic     `json:"primaryMarginBuckets"`
}

type WinConditionAxisRatingDiagnostic struct {
	Axis     string  `json:"axis"`
	Rating   string  `json:"rating"`
	Teams    int     `json:"teams"`
	AvgScore float64 `json:"avgScore"`
	MinScore int     `json:"minScore"`
	MaxScore int     `json:"maxScore"`
}

type WinConditionPrimaryDiagnostic struct {
	Condition string  `json:"condition"`
	Rating    string  `json:"rating"`
	Teams     int     `json:"teams"`
	AvgMargin float64 `json:"avgMargin"`
}

type WinConditionMarginDiagnostic struct {
	Bucket string `json:"bucket"`
	Teams  int    `json:"teams"`
}

type winConditionTeamRecord struct {
	row     TeamCompositionRow
	profile winconditions.TeamProfile
}

type winConditionMetricKey struct {
	patch             string
	platform          string
	queueID           uint16
	rankBucket        string
	teamCondition     string
	teamRating        string
	opponentCondition string
	opponentRating    string
	teamPrimary       uint8
	gameLengthBucket  string
}

type winConditionMetricCounts struct {
	wins  uint64
	games uint64
}

func (r *Repository) QueryTeamCompositions(ctx context.Context, filters TeamCompositionFilters) ([]TeamCompositionRow, error) {
	queueID := filters.QueueID
	if queueID == 0 {
		queueID = 420
	}
	patch := strings.TrimSpace(filters.Patch)
	platform := strings.ToUpper(strings.TrimSpace(filters.Platform))
	query := `
		WITH participant_ranked AS
		(
			SELECT
				p.match_id,
				p.platform,
				p.patch,
				p.queue_id,
				p.team_id,
				p.champion_id,
				p.win,
				multiIf(
					s.snapshot_rank_bucket NOT IN ('', 'UNKNOWN'), s.snapshot_rank_bucket,
					p.rank_bucket
				) AS rank_value
			FROM participants AS p FINAL
			LEFT JOIN
			(
				SELECT
					platform,
					puuid,
					argMax(rank_bucket, fetched_at) AS snapshot_rank_bucket
				FROM summoner_rank_snapshots FINAL
				WHERE queue_type = 'RANKED_SOLO_5x5'
				`
	args := []any{}
	if platform != "" {
		query += " AND platform = ?"
		args = append(args, platform)
	}
	query += `
				GROUP BY platform, puuid
			) AS s
				ON s.platform = p.platform AND s.puuid = p.puuid
			WHERE p.queue_id = ?`
	args = append(args, queueID)
	if patch != "" {
		query += " AND p.patch = ?"
		args = append(args, patch)
	}
	if platform != "" {
		query += " AND p.platform = ?"
		args = append(args, platform)
	}
	query += `
		)
		SELECT
			pr.match_id,
			any(pr.platform) AS platform,
			any(pr.patch) AS patch,
			any(pr.queue_id) AS queue_id,
			pr.team_id,
			toUInt8(max(pr.win)) AS win,
			any(rm.duration_seconds) AS duration_seconds,
			groupArray(toUInt16(pr.champion_id)) AS champion_ids,
			groupArray(rank_value) AS rank_buckets
		FROM participant_ranked AS pr
		INNER JOIN
		(
			SELECT match_id, duration_seconds, game_end_timestamp
			FROM raw_matches FINAL
			WHERE queue_id = ?`
	args = append(args, queueID)
	if patch != "" {
		query += " AND patch = ?"
		args = append(args, patch)
	}
	if platform != "" {
		query += " AND platform = ?"
		args = append(args, platform)
	}
	query += `
		) AS rm ON rm.match_id = pr.match_id
		GROUP BY pr.match_id, pr.team_id
		HAVING length(champion_ids) = 5
		ORDER BY max(rm.game_end_timestamp) DESC`
	if filters.MaxRows > 0 {
		query += " LIMIT ?"
		args = append(args, filters.MaxRows)
	}
	query += `
		SETTINGS
			join_algorithm = 'grace_hash',
			max_bytes_before_external_group_by = 268435456,
			max_bytes_before_external_sort = 268435456`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query team compositions: %w", err)
	}
	defer rows.Close()
	out := []TeamCompositionRow{}
	for rows.Next() {
		var row TeamCompositionRow
		var win uint8
		if err := rows.Scan(&row.MatchID, &row.Platform, &row.Patch, &row.QueueID, &row.TeamID, &win, &row.DurationSeconds, &row.ChampionIDs, &row.RankBuckets); err != nil {
			return nil, fmt.Errorf("scan team composition: %w", err)
		}
		row.Win = win > 0
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) RefreshWinConditionMetrics(ctx context.Context, patch, platform string, queueID uint16) error {
	return r.refreshWinConditionMetricsForPlatforms(ctx, patch, []string{platform}, queueID, true)
}

func (r *Repository) RefreshWinConditionMetricsForPlatforms(ctx context.Context, patch string, platforms []string, queueID uint16) error {
	return r.refreshWinConditionMetricsForPlatforms(ctx, patch, platforms, queueID, true)
}

func (r *Repository) RefreshCombinedWinConditionMetrics(ctx context.Context, patch string, queueID uint16) error {
	patch = strings.TrimSpace(patch)
	if patch == "" {
		return fmt.Errorf("patch is required")
	}
	if queueID == 0 {
		queueID = 420
	}
	return r.refreshAllPlatformWinConditionMetrics(ctx, patch, queueID, time.Now().UTC().Truncate(time.Second))
}

func (r *Repository) refreshWinConditionMetricsForPlatforms(ctx context.Context, patch string, platforms []string, queueID uint16, refreshAll bool) error {
	patch = strings.TrimSpace(patch)
	platforms = normalizeWinConditionPlatforms(platforms)
	if patch == "" || len(platforms) == 0 {
		return fmt.Errorf("patch and platform are required")
	}
	if queueID == 0 {
		queueID = 420
	}
	catalog, err := winconditions.LoadCatalog()
	if err != nil {
		return err
	}
	analyzer := winconditions.NewAnalyzer(catalog)
	compiledAt := time.Now().UTC().Truncate(time.Second)
	for _, platform := range platforms {
		rows, err := r.QueryTeamCompositions(ctx, TeamCompositionFilters{Patch: patch, Platform: platform, QueueID: queueID})
		if err != nil {
			return err
		}
		records := make([]winConditionTeamRecord, 0, len(rows))
		for _, row := range rows {
			record := winConditionTeamRecord{
				row:     row,
				profile: analyzer.TeamProfile(row.ChampionIDs),
			}
			records = append(records, record)
		}
		if err := r.insertWinConditionTeamRecords(ctx, records, analyzer.CatalogPatch(), compiledAt); err != nil {
			return err
		}
		metrics := aggregateWinConditionRecords(records)
		if err := r.insertWinConditionMetrics(ctx, metrics, compiledAt); err != nil {
			return err
		}
		if err := r.deleteOldWinConditionMetrics(ctx, patch, platform, queueID, compiledAt); err != nil {
			return err
		}
	}
	if refreshAll {
		return r.refreshAllPlatformWinConditionMetrics(ctx, patch, queueID, compiledAt)
	}
	return nil
}

func normalizeWinConditionPlatforms(platforms []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		platform = strings.ToUpper(strings.TrimSpace(platform))
		if platform == "" || platform == "ALL" || seen[platform] {
			continue
		}
		seen[platform] = true
		out = append(out, platform)
	}
	sort.Strings(out)
	return out
}

func (r *Repository) QueryWinConditionMetrics(ctx context.Context, filters WinConditionMetricFilters) ([]WinConditionMetricRow, error) {
	queueID := filters.QueueID
	if queueID == 0 {
		queueID = 420
	}
	patch := strings.TrimSpace(filters.Patch)
	platform := strings.ToUpper(strings.TrimSpace(filters.Platform))
	if platform == "" {
		platform = "ALL"
	}
	rankBucket := strings.ToUpper(strings.TrimSpace(filters.RankBucket))
	if rankBucket == "" {
		rankBucket = "ALL"
	}
	args := []any{queueID, platform, rankBucket}
	query := `
		SELECT
			patch,
			platform,
			queue_id,
			rank_bucket,
			team_condition,
			team_rating,
			opponent_condition,
			opponent_rating,
			team_primary,
			game_length_bucket,
			wins,
			games,
			win_rate_percent,
			confidence_percent
		FROM patch_win_condition_metrics FINAL
		WHERE queue_id = ? AND platform = ? AND rank_bucket = ?`
	if patch != "" {
		query += " AND patch = ?"
		args = append(args, patch)
	} else {
		query += `
			AND patch =
			(
				SELECT patch
				FROM patch_win_condition_metrics FINAL
				WHERE queue_id = ? AND platform = ? AND rank_bucket = ? AND patch != 'ALL'
				GROUP BY patch
				ORDER BY
					toUInt16OrZero(arrayElement(splitByChar('.', patch), 1)) DESC,
					toUInt16OrZero(arrayElement(splitByChar('.', patch), 2)) DESC
				LIMIT 1
			)`
		args = append(args, queueID, platform, rankBucket)
	}
	query += `
		ORDER BY games DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query win condition metrics: %w", err)
	}
	defer rows.Close()
	out := []WinConditionMetricRow{}
	for rows.Next() {
		var row WinConditionMetricRow
		if err := rows.Scan(&row.Patch, &row.Platform, &row.QueueID, &row.RankBucket, &row.TeamCondition, &row.TeamRating, &row.OpponentCondition, &row.OpponentRating, &row.TeamPrimary, &row.GameLengthBucket, &row.Wins, &row.Games, &row.WinRate, &row.Confidence); err != nil {
			return nil, fmt.Errorf("scan win condition metric: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) QueryWinConditionDiagnostics(ctx context.Context, filters WinConditionDiagnosticsFilters) (WinConditionDiagnostics, error) {
	queueID := filters.QueueID
	if queueID == 0 {
		queueID = 420
	}
	platform := strings.ToUpper(strings.TrimSpace(filters.Platform))
	rankBucket := strings.ToUpper(strings.TrimSpace(filters.RankBucket))
	if rankBucket == "ALL" {
		rankBucket = ""
	}
	where, args := winConditionDiagnosticsWhere(queueID, filters.Patch, platform, rankBucket)
	diagnostics := WinConditionDiagnostics{
		Platform:   "ALL",
		QueueID:    queueID,
		RankBucket: "ALL",
	}
	if platform != "" {
		diagnostics.Platform = platform
	}
	if rankBucket != "" {
		diagnostics.RankBucket = rankBucket
	}
	if err := r.queryWinConditionDiagnosticsSummary(ctx, where, args, &diagnostics); err != nil {
		return WinConditionDiagnostics{}, err
	}
	axisRatings, err := r.queryWinConditionAxisRatings(ctx, where, args)
	if err != nil {
		return WinConditionDiagnostics{}, err
	}
	diagnostics.AxisRatings = axisRatings
	primaryConditions, err := r.queryWinConditionPrimaryConditions(ctx, where, args)
	if err != nil {
		return WinConditionDiagnostics{}, err
	}
	diagnostics.PrimaryConditions = primaryConditions
	marginBuckets, err := r.queryWinConditionPrimaryMargins(ctx, where, args)
	if err != nil {
		return WinConditionDiagnostics{}, err
	}
	diagnostics.PrimaryMarginBuckets = marginBuckets
	return diagnostics, nil
}

func winConditionDiagnosticsWhere(queueID uint16, patch, platform, rankBucket string) (string, []any) {
	where := "WHERE queue_id = ?"
	args := []any{queueID}
	patch = strings.TrimSpace(patch)
	if patch != "" {
		where += " AND patch = ?"
		args = append(args, patch)
	} else {
		where += `
			AND patch =
			(
				SELECT patch
				FROM match_team_win_conditions FINAL
				WHERE queue_id = ?
				GROUP BY patch
				ORDER BY
					toUInt16OrZero(arrayElement(splitByChar('.', patch), 1)) DESC,
					toUInt16OrZero(arrayElement(splitByChar('.', patch), 2)) DESC
				LIMIT 1
			)`
		args = append(args, queueID)
	}
	if platform != "" {
		where += " AND platform = ?"
		args = append(args, platform)
	}
	if rankBucket != "" {
		where += " AND rank_bucket = ?"
		args = append(args, rankBucket)
	}
	return where, args
}

func (r *Repository) queryWinConditionDiagnosticsSummary(ctx context.Context, where string, args []any, diagnostics *WinConditionDiagnostics) error {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			any(patch) AS resolved_patch,
			toUInt64(count()) AS teams,
			toUInt64(uniqExact(match_id)) AS matches
		FROM match_team_win_conditions FINAL
		`+where, args...)
	var teams, matches uint64
	if err := row.Scan(&diagnostics.Patch, &teams, &matches); err != nil {
		return fmt.Errorf("query win condition diagnostics summary: %w", err)
	}
	diagnostics.Teams = int(teams)
	diagnostics.Matches = int(matches)
	return nil
}

func (r *Repository) queryWinConditionAxisRatings(ctx context.Context, where string, args []any) ([]WinConditionAxisRatingDiagnostic, error) {
	selects := []string{
		"SELECT 'SplitPush' AS axis, splitpush_rating AS rating, toUInt8(splitpush_score) AS score FROM match_team_win_conditions FINAL " + where,
		"SELECT 'Pick' AS axis, pick_rating AS rating, toUInt8(pick_score) AS score FROM match_team_win_conditions FINAL " + where,
		"SELECT 'Siege' AS axis, siege_rating AS rating, toUInt8(siege_score) AS score FROM match_team_win_conditions FINAL " + where,
		"SELECT 'Control' AS axis, control_rating AS rating, toUInt8(control_score) AS score FROM match_team_win_conditions FINAL " + where,
		"SELECT 'TeamFight' AS axis, teamfight_rating AS rating, toUInt8(teamfight_score) AS score FROM match_team_win_conditions FINAL " + where,
	}
	query := `
		SELECT
			axis,
			rating,
			toUInt64(count()) AS teams,
			avg(score) AS avg_score,
			toUInt8(min(score)) AS min_score,
			toUInt8(max(score)) AS max_score
		FROM (` + strings.Join(selects, " UNION ALL ") + `)
		GROUP BY axis, rating
		ORDER BY axis, avg_score DESC, teams DESC`
	rows, err := r.db.QueryContext(ctx, query, repeatArgs(args, len(selects))...)
	if err != nil {
		return nil, fmt.Errorf("query win condition axis ratings: %w", err)
	}
	defer rows.Close()
	out := []WinConditionAxisRatingDiagnostic{}
	for rows.Next() {
		var row WinConditionAxisRatingDiagnostic
		var teams uint64
		var minScore, maxScore uint8
		if err := rows.Scan(&row.Axis, &row.Rating, &teams, &row.AvgScore, &minScore, &maxScore); err != nil {
			return nil, fmt.Errorf("scan win condition axis rating: %w", err)
		}
		row.Teams = int(teams)
		row.AvgScore = winConditionRoundPercent(row.AvgScore)
		row.MinScore = int(minScore)
		row.MaxScore = int(maxScore)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) queryWinConditionPrimaryConditions(ctx context.Context, where string, args []any) ([]WinConditionPrimaryDiagnostic, error) {
	query := `
		SELECT
			primary_condition,
			primary_rating,
			toUInt64(count()) AS teams,
			avg(primary_margin) AS avg_margin
		FROM
		(
			SELECT
				primary_condition,
				primary_rating,
				arrayElement(arrayReverseSort([splitpush_score, pick_score, siege_score, control_score, teamfight_score]), 1)
					- arrayElement(arrayReverseSort([splitpush_score, pick_score, siege_score, control_score, teamfight_score]), 2) AS primary_margin
			FROM match_team_win_conditions FINAL
			` + where + `
		)
		GROUP BY primary_condition, primary_rating
		ORDER BY teams DESC, avg_margin DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query win condition primary diagnostics: %w", err)
	}
	defer rows.Close()
	out := []WinConditionPrimaryDiagnostic{}
	for rows.Next() {
		var row WinConditionPrimaryDiagnostic
		var teams uint64
		if err := rows.Scan(&row.Condition, &row.Rating, &teams, &row.AvgMargin); err != nil {
			return nil, fmt.Errorf("scan win condition primary diagnostic: %w", err)
		}
		row.Teams = int(teams)
		row.AvgMargin = winConditionRoundPercent(row.AvgMargin)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) queryWinConditionPrimaryMargins(ctx context.Context, where string, args []any) ([]WinConditionMarginDiagnostic, error) {
	query := `
		SELECT
			multiIf(primary_margin = 0, 'TIE', primary_margin = 1, '1', primary_margin <= 3, '2-3', primary_margin <= 6, '4-6', '7+') AS margin_bucket,
			toUInt64(count()) AS teams
		FROM
		(
			SELECT
				arrayElement(arrayReverseSort([splitpush_score, pick_score, siege_score, control_score, teamfight_score]), 1)
					- arrayElement(arrayReverseSort([splitpush_score, pick_score, siege_score, control_score, teamfight_score]), 2) AS primary_margin
			FROM match_team_win_conditions FINAL
			` + where + `
		)
		GROUP BY margin_bucket
		ORDER BY
			multiIf(margin_bucket = 'TIE', 0, margin_bucket = '1', 1, margin_bucket = '2-3', 2, margin_bucket = '4-6', 3, 4)`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query win condition primary margin diagnostics: %w", err)
	}
	defer rows.Close()
	out := []WinConditionMarginDiagnostic{}
	for rows.Next() {
		var row WinConditionMarginDiagnostic
		var teams uint64
		if err := rows.Scan(&row.Bucket, &teams); err != nil {
			return nil, fmt.Errorf("scan win condition primary margin diagnostic: %w", err)
		}
		row.Teams = int(teams)
		out = append(out, row)
	}
	return out, rows.Err()
}

func repeatArgs(args []any, count int) []any {
	out := make([]any, 0, len(args)*count)
	for i := 0; i < count; i++ {
		out = append(out, args...)
	}
	return out
}

func (r *Repository) deleteOldWinConditionMetrics(ctx context.Context, patch, platform string, queueID uint16, compiledAt time.Time) error {
	statements := []string{
		`ALTER TABLE match_team_win_conditions DELETE WHERE patch = ? AND platform = ? AND queue_id = ? AND compiled_at < ? SETTINGS mutations_sync = 2`,
		`ALTER TABLE patch_win_condition_metrics DELETE WHERE patch = ? AND platform = ? AND queue_id = ? AND compiled_at < ? SETTINGS mutations_sync = 2`,
	}
	for _, statement := range statements {
		if _, err := r.db.ExecContext(ctx, statement, patch, platform, queueID, compiledAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) insertWinConditionTeamRecords(ctx context.Context, records []winConditionTeamRecord, profilePatch string, compiledAt time.Time) error {
	const insertPrefix = `
		INSERT INTO match_team_win_conditions
		(patch, platform, queue_id, match_id, team_id, win, duration_seconds, champion_ids, rank_bucket, splitpush_score, pick_score, siege_score, control_score, teamfight_score, splitpush_rating, pick_rating, siege_rating, control_rating, teamfight_rating, primary_condition, primary_rating, profile_patch, compiled_at)
		VALUES `
	values := make([]string, 0, len(records))
	for _, record := range records {
		profile := record.profile
		row := record.row
		ratings := profile.Ratings
		win := uint8(0)
		if row.Win {
			win = 1
		}
		values = append(values, fmt.Sprintf(
			"(%s, %s, %d, %s, %d, %d, %d, %s, %s, %d, %d, %d, %d, %d, %s, %s, %s, %s, %s, %s, %s, %s, %s)",
			clickhouseQuote(row.Patch),
			clickhouseQuote(row.Platform),
			row.QueueID,
			clickhouseQuote(row.MatchID),
			row.TeamID,
			win,
			row.DurationSeconds,
			uint16ArraySQL(row.ChampionIDs),
			clickhouseQuote(teamRankBucket(row.RankBuckets)),
			uint8(profile.Scores.SplitPush),
			uint8(profile.Scores.Pick),
			uint8(profile.Scores.Siege),
			uint8(profile.Scores.Control),
			uint8(profile.Scores.TeamFight),
			clickhouseQuote(ratings["splitpush"]),
			clickhouseQuote(ratings["pick"]),
			clickhouseQuote(ratings["siege"]),
			clickhouseQuote(ratings["control"]),
			clickhouseQuote(ratings["teamfight"]),
			clickhouseQuote(profile.PrimaryCondition),
			clickhouseQuote(profile.PrimaryRating),
			clickhouseQuote(profilePatch),
			clickhouseDateTimeSQL(compiledAt),
		))
	}
	return r.execValueInsertChunks(ctx, insertPrefix, values, 1000)
}

func aggregateWinConditionRecords(records []winConditionTeamRecord) map[winConditionMetricKey]winConditionMetricCounts {
	byMatch := make(map[string][]winConditionTeamRecord)
	for _, record := range records {
		byMatch[record.row.MatchID] = append(byMatch[record.row.MatchID], record)
	}
	out := map[winConditionMetricKey]winConditionMetricCounts{}
	for _, teams := range byMatch {
		if len(teams) != 2 {
			continue
		}
		addWinConditionRecordMetrics(out, teams[0], teams[1])
		addWinConditionRecordMetrics(out, teams[1], teams[0])
	}
	return out
}

func addWinConditionRecordMetrics(metrics map[winConditionMetricKey]winConditionMetricCounts, team, opponent winConditionTeamRecord) {
	win := uint64(0)
	if team.row.Win {
		win = 1
	}
	rankBuckets := []string{teamRankBucket(team.row.RankBuckets), "ALL"}
	gameLengthBuckets := []string{winConditionGameLengthBucket(team.row.DurationSeconds), "ALL"}
	for _, axis := range team.profile.Axes {
		teamModes := []uint8{2}
		if axis.Label == team.profile.PrimaryCondition {
			teamModes = append(teamModes, 1)
		}
		for _, opponentAxis := range opponent.profile.Axes {
			opponentModes := []uint8{2}
			if opponentAxis.Label == opponent.profile.PrimaryCondition {
				opponentModes = append(opponentModes, 1)
			}
			for _, rankBucket := range rankBuckets {
				for _, gameLengthBucket := range gameLengthBuckets {
					for _, teamMode := range teamModes {
						for _, opponentMode := range opponentModes {
							key := winConditionMetricKey{
								patch:             team.row.Patch,
								platform:          team.row.Platform,
								queueID:           team.row.QueueID,
								rankBucket:        rankBucket,
								teamCondition:     axis.Label,
								teamRating:        axis.Rating,
								opponentCondition: opponentAxis.Label,
								opponentRating:    opponentAxis.Rating,
								teamPrimary:       winConditionPairPrimaryMode(teamMode, opponentMode),
								gameLengthBucket:  gameLengthBucket,
							}
							counts := metrics[key]
							counts.games++
							counts.wins += win
							metrics[key] = counts
						}
					}
				}
			}
		}
	}
}

func (r *Repository) insertWinConditionMetrics(ctx context.Context, metrics map[winConditionMetricKey]winConditionMetricCounts, compiledAt time.Time) error {
	const insertPrefix = `
		INSERT INTO patch_win_condition_metrics
		(patch, platform, queue_id, rank_bucket, team_condition, team_rating, opponent_condition, opponent_rating, team_primary, game_length_bucket, wins, games, win_rate_percent, confidence_percent, compiled_at)
		VALUES `
	keys := make([]winConditionMetricKey, 0, len(metrics))
	for key := range metrics {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left, right := keys[i], keys[j]
		if left.patch != right.patch {
			return left.patch < right.patch
		}
		if left.platform != right.platform {
			return left.platform < right.platform
		}
		if left.rankBucket != right.rankBucket {
			return left.rankBucket < right.rankBucket
		}
		if left.teamCondition != right.teamCondition {
			return left.teamCondition < right.teamCondition
		}
		if left.teamRating != right.teamRating {
			return left.teamRating < right.teamRating
		}
		if left.opponentCondition != right.opponentCondition {
			return left.opponentCondition < right.opponentCondition
		}
		if left.opponentRating != right.opponentRating {
			return left.opponentRating < right.opponentRating
		}
		if left.teamPrimary != right.teamPrimary {
			return left.teamPrimary < right.teamPrimary
		}
		return left.gameLengthBucket < right.gameLengthBucket
	})
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		counts := metrics[key]
		values = append(values, fmt.Sprintf(
			"(%s, %s, %d, %s, %s, %s, %s, %s, %d, %s, %d, %d, %s, %s, %s)",
			clickhouseQuote(key.patch),
			clickhouseQuote(key.platform),
			key.queueID,
			clickhouseQuote(key.rankBucket),
			clickhouseQuote(key.teamCondition),
			clickhouseQuote(key.teamRating),
			clickhouseQuote(key.opponentCondition),
			clickhouseQuote(key.opponentRating),
			key.teamPrimary,
			clickhouseQuote(key.gameLengthBucket),
			counts.wins,
			counts.games,
			winConditionFloatSQL(winConditionWinRatePercent(counts.wins, counts.games)),
			winConditionFloatSQL(winConditionConfidencePercent(counts.wins, counts.games)),
			clickhouseDateTimeSQL(compiledAt),
		))
	}
	return r.execValueInsertChunks(ctx, insertPrefix, values, 1000)
}

func (r *Repository) refreshAllPlatformWinConditionMetrics(ctx context.Context, patch string, queueID uint16, compiledAt time.Time) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			rank_bucket,
			team_condition,
			team_rating,
			opponent_condition,
			opponent_rating,
			team_primary,
			game_length_bucket,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games
		FROM patch_win_condition_metrics FINAL
		WHERE patch = ? AND platform != 'ALL' AND queue_id = ?
		GROUP BY
			rank_bucket,
			team_condition,
			team_rating,
			opponent_condition,
			opponent_rating,
			team_primary,
			game_length_bucket
		ORDER BY games DESC`, patch, queueID)
	if err != nil {
		return err
	}
	defer rows.Close()

	values := []string{}
	for rows.Next() {
		var key winConditionMetricKey
		var counts winConditionMetricCounts
		if err := rows.Scan(&key.rankBucket, &key.teamCondition, &key.teamRating, &key.opponentCondition, &key.opponentRating, &key.teamPrimary, &key.gameLengthBucket, &counts.wins, &counts.games); err != nil {
			return err
		}
		key.patch = patch
		key.platform = "ALL"
		key.queueID = queueID
		values = append(values, fmt.Sprintf(
			"(%s, %s, %d, %s, %s, %s, %s, %s, %d, %s, %d, %d, %s, %s, %s)",
			clickhouseQuote(key.patch),
			clickhouseQuote(key.platform),
			key.queueID,
			clickhouseQuote(key.rankBucket),
			clickhouseQuote(key.teamCondition),
			clickhouseQuote(key.teamRating),
			clickhouseQuote(key.opponentCondition),
			clickhouseQuote(key.opponentRating),
			key.teamPrimary,
			clickhouseQuote(key.gameLengthBucket),
			counts.wins,
			counts.games,
			winConditionFloatSQL(winConditionWinRatePercent(counts.wins, counts.games)),
			winConditionFloatSQL(winConditionConfidencePercent(counts.wins, counts.games)),
			clickhouseDateTimeSQL(compiledAt),
		))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := r.execValueInsertChunks(ctx, `
		INSERT INTO patch_win_condition_metrics
		(patch, platform, queue_id, rank_bucket, team_condition, team_rating, opponent_condition, opponent_rating, team_primary, game_length_bucket, wins, games, win_rate_percent, confidence_percent, compiled_at)
		VALUES `, values, 1000); err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `ALTER TABLE patch_win_condition_metrics DELETE WHERE patch = ? AND platform = 'ALL' AND queue_id = ? AND compiled_at < ? SETTINGS mutations_sync = 2`, patch, queueID, compiledAt)
	return err
}

func (r *Repository) execValueInsertChunks(ctx context.Context, insertPrefix string, values []string, chunkSize int) error {
	if len(values) == 0 {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	for start := 0; start < len(values); start += chunkSize {
		end := start + chunkSize
		if end > len(values) {
			end = len(values)
		}
		if _, err := r.db.ExecContext(ctx, insertPrefix+strings.Join(values[start:end], ",")); err != nil {
			return err
		}
	}
	return nil
}

func clickhouseQuote(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(value)
	return "'" + escaped + "'"
}

func clickhouseDateTimeSQL(value time.Time) string {
	if value.IsZero() {
		value = time.Now().UTC()
	}
	return "toDateTime(" + clickhouseQuote(value.UTC().Format("2006-01-02 15:04:05")) + ")"
}

func uint16ArraySQL(values []uint16) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatUint(uint64(value), 10))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func winConditionFloatSQL(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func teamRankBucket(rankBuckets []string) string {
	counts := map[string]int{}
	for _, bucket := range rankBuckets {
		normalized := strings.ToUpper(strings.TrimSpace(bucket))
		if normalized == "" || normalized == "UNKNOWN" {
			continue
		}
		counts[normalized]++
	}
	if len(counts) == 0 {
		return "UNKNOWN"
	}
	bestBucket := "UNKNOWN"
	bestCount := -1
	bestRank := -1
	for bucket, count := range counts {
		rank := teamRankBucketOrder(bucket)
		if count > bestCount || (count == bestCount && rank > bestRank) {
			bestBucket = bucket
			bestCount = count
			bestRank = rank
		}
	}
	return bestBucket
}

func teamRankBucketOrder(bucket string) int {
	switch bucket {
	case "IRON":
		return 1
	case "BRONZE":
		return 2
	case "SILVER":
		return 3
	case "GOLD":
		return 4
	case "PLATINUM":
		return 5
	case "EMERALD":
		return 6
	case "DIAMOND":
		return 7
	case "MASTER":
		return 8
	case "GRANDMASTER":
		return 9
	case "CHALLENGER":
		return 10
	default:
		return 0
	}
}

func winConditionGameLengthBucket(durationSeconds uint32) string {
	switch {
	case durationSeconds < 20*60:
		return "0-20"
	case durationSeconds < 25*60:
		return "20-25"
	case durationSeconds < 30*60:
		return "25-30"
	case durationSeconds < 35*60:
		return "30-35"
	default:
		return "35+"
	}
}

func winConditionPairPrimaryMode(teamMode, opponentMode uint8) uint8 {
	return teamMode*10 + opponentMode
}

func winConditionWinRatePercent(wins, games uint64) float64 {
	if games == 0 {
		return 0
	}
	return winConditionRoundPercent(float64(wins) / float64(games) * 100)
}

func winConditionConfidencePercent(wins, games uint64) float64 {
	return winConditionRoundPercent(analytics.WilsonLowerBound(int(wins), int(games), 1.96) * 100)
}

func winConditionRoundPercent(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
