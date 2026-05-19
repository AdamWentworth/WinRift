package clickhouse

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"winrift/services/core/internal/winconditions"
)

type TeamCompositionFilters struct {
	QueueID uint16
	Patch   string
	MaxRows int
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
	TeamPrimary       bool
	GameLengthBucket  string
	Wins              int
	Games             int
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
	teamPrimary       bool
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
				GROUP BY platform, puuid
			) AS s
				ON s.platform = p.platform AND s.puuid = p.puuid
			WHERE p.queue_id = ?`
	args := []any{queueID}
	if strings.TrimSpace(filters.Patch) != "" {
		query += " AND p.patch = ?"
		args = append(args, strings.TrimSpace(filters.Patch))
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
		INNER JOIN raw_matches AS rm FINAL ON rm.match_id = pr.match_id
		GROUP BY pr.match_id, pr.team_id
		HAVING length(champion_ids) = 5
		ORDER BY max(rm.game_end_timestamp) DESC`
	if filters.MaxRows > 0 {
		query += " LIMIT ?"
		args = append(args, filters.MaxRows)
	}

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
	patch = strings.TrimSpace(patch)
	platform = strings.TrimSpace(platform)
	if patch == "" || platform == "" {
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
	rows, err := r.QueryTeamCompositions(ctx, TeamCompositionFilters{Patch: patch, QueueID: queueID})
	if err != nil {
		return err
	}
	filteredRows := make([]TeamCompositionRow, 0, len(rows))
	for _, row := range rows {
		if row.Platform == platform {
			filteredRows = append(filteredRows, row)
		}
	}
	if err := r.deleteWinConditionMetrics(ctx, patch, platform, queueID); err != nil {
		return err
	}
	records := make([]winConditionTeamRecord, 0, len(filteredRows))
	for _, row := range filteredRows {
		record := winConditionTeamRecord{
			row:     row,
			profile: analyzer.TeamProfile(row.ChampionIDs),
		}
		records = append(records, record)
		if err := r.insertWinConditionTeamRecord(ctx, record, analyzer.CatalogPatch()); err != nil {
			return err
		}
	}
	metrics := aggregateWinConditionRecords(records)
	return r.insertWinConditionMetrics(ctx, metrics)
}

