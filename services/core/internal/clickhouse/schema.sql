CREATE DATABASE IF NOT EXISTS winrift;

CREATE TABLE IF NOT EXISTS winrift.collector_frontier
(
    puuid String,
    platform LowCardinality(String),
    source LowCardinality(String),
    source_detail String,
    first_seen_at DateTime,
    last_checked_at Nullable(DateTime),
    next_check_at DateTime,
    priority Int16,
    matches_seen UInt64,
    matches_inserted UInt64,
    matches_skipped UInt64,
    errors UInt64,
    requests_used UInt64,
    attempts UInt32,
    status LowCardinality(String),
    updated_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (platform, puuid);

CREATE TABLE IF NOT EXISTS winrift.summoner_rank_snapshots
(
    puuid String,
    platform LowCardinality(String),
    queue_type LowCardinality(String),
    tier LowCardinality(String),
    division LowCardinality(String),
    league_points Int16,
    wins UInt32,
    losses UInt32,
    rank_bucket LowCardinality(String),
    fetched_at DateTime,
    expires_at DateTime,
    updated_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (platform, puuid, queue_type);

CREATE TABLE IF NOT EXISTS winrift.riot_account_aliases
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
ORDER BY (platform, game_name_normalized, tag_line, puuid);

CREATE TABLE IF NOT EXISTS winrift.riot_request_events
(
    route LowCardinality(String),
    source LowCardinality(String),
    request_count UInt16,
    happened_at DateTime64(3),
    inserted_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = MergeTree
ORDER BY (route, happened_at, source)
TTL toDateTime(happened_at) + INTERVAL 1 DAY DELETE;

CREATE TABLE IF NOT EXISTS winrift.raw_matches
(
    match_id String,
    platform LowCardinality(String),
    queue_id UInt16,
    patch LowCardinality(String),
    game_creation UInt64,
    game_start_timestamp UInt64,
    game_end_timestamp UInt64,
    duration_seconds UInt32,
    raw_json String,
    ingested_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY match_id;

CREATE TABLE IF NOT EXISTS winrift.raw_timelines
(
    match_id String,
    platform LowCardinality(String),
    patch LowCardinality(String),
    queue_id UInt16,
    raw_json String,
    ingested_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY match_id;

CREATE TABLE IF NOT EXISTS winrift.patch_snapshots
(
    patch LowCardinality(String),
    platform LowCardinality(String),
    queue_id UInt16,
    status LowCardinality(String),
    started_at DateTime,
    closed_at Nullable(DateTime),
    raw_retained_until Nullable(DateTime),
    matches UInt64,
    participants UInt64,
    compiled_at Nullable(DateTime),
    notes String,
    updated_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (patch, platform, queue_id);

CREATE TABLE IF NOT EXISTS winrift.patch_build_metrics
(
    patch LowCardinality(String),
    platform LowCardinality(String),
    queue_id UInt16,
    champion_id UInt16,
    role LowCardinality(String),
    opponent_champion_id UInt16,
    rank_bucket LowCardinality(String),
    final_items_signature String,
    core2_signature String,
    core3_signature String,
    rune_signature String,
    spell_signature String,
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
    rank_bucket,
    final_items_signature,
    core2_signature,
    core3_signature,
    rune_signature,
    spell_signature
);

CREATE TABLE IF NOT EXISTS winrift.patch_item_timing_metrics
(
    patch LowCardinality(String),
    platform LowCardinality(String),
    queue_id UInt16,
    champion_id UInt16,
    role LowCardinality(String),
    opponent_champion_id UInt16,
    rank_bucket LowCardinality(String),
    item_slot UInt8,
    item_signature String,
    games UInt64,
    avg_timing_ms Float64,
    p50_timing_ms Float64,
    p75_timing_ms Float64,
    p90_timing_ms Float64,
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
    rank_bucket,
    item_slot,
    item_signature
);

CREATE TABLE IF NOT EXISTS winrift.patch_power_curve_metrics
(
    patch LowCardinality(String),
    platform LowCardinality(String),
    queue_id UInt16,
    champion_id UInt16,
    role LowCardinality(String),
    opponent_champion_id UInt16,
    rank_bucket LowCardinality(String),
    minute_mark UInt8,
    games UInt64,
    avg_level Float64,
    avg_total_gold Float64,
    avg_cs Float64,
    avg_jungle_cs Float64,
    avg_damage_done_to_champions Float64,
    avg_damage_taken Float64,
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
    rank_bucket,
    minute_mark
);

CREATE TABLE IF NOT EXISTS winrift.match_team_win_conditions
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
ORDER BY (patch, platform, queue_id, match_id, team_id);

CREATE TABLE IF NOT EXISTS winrift.patch_win_condition_metrics
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
);

CREATE TABLE IF NOT EXISTS winrift.timeline_participant_frames
(
    match_id String,
    platform LowCardinality(String),
    patch LowCardinality(String),
    queue_id UInt16,
    timestamp_ms UInt32,
    participant_id UInt8,
    level UInt8,
    xp UInt32,
    current_gold UInt32,
    total_gold UInt32,
    minions_killed UInt32,
    jungle_minions_killed UInt32,
    position_x Int32,
    position_y Int32,
    total_damage_done_to_champions UInt32,
    total_damage_taken UInt32,
    ingested_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY (match_id, timestamp_ms, participant_id);

CREATE TABLE IF NOT EXISTS winrift.timeline_item_events
(
    match_id String,
    platform LowCardinality(String),
    patch LowCardinality(String),
    queue_id UInt16,
    timestamp_ms UInt32,
    participant_id UInt8,
    event_type LowCardinality(String),
    item_id UInt32,
    before_id UInt32,
    after_id UInt32,
    ingested_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY (match_id, participant_id, timestamp_ms, event_type, item_id);

CREATE TABLE IF NOT EXISTS winrift.timeline_combat_events
(
    match_id String,
    platform LowCardinality(String),
    patch LowCardinality(String),
    queue_id UInt16,
    timestamp_ms UInt32,
    killer_id UInt8,
    victim_id UInt8,
    assisting_participant_ids String,
    bounty UInt32,
    shutdown_bounty UInt32,
    position_x Int32,
    position_y Int32,
    ingested_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY (match_id, timestamp_ms, killer_id, victim_id);

CREATE TABLE IF NOT EXISTS winrift.timeline_objective_events
(
    match_id String,
    platform LowCardinality(String),
    patch LowCardinality(String),
    queue_id UInt16,
    timestamp_ms UInt32,
    event_type LowCardinality(String),
    killer_id UInt8,
    team_id UInt16,
    monster_type LowCardinality(String),
    monster_sub_type LowCardinality(String),
    building_type LowCardinality(String),
    tower_type LowCardinality(String),
    lane_type LowCardinality(String),
    position_x Int32,
    position_y Int32,
    ingested_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY (match_id, timestamp_ms, event_type, team_id);

CREATE TABLE IF NOT EXISTS winrift.participants
(
    match_id String,
    platform LowCardinality(String),
    patch LowCardinality(String),
    queue_id UInt16,
    participant_id UInt8,
    puuid String,
    team_id UInt16,
    champion_id UInt16,
    champion_name String,
    role LowCardinality(String),
    win UInt8,
    kills UInt16,
    deaths UInt16,
    assists UInt16,
    item0 UInt32,
    item1 UInt32,
    item2 UInt32,
    item3 UInt32,
    item4 UInt32,
    item5 UInt32,
    trinket_item UInt32,
    summoner_spell1 UInt16,
    summoner_spell2 UInt16,
    primary_rune_tree UInt16,
    secondary_rune_tree UInt16,
    keystone UInt16,
    rune_signature String,
    spell_signature String,
    final_items_signature String,
    core2_signature String,
    core3_signature String,
    rank_bucket LowCardinality(String),
    ingested_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY (match_id, participant_id);

CREATE TABLE IF NOT EXISTS winrift.participant_matchups AS winrift.participants
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY (match_id, participant_id);

ALTER TABLE winrift.participant_matchups ADD COLUMN IF NOT EXISTS opponent_participant_id UInt8;
ALTER TABLE winrift.participant_matchups ADD COLUMN IF NOT EXISTS opponent_champion_id UInt16;
ALTER TABLE winrift.participant_matchups ADD COLUMN IF NOT EXISTS opponent_role LowCardinality(String);

CREATE MATERIALIZED VIEW IF NOT EXISTS winrift.build_analytics_mv
ENGINE = SummingMergeTree
ORDER BY
(
    platform,
    queue_id,
    champion_id,
    role,
    opponent_champion_id,
    patch_bucket,
    rank_bucket,
    final_items_signature,
    core2_signature,
    core3_signature,
    rune_signature,
    spell_signature
)
AS
SELECT
    platform,
    queue_id,
    champion_id,
    role,
    opponent_champion_id,
    patch AS patch_bucket,
    rank_bucket,
    final_items_signature,
    core2_signature,
    core3_signature,
    rune_signature,
    spell_signature,
    toUInt64(sum(win)) AS wins,
    toUInt64(count()) AS games
FROM winrift.participant_matchups
GROUP BY
    platform,
    queue_id,
    champion_id,
    role,
    opponent_champion_id,
    patch_bucket,
    rank_bucket,
    final_items_signature,
    core2_signature,
    core3_signature,
    rune_signature,
    spell_signature;
