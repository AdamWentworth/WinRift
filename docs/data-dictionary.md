# Data Dictionary

## `raw_matches`

One row per Riot match payload. Deduped by `match_id` with `ReplacingMergeTree`.

Important columns: `match_id`, `platform`, `queue_id`, `patch`, timestamps, duration, `raw_json`.

## `raw_timelines`

One row per Riot timeline payload. Used for item purchase order and future power-spike work.

## `patch_snapshots`

One row per patch/platform/queue lifecycle state. Tracks whether a patch is collecting, compiling, or closed, plus match counts, participant counts, compile timestamp, and raw retention date.

## `patch_build_metrics`

Compact closed-patch build matchup metrics. This is the historical equivalent of the live `build_analytics_mv`.

## `patch_item_timing_metrics`

Compact closed-patch item timing metrics for first, second, and third non-trinket item purchases.

## `patch_power_curve_metrics`

Compact closed-patch participant power-curve metrics at 10, 15, and 20 minutes.

## `participants`

One row per player per match.

Important columns: champion, role, team, win, KDA, six final item slots, trinket, summoner spells, rune trees, keystone, rune signature, spell signature, final item signature, core item signatures, rank bucket.

## `participant_matchups`

One row per participant paired against an opponent. Default pairing is same `teamPosition` on the enemy team; fallback is the first enemy if role data is missing.

## `timeline_participant_frames`

One row per timeline frame per participant. Used for power-curve analysis.

Important columns: timestamp, participant, level, XP, current/total gold, lane/jungle CS, map position, champion damage dealt, and damage taken.

## `timeline_item_events`

One row per item timeline event.

Stored event types: `ITEM_PURCHASED`, `ITEM_SOLD`, `ITEM_DESTROYED`, and `ITEM_UNDO`.

Important columns: timestamp, participant, item id, before id, and after id.

## `timeline_combat_events`

One row per `CHAMPION_KILL`.

Important columns: timestamp, killer, victim, assisting participant ids, bounty, shutdown bounty, and map position.

## `timeline_objective_events`

One row per major map/objective event.

Stored event types: `ELITE_MONSTER_KILL`, `BUILDING_KILL`, and `TURRET_PLATE_DESTROYED`.

Important columns: timestamp, killer, team, monster type/subtype, building type, tower type, lane type, and map position.

## `build_analytics_mv`

Grouped read view over `participant_matchups`.

Dimensions: champion, role, opponent champion, patch, rank bucket, final item signature, core item signatures, rune signature, spell signature.

Measures: wins and games. The API computes winrate and Wilson lower-bound confidence.
