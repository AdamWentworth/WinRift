# Storage Policy

WinRift intentionally stores both raw Riot payloads and normalized analytics rows for active patches. ClickHouse compression makes this practical, and raw payloads let us rebuild read models when we improve the model later.

The policy is not "store everything forever." The policy is "keep raw current enough to iterate, compile old patches into summaries, then prune raw payload and timeline detail."

## Data Classes

Hot raw/current data:

- `raw_matches`
- `raw_timelines`
- `participants`
- `participant_matchups`
- `participant_performance`
- `timeline_participant_frames`
- `timeline_item_events`
- `timeline_skill_events`
- `timeline_combat_events`
- `timeline_objective_events`
- `champion_bans`

Current read models:

- `item_slot_analytics`
- `starting_loadout_analytics`
- `build_signature_analytics`
- `champion_role_analytics`
- `champion_skill_analytics`
- `champion_ban_analytics`
- `champion_build_variant_analytics`
- `match_team_win_conditions`
- `patch_win_condition_metrics`
- `summoner_profile_summary`
- `summoner_champion_summary`
- `summoner_champion_role_summary`

Historical compact metrics:

- `patch_build_metrics`
- `patch_item_timing_metrics`
- `patch_power_curve_metrics`
- `patch_win_condition_metrics`
- `patch_snapshots`

Historical normalized app index:

- `participants`
- `participant_matchups`
- `participant_performance`

This index is intentionally retained after raw pruning for now. It is far smaller than raw match/timeline payloads and keeps old patch pages, champion guides, tier lists, profiles, and matchup drilldowns alive. Only prune it after we add equivalent durable summary tables for every app surface.

Operational/cache tables:

- `collector_frontier`
- `summoner_rank_snapshots`
- `summoner_account_snapshots`
- `riot_account_aliases`
- `riot_request_events`

`riot_request_events` already has a one-day TTL. Rank/account snapshots are caches and can be regenerated from Riot if needed, subject to rate limits.

## Patch Retention

Active collection should keep the current patch and most recent previous patch:

```text
COLLECTOR_CURRENT_PATCH=16.17
COLLECTOR_PATCH_RETENTION_COUNT=2
```

With those settings, the worker accepts `16.17` and `16.16`. When the current patch becomes `16.18`, update the env value and the window becomes `16.18` and `16.17`.

The worker stops walking a PUUID's match history when it reaches a patch older than the active window. That saves request budget and avoids filling the database with stale matches.

Only set this after confirming the old patch has been archived or backed up:

```text
COLLECTOR_PRUNE_OLD_PATCHES_ON_START=true
```

Startup pruning is raw-only and requires patches to be marked `closed`. It should not be treated as a casual startup default on a production server; prefer running `patchctl -action archive` deliberately.

## Closing A Patch

When a patch is done:

```bash
docker compose run --rm api /winrift-patchctl -action archive -patch 16.9 -platform ALL -queue 420 -retain-days 0
```

The archive step writes compact metrics and app-facing summaries, then prunes raw/timeline detail by default. To do the compile/summary pass first and inspect counts before deleting raw rows:

```bash
docker compose run --rm api /winrift-patchctl -action archive -patch 16.9 -platform ALL -queue 420 -retain-days 7 -prune-raw=false
```

Then prune raw rows later:

```bash
docker compose run --rm api /winrift-patchctl -action delete-raw -patch 16.9 -platform ALL -queue 420
```

`platform = ALL` resolves the patch's stored platforms from ClickHouse. Single-platform compile/delete still exists for emergency or partial repairs.

## Disk Placement

For the dev laptop, the default Docker named volume is fine:

```text
clickhouse_data:/var/lib/clickhouse
```

For the home server, prefer this order:

1. Local SSD/NVMe if enough space is available.
2. Dedicated local HDD/SSD mounted at `/mnt/storage/clickhouse`.
3. NAS as backup/archive storage.
4. NAS as primary ClickHouse storage only if local disk is impossible and you accept slower, more fragile writes.

ClickHouse is happier with local disk for active writes. The NAS is a better place for nightly compressed backups and closed-patch archives.

The current laptop ClickHouse volume is about `56G`. Active ClickHouse parts are much smaller than that, so some of the volume is likely merge/temp/detached overhead from heavy ingest and restarts. Treat that as a warning: the server's mounted data SSD is about `119G`, which is enough for the initial migration but not enough for casual unbounded growth.