func (r *Repository) QueryWinConditionMetrics(ctx context.Context, filters WinConditionMetricFilters) ([]WinConditionMetricRow, error) {
	queueID := filters.QueueID
	if queueID == 0 {
		queueID = 420
	}
	patchExpr := "'ALL'"
	platformExpr := "'ALL'"
	rankBucketExpr := "'ALL'"
	args := []any{queueID}
	query := `
		SELECT
			%s AS patch_bucket,
			%s AS platform_bucket,
			queue_id,
			%s AS rank_bucket,
			team_condition,
			team_rating,
			opponent_condition,
			opponent_rating,
			team_primary,
			game_length_bucket,
			toUInt64(sum(wins)) AS wins,
			toUInt64(sum(games)) AS games
		FROM patch_win_condition_metrics FINAL
		WHERE queue_id = ?`
	if strings.TrimSpace(filters.Patch) != "" {
		patchExpr = "patch"
		query += " AND patch = ?"
		args = append(args, strings.TrimSpace(filters.Patch))
	}
	if strings.TrimSpace(filters.RankBucket) != "" {
		rankBucketExpr = "rank_bucket"
		query += " AND rank_bucket = ?"
		args = append(args, strings.ToUpper(strings.TrimSpace(filters.RankBucket)))
	}
	query = fmt.Sprintf(query, patchExpr, platformExpr, rankBucketExpr)
	query += `
		GROUP BY
			patch_bucket,
			platform_bucket,
			queue_id,
			rank_bucket,
			team_condition,
			team_rating,
			opponent_condition,
			opponent_rating,
			team_primary,
			game_length_bucket
		ORDER BY games DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query win condition metrics: %w", err)
	}
	defer rows.Close()
	out := []WinConditionMetricRow{}
	for rows.Next() {
		var row WinConditionMetricRow
		var primary uint8
		if err := rows.Scan(&row.Patch, &row.Platform, &row.QueueID, &row.RankBucket, &row.TeamCondition, &row.TeamRating, &row.OpponentCondition, &row.OpponentRating, &primary, &row.GameLengthBucket, &row.Wins, &row.Games); err != nil {
			return nil, fmt.Errorf("scan win condition metric: %w", err)
		}
		row.TeamPrimary = primary > 0
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) deleteWinConditionMetrics(ctx context.Context, patch, platform string, queueID uint16) error {
	statements := []string{
		`ALTER TABLE match_team_win_conditions DELETE WHERE patch = ? AND platform = ? AND queue_id = ? SETTINGS mutations_sync = 2`,
		`ALTER TABLE patch_win_condition_metrics DELETE WHERE patch = ? AND platform = ? AND queue_id = ? SETTINGS mutations_sync = 2`,
	}
	for _, statement := range statements {
		if _, err := r.db.ExecContext(ctx, statement, patch, platform, queueID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) insertWinConditionTeamRecord(ctx context.Context, record winConditionTeamRecord, profilePatch string) error {
	profile := record.profile
	row := record.row
	ratings := profile.Ratings
	win := uint8(0)
	if row.Win {
		win = 1
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO match_team_win_conditions
		(patch, platform, queue_id, match_id, team_id, win, duration_seconds, champion_ids, rank_bucket, splitpush_score, pick_score, siege_score, control_score, teamfight_score, splitpush_rating, pick_rating, siege_rating, control_rating, teamfight_rating, primary_condition, primary_rating, profile_patch)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.Patch,
		row.Platform,
		row.QueueID,
		row.MatchID,
		row.TeamID,
		win,
		row.DurationSeconds,
		row.ChampionIDs,
		teamRankBucket(row.RankBuckets),
		uint8(profile.Scores.SplitPush),
		uint8(profile.Scores.Pick),
		uint8(profile.Scores.Siege),
		uint8(profile.Scores.Control),
		uint8(profile.Scores.TeamFight),
		ratings["splitpush"],
		ratings["pick"],
		ratings["siege"],
		ratings["control"],
		ratings["teamfight"],
		profile.PrimaryCondition,
		profile.PrimaryRating,
		profilePatch,
	)
	return err
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
	for _, axis := range team.profile.Axes {
		key := winConditionMetricKey{
			patch:             team.row.Patch,
			platform:          team.row.Platform,
			queueID:           team.row.QueueID,
			rankBucket:        teamRankBucket(team.row.RankBuckets),
			teamCondition:     axis.Label,
			teamRating:        axis.Rating,
			opponentCondition: opponent.profile.PrimaryCondition,
			opponentRating:    opponent.profile.PrimaryRating,
			teamPrimary:       axis.Label == team.profile.PrimaryCondition,
			gameLengthBucket:  winConditionGameLengthBucket(team.row.DurationSeconds),
		}
		counts := metrics[key]
		counts.games++
		counts.wins += win
		metrics[key] = counts
	}
}

func (r *Repository) insertWinConditionMetrics(ctx context.Context, metrics map[winConditionMetricKey]winConditionMetricCounts) error {
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
			return left.teamPrimary
		}
		return left.gameLengthBucket < right.gameLengthBucket
	})
	for _, key := range keys {
		counts := metrics[key]
		primary := uint8(0)
		if key.teamPrimary {
			primary = 1
		}
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO patch_win_condition_metrics
			(patch, platform, queue_id, rank_bucket, team_condition, team_rating, opponent_condition, opponent_rating, team_primary, game_length_bucket, wins, games)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			key.patch,
			key.platform,
			key.queueID,
			key.rankBucket,
			key.teamCondition,
			key.teamRating,
			key.opponentCondition,
			key.opponentRating,
			primary,
			key.gameLengthBucket,
			counts.wins,
			counts.games,
		); err != nil {
			return err
		}
	}
	return nil
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
