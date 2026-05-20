package clickhouse

import (
	"context"
	"strings"
	"time"
)

type AccountAlias struct {
	PUUID    string
	Platform string
	GameName string
	TagLine  string
	LastSeen time.Time
}

func (r *Repository) EnsureRuntimeSchema(ctx context.Context) error {
	statements := []string{`
		CREATE TABLE IF NOT EXISTS riot_account_aliases
		(
			puuid String,
			platform LowCardinality(String),
			game_name String,
			game_name_normalized String,
			tag_line String,
			last_seen_at DateTime,
			updated_at DateTime64(3) DEFAULT now64(3)
		)
		ENGINE = ReplacingMergeTree(updated_at)
		ORDER BY (platform, game_name_normalized, tag_line, puuid)
	`, `
		CREATE TABLE IF NOT EXISTS riot_request_events
		(
			route LowCardinality(String),
			source LowCardinality(String),
			request_count UInt16,
			happened_at DateTime64(3),
			inserted_at DateTime64(3) DEFAULT now64(3)
		)
		ENGINE = MergeTree
		ORDER BY (route, happened_at, source)
		TTL toDateTime(happened_at) + INTERVAL 1 DAY DELETE
	`, `
		CREATE TABLE IF NOT EXISTS match_team_win_conditions
		(
			patch LowCardinality(String),
			platform LowCardinality(String),
			queue_id UInt16,
			match_id String,
			team_id UInt16,
			win UInt8,
			duration_seconds UInt32,
			champion_ids Array(UInt16),
			rank_bucket LowCardinality(String),
			splitpush_score UInt8,
			pick_score UInt8,
			siege_score UInt8,
			control_score UInt8,
			teamfight_score UInt8,
			splitpush_rating LowCardinality(String),
			pick_rating LowCardinality(String),
			siege_rating LowCardinality(String),
			control_rating LowCardinality(String),
			teamfight_rating LowCardinality(String),
			primary_condition LowCardinality(String),
			primary_rating LowCardinality(String),
			profile_patch LowCardinality(String),
			compiled_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(compiled_at)
		ORDER BY (patch, platform, queue_id, match_id, team_id)
	`, `
		CREATE TABLE IF NOT EXISTS patch_win_condition_metrics
		(
			patch LowCardinality(String),
			platform LowCardinality(String),
			queue_id UInt16,
			rank_bucket LowCardinality(String),
			team_condition LowCardinality(String),
			team_rating LowCardinality(String),
			opponent_condition LowCardinality(String),
			opponent_rating LowCardinality(String),
			team_primary UInt8,
			game_length_bucket LowCardinality(String),
			wins UInt64,
			games UInt64,
			win_rate_percent Float64,
			confidence_percent Float64,
			compiled_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(compiled_at)
		ORDER BY
		(
			patch,
			platform,
			queue_id,
			rank_bucket,
			team_condition,
			team_rating,
			opponent_condition,
			opponent_rating,
			team_primary,
			game_length_bucket
		)
	`, `
		ALTER TABLE patch_win_condition_metrics ADD COLUMN IF NOT EXISTS win_rate_percent Float64 AFTER games
	`, `
		ALTER TABLE patch_win_condition_metrics ADD COLUMN IF NOT EXISTS confidence_percent Float64 AFTER win_rate_percent
	`, `
		CREATE TABLE IF NOT EXISTS timeline_skill_events
		(
			match_id String,
			platform LowCardinality(String),
			patch LowCardinality(String),
			queue_id UInt16,
			timestamp_ms UInt32,
			participant_id UInt8,
			skill_slot UInt8,
			skill_order UInt8,
			level_up_type LowCardinality(String),
			ingested_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(ingested_at)
		ORDER BY (match_id, participant_id, skill_order)
	`, `
		CREATE TABLE IF NOT EXISTS champion_bans
		(
			match_id String,
			platform LowCardinality(String),
			patch LowCardinality(String),
			queue_id UInt16,
			team_id UInt16,
			champion_id UInt16,
			pick_turn UInt8,
			ingested_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(ingested_at)
		ORDER BY (match_id, team_id, pick_turn, champion_id)
	`, `
		CREATE TABLE IF NOT EXISTS champion_skill_analytics
		(
			patch LowCardinality(String),
			platform LowCardinality(String),
			queue_id UInt16,
			champion_id UInt16,
			role LowCardinality(String),
			rank_bucket LowCardinality(String),
			skill_order_signature String,
			wins UInt64,
			games UInt64,
			compiled_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(compiled_at)
		ORDER BY
		(
			patch,
			platform,
			queue_id,
			champion_id,
			role,
			rank_bucket,
			skill_order_signature
		)
	`, `
		CREATE TABLE IF NOT EXISTS champion_ban_analytics
		(
			patch LowCardinality(String),
			platform LowCardinality(String),
			queue_id UInt16,
			champion_id UInt16,
			bans UInt64,
			games UInt64,
			compiled_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(compiled_at)
		ORDER BY (patch, platform, queue_id, champion_id)
	`, `
		CREATE TABLE IF NOT EXISTS item_slot_analytics
		(
			patch LowCardinality(String),
			platform LowCardinality(String),
			queue_id UInt16,
			item_context LowCardinality(String),
			champion_id UInt16,
			role LowCardinality(String),
			opponent_champion_id UInt16,
			rank_bucket LowCardinality(String),
			item_slot UInt8,
			item_id UInt32,
			wins UInt64,
			games UInt64,
			compiled_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(compiled_at)
		ORDER BY
		(
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
		)
	`}
	for _, statement := range statements {
		if _, err := r.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) UpsertAccountAlias(ctx context.Context, alias AccountAlias) error {
	if alias.PUUID == "" || alias.Platform == "" || alias.GameName == "" || alias.TagLine == "" {
		return nil
	}
	if alias.LastSeen.IsZero() {
		alias.LastSeen = time.Now()
	}
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO riot_account_aliases
			(puuid, platform, game_name, game_name_normalized, tag_line, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		alias.PUUID,
		alias.Platform,
		alias.GameName,
		normalizeAliasName(alias.GameName),
		alias.TagLine,
		alias.LastSeen,
	)
	return err
}

func (r *Repository) ResolveAccountAliases(ctx context.Context, platform, gameName string, limit int) ([]AccountAlias, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			puuid,
			platform,
			argMax(game_name, updated_at) AS game_name,
			tag_line,
			max(last_seen_at) AS last_seen_at
		FROM riot_account_aliases FINAL
		WHERE platform = ? AND game_name_normalized = ?
		GROUP BY puuid, platform, tag_line
		ORDER BY last_seen_at DESC
		LIMIT ?`,
		platform,
		normalizeAliasName(gameName),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	aliases := []AccountAlias{}
	for rows.Next() {
		var alias AccountAlias
		if err := rows.Scan(&alias.PUUID, &alias.Platform, &alias.GameName, &alias.TagLine, &alias.LastSeen); err != nil {
			return nil, err
		}
		aliases = append(aliases, alias)
	}
	return aliases, rows.Err()
}

func (r *Repository) SearchAccountAliases(ctx context.Context, platform, gameNamePrefix string, limit int) ([]AccountAlias, error) {
	if limit <= 0 {
		limit = 6
	}
	normalizedPrefix := normalizeAliasName(gameNamePrefix)
	if normalizedPrefix == "" {
		return nil, nil
	}
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			puuid,
			platform,
			argMax(game_name, updated_at) AS game_name,
			tag_line,
			max(last_seen_at) AS last_seen_at,
			game_name_normalized
		FROM riot_account_aliases FINAL
		WHERE platform = ? AND startsWith(game_name_normalized, ?)
		GROUP BY puuid, platform, tag_line, game_name_normalized
		ORDER BY (game_name_normalized = ?) DESC, last_seen_at DESC
		LIMIT ?`,
		platform,
		normalizedPrefix,
		normalizedPrefix,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	aliases := []AccountAlias{}
	for rows.Next() {
		var alias AccountAlias
		var normalizedName string
		if err := rows.Scan(&alias.PUUID, &alias.Platform, &alias.GameName, &alias.TagLine, &alias.LastSeen, &normalizedName); err != nil {
			return nil, err
		}
		aliases = append(aliases, alias)
	}
	return aliases, rows.Err()
}

func (r *Repository) FetchAccountAliasCandidates(ctx context.Context, platform string, limit int) ([]AccountAlias, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			puuid,
			platform,
			count() AS participant_rows
		FROM participants FINAL
		WHERE platform = ?
			AND puuid != ''
			AND puuid NOT IN
			(
				SELECT puuid
				FROM riot_account_aliases FINAL
				WHERE platform = ?
			)
		GROUP BY puuid, platform
		ORDER BY participant_rows DESC
		LIMIT ?`,
		platform, platform, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	aliases := []AccountAlias{}
	for rows.Next() {
		var alias AccountAlias
		var participantRows uint64
		if err := rows.Scan(&alias.PUUID, &alias.Platform, &participantRows); err != nil {
			return nil, err
		}
		aliases = append(aliases, alias)
	}
	return aliases, rows.Err()
}

func normalizeAliasName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
