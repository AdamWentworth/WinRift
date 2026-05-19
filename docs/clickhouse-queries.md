# ClickHouse Queries

## Recent Raw Matches

```sql
SELECT match_id, platform, patch, queue_id, duration_seconds, ingested_at
FROM raw_matches FINAL
ORDER BY ingested_at DESC
LIMIT 20;
```

## Collector Frontier

```sql
SELECT
  platform,
  status,
  source,
  priority,
  attempts,
  matches_seen,
  matches_inserted,
  errors,
  next_check_at
FROM collector_frontier FINAL
ORDER BY priority DESC, next_check_at ASC
LIMIT 50;
```

## Rank Snapshot Cache

```sql
SELECT
  platform,
  queue_type,
  rank_bucket,
  tier,
  division,
  league_points,
  wins,
  losses,
  fetched_at,
  expires_at
FROM summoner_rank_snapshots FINAL
ORDER BY fetched_at DESC
LIMIT 50;
```

## Champion vs Opponent Builds

```sql
SELECT
  final_items_signature,
  core3_signature,
  rune_signature,
  spell_signature,
  sum(wins) AS wins,
  sum(games) AS games,
  wins / games AS win_rate
FROM build_analytics_mv
WHERE champion_id = 26
  AND role = 'TOP'
  AND opponent_champion_id = 86
GROUP BY final_items_signature, core3_signature, rune_signature, spell_signature
HAVING games >= 5
ORDER BY win_rate DESC, games DESC
LIMIT 25;
```

## Low Sample Review

```sql
SELECT champion_id, role, opponent_champion_id, count() AS rows
FROM build_analytics_mv
WHERE games < 5
GROUP BY champion_id, role, opponent_champion_id
ORDER BY rows DESC
LIMIT 50;
```

## Item Spike Timings

```sql
SELECT
  participant_id,
  item_id,
  min(timestamp_ms) / 60000 AS first_purchase_minute
FROM timeline_item_events
WHERE match_id = 'NA1_...'
  AND event_type = 'ITEM_PURCHASED'
GROUP BY participant_id, item_id
ORDER BY participant_id, first_purchase_minute;
```

## Ten Minute Participant State

```sql
SELECT
  participant_id,
  level,
  total_gold,
  minions_killed,
  jungle_minions_killed,
  total_damage_done_to_champions
FROM timeline_participant_frames
WHERE match_id = 'NA1_...'
  AND timestamp_ms BETWEEN 590000 AND 610000
ORDER BY participant_id;
```

## Objective Timeline

```sql
SELECT
  timestamp_ms / 60000 AS minute,
  event_type,
  team_id,
  monster_type,
  monster_sub_type,
  building_type,
  tower_type,
  lane_type
FROM timeline_objective_events
WHERE match_id = 'NA1_...'
ORDER BY timestamp_ms;
```

## Patch Snapshots

```sql
SELECT
  patch,
  platform,
  queue_id,
  status,
  matches,
  participants,
  compiled_at,
  raw_retained_until
FROM patch_snapshots FINAL
ORDER BY patch DESC, updated_at DESC;
```

## Closed Patch Build Metrics

```sql
SELECT
  champion_id,
  role,
  opponent_champion_id,
  final_items_signature,
  wins,
  games,
  wins / games AS win_rate
FROM patch_build_metrics FINAL
WHERE patch = '16.10'
  AND platform = 'NA1'
  AND queue_id = 420
ORDER BY win_rate DESC, games DESC
LIMIT 25;
```

## Win-Condition Team Rows

```sql
SELECT
  patch,
  platform,
  count() AS team_rows,
  uniqExact(match_id) AS matches
FROM match_team_win_conditions FINAL
GROUP BY patch, platform
ORDER BY patch DESC, platform;
```

## Win-Condition Matchup Metrics

```sql
SELECT
  team_condition,
  team_rating,
  opponent_condition,
  opponent_rating,
  game_length_bucket,
  sum(wins) AS wins,
  sum(games) AS games,
  wins / games AS win_rate
FROM patch_win_condition_metrics FINAL
WHERE patch = '16.10'
  AND queue_id = 420
  AND team_condition = 'Pick'
  AND team_rating = 'B'
  AND opponent_condition = 'Siege'
  AND opponent_rating = 'B'
GROUP BY
  team_condition,
  team_rating,
  opponent_condition,
  opponent_rating,
  game_length_bucket
ORDER BY game_length_bucket;
```
