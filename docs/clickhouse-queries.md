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

## Win-Condition Grade Distribution

Use this to pressure-test whether the current letter grades are balanced or whether most real teams cluster into a narrow band.

```sql
SELECT
  axis,
  rating,
  count() AS teams,
  round(avg(score), 2) AS avg_score,
  min(score) AS min_score,
  max(score) AS max_score
FROM
(
  SELECT 'SplitPush' AS axis, splitpush_rating AS rating, splitpush_score AS score FROM match_team_win_conditions FINAL WHERE patch = '16.17'
  UNION ALL
  SELECT 'Pick' AS axis, pick_rating AS rating, pick_score AS score FROM match_team_win_conditions FINAL WHERE patch = '16.17'
  UNION ALL
  SELECT 'Siege' AS axis, siege_rating AS rating, siege_score AS score FROM match_team_win_conditions FINAL WHERE patch = '16.17'
  UNION ALL
  SELECT 'Control' AS axis, control_rating AS rating, control_score AS score FROM match_team_win_conditions FINAL WHERE patch = '16.17'
  UNION ALL
  SELECT 'TeamFight' AS axis, teamfight_rating AS rating, teamfight_score AS score FROM match_team_win_conditions FINAL WHERE patch = '16.17'
)
GROUP BY axis, rating
ORDER BY axis, avg_score DESC;
```

## Win-Condition Primary Margins

Use this to see whether teams usually have a clear primary condition or whether primary labels are often near ties.

```sql
SELECT
  multiIf(primary_margin = 0, 'TIE', primary_margin = 1, '1', primary_margin <= 3, '2-3', primary_margin <= 6, '4-6', '7+') AS margin_bucket,
  count() AS teams
FROM
(
  SELECT
    arrayElement(arrayReverseSort([splitpush_score, pick_score, siege_score, control_score, teamfight_score]), 1)
      - arrayElement(arrayReverseSort([splitpush_score, pick_score, siege_score, control_score, teamfight_score]), 2) AS primary_margin
  FROM match_team_win_conditions FINAL
  WHERE patch = '16.17'
)
GROUP BY margin_bucket
ORDER BY multiIf(margin_bucket = 'TIE', 0, margin_bucket = '1', 1, margin_bucket = '2-3', 2, margin_bucket = '4-6', 3, 4);
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
WHERE patch = '16.17'
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
  wins,
  games,
  win_rate_percent,
  confidence_percent
FROM patch_win_condition_metrics FINAL
WHERE patch = '16.17'
  AND platform = 'ALL'
  AND queue_id = 420
  AND rank_bucket = 'ALL'
  AND team_primary = 11
  AND team_condition = 'Pick'
  AND team_rating = 'B'
  AND opponent_condition = 'Siege'
  AND opponent_rating = 'B'
ORDER BY game_length_bucket;
```
