# Data Dictionary

## `raw_matches`

One row per Riot match payload. Deduped by `match_id` with `ReplacingMergeTree`.

Important columns: `match_id`, `platform`, `queue_id`, `patch`, timestamps, duration, `raw_json`.

## `collector_frontier`

Durable collection queue keyed by `platform` and `puuid`.

Important columns: source, source detail, first seen time, last checked time, next check time, priority, attempts, request/match counters, error count, and status.

Statuses:

- `pending`: queued and not checked yet.
- `active`: checked successfully and scheduled for a future pass.
- `error`: checked with non-auth errors and scheduled for retry.
- `blocked`: auth failure; do not retry until credentials are fixed.

## `raw_timelines`

One row per Riot timeline payload. Used for item purchase order and future power-spike work.

## `summoner_rank_snapshots`

Cached ranked metadata keyed by `platform`, `puuid`, and `queue_type`.

Important columns: tier, division, league points, wins, losses, rank bucket, fetched time, and expiry time. Current ingestion uses `RANKED_SOLO_5x5` snapshots for participant `rank_bucket` when rank enrichment is enabled. Unranked players are cached as `UNRANKED` so they are not repeatedly queried.

## `patch_snapshots`

One row per patch/platform/queue lifecycle state. Tracks whether a patch is collecting, compiling, or closed, plus match counts, participant counts, compile timestamp, and raw retention date.

## `patch_build_metrics`

Compact closed-patch build matchup metrics. This is the historical equivalent of the live `build_analytics_mv`.

## `patch_item_timing_metrics`

Compact closed-patch item timing metrics for first, second, and third non-trinket item purchases.

## `item_slot_analytics`

Current read model for live build matchup cards. One row per patch, item context, champion, role, opponent champion, rank bucket, item slot, and item id.

`item_context` is one of:

- `DEFAULT`: lane build items, excluding jungle/support-specific starts.
- `JUNGLE`: lane build items plus jungle items.
- `SUPPORT`: lane build items plus support items.

The live API reads this table first. If it is empty, the API can fall back to the older timeline scan, but normal operation should refresh this table and serve item cards from compact aggregate rows.

Refresh options:

- Local/dev API: `POST /api/dev/analytics/item-slots/refresh`
- CLI: `patchctl -action item-slots -patch 16.10 -queue 420`

## `patch_power_curve_metrics`

Compact closed-patch participant power-curve metrics at 10, 15, and 20 minutes.

## `match_team_win_conditions`

Derived one-row-per-team match strategy profile.

Important columns: match, platform, patch, queue, team id, win/loss, duration, champion ids, team rank bucket, five win-condition scores, five letter ratings, primary condition, primary rating, and profile patch.

This table is generated from raw participant rows plus `services/core/internal/winconditions/champion_profiles.json`; it does not come from Riot directly.

## `patch_win_condition_metrics`

Compiled win-condition matchup metrics used by the live UI.

Dimensions: patch, platform, queue, rank bucket, team condition, team rating, opponent condition, opponent rating, primary-mode pairing, and game-length bucket.

Measures: wins, games, precomputed winrate percentage, and precomputed Wilson lower-bound confidence percentage.

Rollups are stored as normal rows: `platform = ALL`, `rank_bucket = ALL`, and `game_length_bucket = ALL`.

`team_primary` stores the team/opponent primary-mode pair as two digits. The first digit is the team's mode and the second is the opponent's mode: `1` means primary-only and `2` means any primary state. For example, `11` means team primary vs opponent primary, `21` means team any-state alternative vs opponent primary, and `22` means alternative-vs-alternative. Live UI requests should read these compiled rows directly instead of recomputing percentages from raw team rows.

## `participants`

One row per player per match.

Important columns: champion, role, team, win, KDA, six final item slots, trinket, summoner spells, rune trees, keystone, rune signature, spell signature, final item signature, core item signatures, rank bucket.

`rank_bucket` is the player's ranked tier grouping, for example `IRON`, `GOLD`, `DIAMOND`, or `MASTER+`. Match-V5 does not include rank, so rows ingest as `UNKNOWN` unless rank enrichment is enabled and a fresh `summoner_rank_snapshots` row is available or fetched.

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
