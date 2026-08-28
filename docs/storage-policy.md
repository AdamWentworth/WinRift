# Storage Policy

WinRift keeps raw Riot payloads only while they are useful for active-model iteration. Closed patches are compiled into durable summaries before bulky match, timeline, and event detail is pruned.

The policy is:

1. retain raw and normalized detail for the active patch window;
2. build compact app-facing read models continuously;
3. archive a patch before it leaves the ingestion window;
4. retain historical summaries and the normalized lookup index;
5. prune raw payloads and high-volume timeline detail from closed patches.

## Data Classes

Active raw and normalized data includes:

- `raw_matches`, `raw_timelines`
- `participants`, `participant_matchups`, `participant_performance`
- timeline frames plus item, skill, combat, and objective events
- champion bans and team kill summaries

Current read models include:

- item slots and starting loadouts
- champion role, guide, matchup, skill, ban, signature, and build-variant summaries
- champion-page bundle cache
- win-condition team rows and patch metrics
- summoner identity, profile, champion, recent-match, and build summaries

Historical compact data includes:

- `patch_snapshots`
- `patch_build_metrics`
- `patch_item_timing_metrics`
- `patch_power_curve_metrics`
- `patch_win_condition_metrics`

The normalized participant index remains after raw pruning because archived champion pages, tier lists, profiles, and matchup drilldowns still depend on it. It should be removed only after equivalent durable summaries exist for every app surface.

Operational caches such as frontier state, account aliases, and rank snapshots are regenerable. `riot_request_events` has a one-day TTL.

## Patch Retention

Production normally collects the current and immediately previous patch:

```text
COLLECTOR_CURRENT_PATCH=16.17
COLLECTOR_PATCH_RETENTION_COUNT=2
```

The worker stops walking a PUUID's history when it reaches a patch outside that window. Advancing `COLLECTOR_CURRENT_PATCH` changes the ingestion boundary; it does not by itself prove that the departing patch was safely archived.

Keep startup pruning disabled unless the rollover procedure has verified the archive:

```text
COLLECTOR_PRUNE_OLD_PATCHES_ON_START=false
```

The production `riotkey` helper performs bounded, resumable rollover maintenance before moving the patch boundary.

## Archive and Prune

Compile a completed patch and prune its raw detail:

```bash
docker compose run --rm api \
  /winrift-patchctl -action archive -patch 16.9 -platform ALL -queue 420 -retain-days 0
```

To compile first and retain raw rows for an inspection window:

```bash
docker compose run --rm api \
  /winrift-patchctl -action archive -patch 16.9 -platform ALL -queue 420 \
  -retain-days 7 -prune-raw=false
```

Prune the raw rows later:

```bash
docker compose run --rm api \
  /winrift-patchctl -action delete-raw -patch 16.9 -platform ALL -queue 420
```

`platform=ALL` resolves the patch's stored platforms from ClickHouse. Single-platform operations remain available for partial repair.

## Disk Placement

Use local SSD/NVMe storage for active ClickHouse writes. A NAS is better suited to backups and closed-patch exports than to primary database writes.

Production paths are configured rather than embedded in application code:

```text
WINRIFT_CLICKHOUSE_DATA_DIR=/mnt/storage/clickhouse/data
WINRIFT_CLICKHOUSE_HOT_DIR=/mnt/storage/clickhouse/hot
WINRIFT_CLICKHOUSE_LOG_DIR=/mnt/storage/clickhouse/logs
WINRIFT_CLICKHOUSE_BACKUP_DIR=/mnt/storage/clickhouse/backups
```

The production deployment workflow refuses to start ClickHouse when its storage mount guard is not satisfied, which prevents database files from silently filling the OS disk.

## ClickHouse Diagnostics

ClickHouse system logs can outgrow application data if left unbounded. Configuration under `ops/clickhouse/`:

- applies a one-day TTL to system log tables;
- lowers `text_log` to warning level;
- disables per-query, view, thread, processor-profile, and high-frequency query-metric logging for the application profile.

These tables contain diagnostics, not Riot match data or WinRift read models. Production cleanup commands live in [Production Operations](../ops/prod/README.md).

## Backup and Restore

Backups protect the database; closed-patch summaries limit how much must be restored. A backup target should be outside the active ClickHouse disk.

For a simple cold backup:

1. stop the app stack;
2. archive the resolved ClickHouse volume or host-path directory;
3. copy the archive to the backup target;
4. restart ClickHouse and the API;
5. verify health and representative read endpoints before starting the worker.

A restore drill follows the inverse order:

1. stop the stack;
2. preserve the existing data directory;
3. restore into a clean ClickHouse path;
4. start ClickHouse and the API only;
5. verify table counts, patch metadata, and cached page endpoints;
6. start the worker after the restored database is known good.

Do not combine an uncertain restore with new collector writes.

## Long-Term Retention

Keep indefinitely:

- closed-patch compact metrics and app-facing summaries;
- the normalized app index until fully replaced by durable summaries;
- patch snapshots;
- win-condition profile data and enough metadata to reproduce each metric.

Keep only for active analysis:

- raw matches and timelines;
- per-frame timeline rows;
- item, skill, combat, and objective event detail;
- raw champion-ban rows.

This preserves patch history and portfolio-relevant analytics without allowing raw payload growth to become the storage strategy.
