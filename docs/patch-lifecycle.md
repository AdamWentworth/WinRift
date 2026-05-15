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
- `patch_snapshots`

It also clears that patch from the live `build_analytics_mv` so closed patch results come from compact metrics and do not double-count.

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

## Start A Patch

Optional marker:

```bash
docker compose run --rm api /winrift-patchctl -action collecting -patch 16.11 -platform NA1 -queue 420
```
