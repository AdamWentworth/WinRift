package clickhouse

import (
	"context"
	"database/sql"
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

type SummonerAccountSnapshot struct {
	PUUID         string
	Platform      string
	SummonerID    string
	AccountID     string
	ProfileIconID uint32
	SummonerLevel uint64
	FetchedAt     time.Time
	ExpiresAt     time.Time
}

func (r *Repository) CachedChampionPageBundle(ctx context.Context, cacheKey string) ([]byte, bool, error) {
	cacheKey = strings.TrimSpace(cacheKey)
	if cacheKey == "" {
		return nil, false, nil
	}
	var payload string
	err := r.db.QueryRowContext(
		ctx,
		`SELECT payload_json
		FROM champion_page_bundle_cache FINAL
		WHERE cache_key = ? AND expires_at > now()
		ORDER BY compiled_at DESC
		LIMIT 1`,
		cacheKey,
	).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return []byte(payload), true, nil
}

func (r *Repository) StoreChampionPageBundle(ctx context.Context, cacheKey string, body []byte, ttl time.Duration) error {
	cacheKey = strings.TrimSpace(cacheKey)
	if cacheKey == "" || len(body) == 0 || ttl <= 0 {
		return nil
	}
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO champion_page_bundle_cache (cache_key, payload_json, expires_at, compiled_at) VALUES (?, ?, ?, now())`,
		cacheKey,
		string(body),
		time.Now().Add(ttl),
	)
	return err
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
		CREATE TABLE IF NOT EXISTS summoner_account_snapshots
		(
			puuid String,
			platform LowCardinality(String),
			summoner_id String,
			account_id String,
			profile_icon_id UInt32,
			summoner_level UInt64,
			fetched_at DateTime,
			expires_at DateTime,
			updated_at DateTime64(3) DEFAULT now64(3)
		)
		ENGINE = ReplacingMergeTree(updated_at)
		ORDER BY (platform, puuid)
	`, `
		CREATE TABLE IF NOT EXISTS summoner_identity_summary
		(
			platform LowCardinality(String),
			puuid String,
			game_name String,
			tag_line String,
			profile_icon_id UInt32,
			summoner_level UInt64,
			last_seen_at DateTime,
			compiled_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(compiled_at)
		ORDER BY (platform, puuid)
	`, `
		CREATE TABLE IF NOT EXISTS summoner_profile_summary
		(
			platform LowCardinality(String),
			queue_id UInt16,
			puuid String,
			games UInt64,
			wins UInt64,
			kills UInt64,
			deaths UInt64,
			assists UInt64,
			first_seen_at DateTime,
			last_seen_at DateTime,
			compiled_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(compiled_at)
		ORDER BY (platform, queue_id, puuid)
	`, `
		CREATE TABLE IF NOT EXISTS summoner_champion_summary
		(
			platform LowCardinality(String),
			queue_id UInt16,
			puuid String,
			champion_id UInt16,
			games UInt64,
			wins UInt64,
			kills UInt64,
			deaths UInt64,
			assists UInt64,
			first_seen_at DateTime,
			last_seen_at DateTime,
			compiled_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(compiled_at)
		ORDER BY (platform, queue_id, puuid, champion_id)
	`, `
		CREATE TABLE IF NOT EXISTS summoner_champion_role_summary
		(
			platform LowCardinality(String),
			queue_id UInt16,
			puuid String,
			champion_id UInt16,
			role LowCardinality(String),
			games UInt64,
			wins UInt64,
			kills UInt64,
			deaths UInt64,
			assists UInt64,
			first_seen_at DateTime,
			last_seen_at DateTime,
			compiled_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(compiled_at)
		ORDER BY (platform, queue_id, puuid, champion_id, role)
	`, `
		CREATE TABLE IF NOT EXISTS summoner_recent_match_summary
		(
			platform LowCardinality(String),
			queue_id UInt16,
			puuid String,
			match_id String,
			patch LowCardinality(String),
			champion_id UInt16,
			role LowCardinality(String),
			win UInt8,
			kills UInt16,
			deaths UInt16,
			assists UInt16,
			game_start_timestamp UInt64,
			duration_seconds UInt32,
			compiled_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(compiled_at)
		ORDER BY (platform, queue_id, puuid, game_start_timestamp, match_id)
	`, `
		CREATE TABLE IF NOT EXISTS summoner_build_summary
		(
			platform LowCardinality(String),
			queue_id UInt16,
			puuid String,
			champion_id UInt16,
			role LowCardinality(String),
			final_items_signature String,
			core2_signature String,
			core3_signature String,
			rune_signature String,
			spell_signature String,
			games UInt64,
			wins UInt64,
			kills UInt64,
			deaths UInt64,
			assists UInt64,
			compiled_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(compiled_at)
		ORDER BY
		(
			platform,
			queue_id,
			puuid,
			champion_id,
			role,
			final_items_signature,
			core2_signature,
			core3_signature,
			rune_signature,
			spell_signature
		)
	`, `
		CREATE TABLE IF NOT EXISTS champion_page_bundle_cache
		(
			cache_key String,
			payload_json String,
			expires_at DateTime,
			compiled_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(compiled_at)
		ORDER BY cache_key
		TTL expires_at DELETE
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
		CREATE TABLE IF NOT EXISTS champion_guide_summary_analytics
		(
			patch LowCardinality(String),
			platform LowCardinality(String),
			queue_id UInt16,
			champion_id UInt16,
			role LowCardinality(String),
			rank_bucket LowCardinality(String),
			wins UInt64,
			games UInt64,
			kills UInt64,
			deaths UInt64,
			assists UInt64,
			gold_earned_sum UInt64,
			cs_sum UInt64,
			damage_dealt_to_champions_sum UInt64,
			damage_taken_sum UInt64,
			damage_self_mitigated_sum UInt64,
			damage_dealt_to_objectives_sum UInt64,
			damage_dealt_to_structures_sum UInt64,
			vision_score_sum UInt64,
			time_ccing_others_sum UInt64,
			team_utility_sum UInt64,
			structure_takedowns_sum UInt64,
			objective_takedowns_sum UInt64,
			total_time_spent_dead_sum UInt64,
			time_played_sum UInt64,
			kill_participation_sum Float64,
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
			rank_bucket
		)
	`, `
		CREATE TABLE IF NOT EXISTS champion_guide_scope_analytics
		(
			patch LowCardinality(String),
			platform LowCardinality(String),
			queue_id UInt16,
			role LowCardinality(String),
			rank_bucket LowCardinality(String),
			participant_samples UInt64,
			match_count UInt64,
			compiled_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(compiled_at)
		ORDER BY
		(
			patch,
			platform,
			queue_id,
			role,
			rank_bucket
		)
	`, `
		CREATE TABLE IF NOT EXISTS champion_matchup_analytics
		(
			patch LowCardinality(String),
			platform LowCardinality(String),
			queue_id UInt16,
			champion_id UInt16,
			role LowCardinality(String),
			opponent_champion_id UInt16,
			rank_bucket LowCardinality(String),
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
			opponent_champion_id,
			rank_bucket
		)
	`, `
		CREATE TABLE IF NOT EXISTS champion_signature_analytics
		(
			patch LowCardinality(String),
			platform LowCardinality(String),
			queue_id UInt16,
			champion_id UInt16,
			role LowCardinality(String),
			rank_bucket LowCardinality(String),
			signature_type LowCardinality(String),
			signature String,
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
			signature_type,
			signature
		)
	`, `
		CREATE TABLE IF NOT EXISTS champion_build_variant_analytics
		(
			patch LowCardinality(String),
			platform LowCardinality(String),
			queue_id UInt16,
			champion_id UInt16,
			role LowCardinality(String),
			rank_bucket LowCardinality(String),
			variant_key String,
			variant_label String,
			variant_tags Array(String),
			core2_signature String,
			core3_signature String,
			final_items_signature String,
			rune_signature String,
			spell_signature String,
			skill_order_signature String,
			skill_order_wins UInt64,
			skill_order_games UInt64,
			wins UInt64,
			games UInt64,
			build_count UInt64,
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
			variant_key
		)
	`, `
		CREATE TABLE IF NOT EXISTS participant_performance
		(
			match_id String,
			platform LowCardinality(String),
			patch LowCardinality(String),
			queue_id UInt16,
			participant_id UInt8,
			champion_id UInt16,
			role LowCardinality(String),
			gold_earned UInt32,
			gold_spent UInt32,
			total_minions_killed UInt32,
			neutral_minions_killed UInt32,
			total_damage_dealt_to_champions UInt32,
			physical_damage_dealt_to_champions UInt32,
			magic_damage_dealt_to_champions UInt32,
			true_damage_dealt_to_champions UInt32,
			total_damage_taken UInt32,
			damage_self_mitigated UInt32,
			damage_dealt_to_objectives UInt32,
			damage_dealt_to_turrets UInt32,
			damage_dealt_to_buildings UInt32,
			vision_score UInt32,
			wards_placed UInt32,
			wards_killed UInt32,
			detector_wards_placed UInt32,
			time_ccing_others UInt32,
			total_heal UInt32,
			total_heals_on_teammates UInt32,
			total_damage_shielded_on_teammates UInt32,
			turret_takedowns UInt32,
			inhibitor_takedowns UInt32,
			dragon_kills UInt32,
			baron_kills UInt32,
			objectives_stolen UInt32,
			total_time_spent_dead UInt32,
			time_played UInt32,
			ingested_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(ingested_at)
		ORDER BY (match_id, participant_id)
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
	`, `
		CREATE TABLE IF NOT EXISTS starting_loadout_analytics
		(
			patch LowCardinality(String),
			platform LowCardinality(String),
			queue_id UInt16,
			item_context LowCardinality(String),
			champion_id UInt16,
			role LowCardinality(String),
			opponent_champion_id UInt16,
			rank_bucket LowCardinality(String),
			item_signature String,
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
			item_signature
		)
	`}
	for _, statement := range statements {
		if _, err := r.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return r.ensureParticipantPerformanceColumns(ctx)
}

func (r *Repository) ensureParticipantPerformanceColumns(ctx context.Context) error {
	columns := []struct {
		name     string
		dataType string
		after    string
	}{
		{"gold_earned", "UInt32", "assists"},
		{"gold_spent", "UInt32", "gold_earned"},
		{"total_minions_killed", "UInt32", "gold_spent"},
		{"neutral_minions_killed", "UInt32", "total_minions_killed"},
		{"total_damage_dealt_to_champions", "UInt32", "neutral_minions_killed"},
		{"physical_damage_dealt_to_champions", "UInt32", "total_damage_dealt_to_champions"},
		{"magic_damage_dealt_to_champions", "UInt32", "physical_damage_dealt_to_champions"},
		{"true_damage_dealt_to_champions", "UInt32", "magic_damage_dealt_to_champions"},
		{"total_damage_taken", "UInt32", "true_damage_dealt_to_champions"},
		{"damage_self_mitigated", "UInt32", "total_damage_taken"},
		{"damage_dealt_to_objectives", "UInt32", "damage_self_mitigated"},
		{"damage_dealt_to_turrets", "UInt32", "damage_dealt_to_objectives"},
		{"damage_dealt_to_buildings", "UInt32", "damage_dealt_to_turrets"},
		{"vision_score", "UInt32", "damage_dealt_to_buildings"},
		{"wards_placed", "UInt32", "vision_score"},
		{"wards_killed", "UInt32", "wards_placed"},
		{"detector_wards_placed", "UInt32", "wards_killed"},
		{"time_ccing_others", "UInt32", "detector_wards_placed"},
		{"total_heal", "UInt32", "time_ccing_others"},
		{"total_heals_on_teammates", "UInt32", "total_heal"},
		{"total_damage_shielded_on_teammates", "UInt32", "total_heals_on_teammates"},
		{"turret_takedowns", "UInt32", "total_damage_shielded_on_teammates"},
		{"inhibitor_takedowns", "UInt32", "turret_takedowns"},
		{"dragon_kills", "UInt32", "inhibitor_takedowns"},
		{"baron_kills", "UInt32", "dragon_kills"},
		{"objectives_stolen", "UInt32", "baron_kills"},
		{"total_time_spent_dead", "UInt32", "objectives_stolen"},
		{"time_played", "UInt32", "total_time_spent_dead"},
	}
	for _, table := range []string{"participants", "participant_matchups"} {
		for _, column := range columns {
			statement := "ALTER TABLE " + table + " ADD COLUMN IF NOT EXISTS " + column.name + " " + column.dataType + " AFTER " + column.after
			if _, err := r.db.ExecContext(ctx, statement); err != nil {
				return err
			}
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

func (r *Repository) UpsertSummonerAccountSnapshot(ctx context.Context, snapshot SummonerAccountSnapshot) error {
	if snapshot.PUUID == "" || snapshot.Platform == "" {
		return nil
	}
	if snapshot.FetchedAt.IsZero() {
		snapshot.FetchedAt = time.Now()
	}
	if snapshot.ExpiresAt.IsZero() {
		snapshot.ExpiresAt = snapshot.FetchedAt.Add(7 * 24 * time.Hour)
	}
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO summoner_account_snapshots
			(puuid, platform, summoner_id, account_id, profile_icon_id, summoner_level, fetched_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshot.PUUID,
		snapshot.Platform,
		snapshot.SummonerID,
		snapshot.AccountID,
		snapshot.ProfileIconID,
		snapshot.SummonerLevel,
		snapshot.FetchedAt,
		snapshot.ExpiresAt,
	)
	return err
}

func (r *Repository) LatestSummonerAccountSnapshot(ctx context.Context, platform, puuid string) (SummonerAccountSnapshot, error) {
	var snapshot SummonerAccountSnapshot
	err := r.db.QueryRowContext(
		ctx,
		`SELECT
			puuid,
			platform,
			summoner_id,
			account_id,
			profile_icon_id,
			summoner_level,
			fetched_at,
			expires_at
		FROM summoner_account_snapshots FINAL
		WHERE platform = ? AND puuid = ?
		ORDER BY fetched_at DESC
		LIMIT 1`,
		platform,
		puuid,
	).Scan(
		&snapshot.PUUID,
		&snapshot.Platform,
		&snapshot.SummonerID,
		&snapshot.AccountID,
		&snapshot.ProfileIconID,
		&snapshot.SummonerLevel,
		&snapshot.FetchedAt,
		&snapshot.ExpiresAt,
	)
	return snapshot, err
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