## ClickHouse System Logs

ClickHouse's own `system.*_log` tables can outgrow the actual WinRift dataset if left at defaults. On the production server this happened with `system.text_log`, `system.processors_profile_log`, `system.query_log`, and `system.part_log`.

WinRift now ships bounded ClickHouse config in `ops/clickhouse/`:

- `ops/clickhouse/config.d/winrift-system-logs.xml` keeps ClickHouse system log tables on a one-day TTL and lowers `text_log` to warning level.
- `ops/clickhouse/users.d/winrift-query-logging.xml` disables per-query, query-view, query-thread, and processor-profile logging for the default profile.

These tables are diagnostic history only. Clearing them does not delete Riot match data, normalized participants, build analytics, champion guides, profile summaries, or win-condition summaries.

Useful storage check:

```bash
docker compose exec clickhouse clickhouse-client --query "
  SELECT database, formatReadableSize(sum(bytes_on_disk)) AS active_size, sum(rows) AS rows
  FROM system.parts
  WHERE active
  GROUP BY database
  ORDER BY sum(bytes_on_disk) DESC"
```

If the system database is unexpectedly large, inspect table sizes:

```bash
docker compose exec clickhouse clickhouse-client --query "
  SELECT database, table, formatReadableSize(sum(bytes_on_disk)) AS size, sum(rows) AS rows
  FROM system.parts
  WHERE active
  GROUP BY database, table
  ORDER BY sum(bytes_on_disk) DESC
  LIMIT 30"
```

Use the production env value to move ClickHouse without changing app code:

```text
WINRIFT_CLICKHOUSE_DATA_DIR=/mnt/storage/clickhouse/data
WINRIFT_CLICKHOUSE_LOG_DIR=/mnt/storage/clickhouse/logs
WINRIFT_CLICKHOUSE_BACKUP_DIR=/mnt/storage/clickhouse/backups
```

## Backup Targets

Suggested server/NAS layout:

```text
/mnt/storage/clickhouse/
  data/
  logs/
  backups/

/mnt/nas/winrift/
  backups/
  closed-patch-exports/
  asset-cache/
```

Backups are for disaster recovery. Closed-patch exports are optional smaller artifacts we can generate later from compact metrics. Asset cache is for optional Data Dragon splash art caching and should stay outside git.

## Volume Backup

First find the actual Compose volume name:

```bash
docker volume ls | grep clickhouse_data
```

Then make a cold backup. This is the safest simple backup because ClickHouse is stopped while files are copied:

```bash
make down
docker run --rm \
  -v winrift_clickhouse_data:/var/lib/clickhouse:ro \
  -v /mnt/nas/winrift/backups:/backup \
  alpine sh -c 'tar -czf /backup/clickhouse-$(date +%F-%H%M).tgz -C /var/lib/clickhouse .'
make up
```

Replace `winrift_clickhouse_data` with the real volume name from `docker volume ls`.

For production, do this on a maintenance cadence or add a backup job that stops only the app stack long enough to capture a consistent archive.

## Restore Drill

Restores should be practiced before relying on backups.

High-level restore flow:

1. Stop the stack with `make down`.
2. Move the existing ClickHouse volume or host-path directory aside.
3. Restore the backup into a clean ClickHouse data directory.
4. Start ClickHouse and API with `make up`.
5. Verify basic counts and read endpoints before starting the worker.

Do not start the worker until the restored database has been checked. A restore problem should not be mixed with new collection writes.

## What To Keep Long-Term

Keep indefinitely:

- closed-patch compact metrics
- app-facing summary tables
- normalized app index until equivalent durable summaries exist
- `patch_snapshots`
- win-condition profile JSON and narrative docs
- enough metadata to explain how a metric was produced

Keep only for active/current analysis:

- raw matches and raw timelines
- per-frame timeline rows
- item/combat/objective event detail
- raw champion ban rows

This gives us trend history per patch without letting raw payloads grow forever.

## Open Improvements

- Add a formal ClickHouse migration command.
- Add a production backup script once the server storage path is chosen.
- Add a closed-patch export command for compact metric snapshots.
- Add disk-usage dashboard queries to track raw, timeline, summary, and cache tables separately.
