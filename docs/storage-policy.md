# Storage Policy

WinRift intentionally stores both raw Riot payloads and normalized analytics rows for active patches. ClickHouse compression makes this practical, and raw payloads let us rebuild read models when we improve the model later.

The policy is not "store everything forever." The policy is "keep raw current enough to iterate, compile old patches into metrics, then prune raw detail."

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
COLLECTOR_CURRENT_PATCH=16.10
COLLECTOR_PATCH_RETENTION_COUNT=2
```

With those settings, the worker accepts `16.10` and `16.9`. When the current patch becomes `16.11`, update the env value and the window becomes `16.11` and `16.10`.

The worker stops walking a PUUID's match history when it reaches a patch older than the active window. That saves request budget and avoids filling the database with stale matches.

Only set this after confirming the old patch has been compiled or backed up:

```text
COLLECTOR_PRUNE_OLD_PATCHES_ON_START=true
```

Pruning deletes raw/detailed rows outside the active patch window. It should not be treated as a casual startup default on a production server.

## Closing A Patch

When a patch is done:

```bash
docker compose run --rm api /winrift-patchctl -action compile -patch 16.10 -platform NA1 -queue 420 -retain-days 30
```

The compile step writes compact metrics for build, item timing, power curve, and win-condition reads. After compile and whatever backup window you want:

```bash
docker compose run --rm api /winrift-patchctl -action delete-raw -patch 16.10 -platform NA1 -queue 420
```

Run those commands once per platform in `COLLECTOR_PLATFORMS`. The current `patchctl` CLI is platform-specific for compile/delete. The worker's scheduled win-condition refresh can produce `platform = ALL` rollups for live reads, but closing raw patch data remains a per-platform operation.

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
- `patch_snapshots`
- win-condition profile JSON and narrative docs
- enough metadata to explain how a metric was produced

Keep only for active/current analysis:

- raw matches and raw timelines
- per-frame timeline rows
- item/combat/objective event detail
- participant rows outside the active two-patch window

This gives us trend history per patch without letting raw payloads grow forever.

## Open Improvements

- Add a formal ClickHouse migration command.
- Add a production backup script once the server storage path is chosen.
- Add a closed-patch export command for compact metric snapshots.
- Add disk-usage dashboard queries to track raw, timeline, summary, and cache tables separately.
