# Patch Lifecycle

WinRift treats patches as hard analytics boundaries. Current patch data keeps raw payloads and detailed normalized rows. Closed patches keep app-facing summaries plus a small normalized lookup index so old patch pages keep working after bulky raw payloads are pruned.

## States

- `collecting`: current patch, raw and normalized data are being inserted.
- `compiling`: closeout is running and compact metric tables are being populated.
- `closed`: compact patch metrics are available; raw data can be retained briefly or deleted.

Tracked in `patch_snapshots`.

## Archive A Patch

Use archive when a patch falls out of the active collection window. This is the safe closeout path:

1. Backfill participant performance from raw match payloads.
2. Backfill skill-order and ban events from raw payloads.
3. Refresh item slot and starting-loadout summaries.
4. Refresh champion guide variants, skills, and ban summaries.
5. Refresh summoner profile summaries.
6. Compile per-platform build, item timing, power curve, and win-condition metrics.
7. Delete only raw payload/timeline detail if `-prune-raw=true`.

Docker:

```bash
docker compose run --rm api /winrift-patchctl -action archive -patch 16.9 -platform ALL -queue 420 -retain-days 0
```

Local Go:

```bash
cd core
go run ./cmd/patchctl -action archive -patch 16.9 -platform ALL -queue 420 -retain-days 0
```

Archive writes or refreshes:

- `patch_build_metrics`
- `patch_item_timing_metrics`
- `patch_power_curve_metrics`
- `item_slot_analytics`
- `starting_loadout_analytics`
- `build_signature_analytics`
- `champion_role_analytics`
- `champion_skill_analytics`
- `champion_ban_analytics`
- `champion_build_variant_analytics`
- `summoner_profile_summary`
- `summoner_champion_summary`
- `summoner_champion_role_summary`
- `match_team_win_conditions`
- `patch_win_condition_metrics`
- `patch_snapshots`

It also clears that patch from the live `build_analytics_mv` so closed patch results come from compact metrics and do not double-count. Build queries explicitly avoid counting both `participant_matchups` and `patch_build_metrics` for the same compiled platform.

## Compile One Platform Only

Compile is still available when you want to close one platform without running the full archive pipeline:

```bash
docker compose run --rm api /winrift-patchctl -action compile -patch 16.10 -platform NA1 -queue 420 -retain-days 30
```

## Refresh Win-Condition Metrics Only

Win-condition metrics can be refreshed without closing a patch or touching build metrics:

```bash
docker compose run --rm api /winrift-patchctl -action win-conditions -patch 16.10 -platform NA1 -queue 420
```

This is useful during active collection because the live UI can read fast compiled strategy metrics while raw match rows continue to grow.

The refresh writes patch/platform rows and also refreshes `platform = ALL` rollups for that patch. It stores `game_length_bucket = ALL` totals, per-duration buckets, `rank_bucket = ALL` totals, winrate percentages, and Wilson confidence percentages so live requests do not recompute those numbers.

## Delete Raw Closed-Patch Data

Run this only after archive or compile succeeds and the retention window has passed:

```bash
docker compose run --rm api /winrift-patchctl -action delete-raw -patch 16.10 -platform ALL -queue 420
```

This removes bulky raw/timeline rows for the patch:

- raw matches and timelines
- timeline frames/events
- raw champion ban events

Compact patch metrics remain. The normalized app index also remains:

- `participants`
- `participant_matchups`
- `participant_performance`

Those retained normalized rows are intentionally much smaller than raw payloads and keep champion guide, tier list, profile, and matchup views useful for old patches. We can prune them later only after equivalent durable read models exist.

Derived summary rows and metrics also remain:

- `match_team_win_conditions`
- `patch_win_condition_metrics`
- build, item, skill, ban, and profile summary tables

## Start A Patch

Optional marker:

```bash
docker compose run --rm api /winrift-patchctl -action collecting -patch 16.11 -platform NA1 -queue 420
```
