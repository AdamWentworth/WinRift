# Patch Lifecycle

WinRift treats patches as hard analytics boundaries. Current patch data keeps raw payloads and detailed normalized rows; closed patches keep compact metrics.

## States

- `collecting`: current patch, raw and normalized data are being inserted.
- `compiling`: closeout is running and compact metric tables are being populated.
- `closed`: compact patch metrics are available; raw data can be retained briefly or deleted.

Tracked in `patch_snapshots`.

## Compile A Patch

Local Go:

```bash
cd services/core
go run ./cmd/patchctl -action compile -patch 16.10 -platform NA1 -queue 420 -retain-days 30
```

Docker:

```bash
docker compose run --rm api /winrift-patchctl -action compile -patch 16.10 -platform NA1 -queue 420 -retain-days 30
```

Compile writes:

- `patch_build_metrics`
- `patch_item_timing_metrics`
- `patch_power_curve_metrics`
- `match_team_win_conditions`
- `patch_win_condition_metrics`
- `patch_snapshots`

It also clears that patch from the live `build_analytics_mv` so closed patch results come from compact metrics and do not double-count.

## Refresh Win-Condition Metrics Only

Win-condition metrics can be refreshed without closing a patch or touching build metrics:

```bash
docker compose run --rm api /winrift-patchctl -action win-conditions -patch 16.10 -platform NA1 -queue 420
```

This is useful during active collection because the live UI can read fast compiled strategy metrics while raw match rows continue to grow.

The refresh writes patch/platform rows and also refreshes `platform = ALL` rollups for that patch. It stores `game_length_bucket = ALL` totals, per-duration buckets, `rank_bucket = ALL` totals, winrate percentages, and Wilson confidence percentages so live requests do not recompute those numbers.

## Delete Raw Closed-Patch Data

Run this only after compile succeeds and the retention window has passed:

```bash
docker compose run --rm api /winrift-patchctl -action delete-raw -patch 16.10 -platform NA1 -queue 420
```

This removes raw/detailed rows for the patch:

- raw matches and timelines
- participant rows
- matchup rows
- timeline frames/events
- live build aggregate rows

Compact patch metrics remain.

Derived win-condition rows and metrics also remain:

- `match_team_win_conditions`
- `patch_win_condition_metrics`

## Start A Patch

Optional marker:

```bash
docker compose run --rm api /winrift-patchctl -action collecting -patch 16.11 -platform NA1 -queue 420
```
