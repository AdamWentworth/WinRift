# Production Performance And Refresh Audit: 2026-05-31

This audit was run against the private server deployment at `http://192.168.1.77:8000` from the dev laptop. It focuses on whether the worker, API, ClickHouse storage, read-model refreshes, and page-facing endpoints are healthy enough to keep building on.

## Short Version

The deployment is healthy. The worker is collecting, all summary refresh lanes are completing, and warm champion-guide reads are now effectively instant on the private LAN.

The main remaining performance risk is not the normal hot path. It is the first request after a cache miss plus a few refresh lanes that still rebuild by deleting rows before inserting their next snapshot.

## Runtime Health

Observed container state:

| Container | State |
| --- | --- |
| `winrift_api` | Up |
| `winrift_worker` | Up |
| `winrift_monitor` | Up |
| `winrift_clickhouse` | Up and healthy |

Worker refresh status was written successfully at `2026-06-01T00:16:11Z`:

| Lane | Patch scope | Last duration | Result |
| --- | --- | ---: | --- |
| Champion guide analytics | `16.11,16.10` | 140.1 s | Success |
| Item-slot analytics | `16.11` | 16.9 s | Success |
| Win-condition analytics | `16.11` | 21.1 s | Success |
| Summoner profile analytics | all retained rows | 36.3 s | Success |

No recent auth failures, Riot rate-limit stops, or refresh failures were visible in the sampled worker logs.

## Stored Data

Raw retained match count:

| Patch | Matches | Latest ingest |
| --- | ---: | --- |
| `16.10` | 36,283 | `2026-06-01 00:24:32` |
| `16.11` | 4,581 | `2026-06-01 00:24:02` |

Raw `16.9` rows have been pruned from bulky raw tables, while derived `16.9` summaries are still retained in read models.

Summary/read-model rows currently include:

| Table | Rows |
| --- | ---: |
| `champion_page_bundle_cache` | 866 |
| `champion_guide_summary_analytics` | 3,380 |
| `item_slot_analytics` | 2,963,617 |
| `starting_loadout_analytics` | 305,150 |
| `patch_win_condition_metrics` | 2,201,461 |
| `summoner_profile_summary` | 106,726 |
| `summoner_recent_match_summary` | 510,919 |
| `summoner_build_summary` | 509,405 |

## Storage

ClickHouse reports:

| Measure | Value |
| --- | ---: |
| Active table parts | 5.80 GiB |
| Active rows | 51,926,037 |
| ClickHouse disk total | 116.81 GiB |
| ClickHouse disk free | 36.20 GiB |

Largest active tables:

| Table | Active disk |
| --- | ---: |
| `raw_timelines` | 4.26 GiB |
| `raw_matches` | 631.66 MiB |
| `timeline_participant_frames` | 297.29 MiB |
| `timeline_item_events` | 118.84 MiB |
| `participant_matchups` | 79.70 MiB |
| `participants` | 79.66 MiB |
| `champion_page_bundle_cache` | 44.76 MiB |

The storage pressure is reasonable for the current MVP, but raw timelines are the clear long-term cost center. ClickHouse logs also need periodic attention because log volume can grow independently of table data.

## API Timing

The API smoke script was run with one warmup and five measured requests for both `16.10` and `16.11`.

### Warm Path

| Endpoint | `16.10` avg/max | `16.11` avg/max | Read |
| --- | ---: | ---: | --- |
| `/api/health` | 3 / 4 ms | 3 / 3 ms | Good |
| `/api/analytics/patches` | 67 / 83 ms | 73 / 87 ms | Good |
| `/api/summoners/leaderboard` | 233 / 832 ms | 233 / 831 ms | Usually good, occasional warning |
| `/api/analytics/champion-roles` | 26 / 32 ms | 27 / 30 ms | Good |
| `/api/analytics/champion-guides` | 50 / 57 ms | 51 / 55 ms | Good |
| Champion page, Aatrox | 14 / 17 ms | 12 / 14 ms | Good |
| Champion page, Kled matchup | 15 / 18 ms | 13 / 16 ms | Good |
| Champion page, Lee Sin matchup | 13 / 15 ms | 11 / 12 ms | Good |

Warm champion-guide pages are no longer the obvious bottleneck.

### Cold Or Miss Path

API logs show several expensive first reads before cache reuse:

| Endpoint shape | Cold or first observed timing |
| --- | ---: |
| Champion page, Aatrox `16.10` | 5.0 s |
| Champion page, Aatrox `16.11` | 4.6 s |
| Champion page, Kled matchup | 1.5-1.9 s |
| Champion page, Lee Sin matchup | 3.9-4.7 s |
| Build advice, Aatrox vs Darius | 1.9 s |
| Summoner profile | 2.8-13.1 s |
| Win-condition validation | 5.1 s cold, then about 285 ms |

Interpretation: caching and read models work once a page has been built, but the cold-cache path is still visible enough that users can notice it.

## Refresh-Lane Risk

The champion-guide refresh lane already stages new rows before deleting older `compiled_at` snapshots. That is the safer pattern.

The following lanes still deserve the same treatment:

| Lane | Current concern |
| --- | --- |
| Summoner profile summaries | Refresh deletes summary tables before inserting the next snapshot. A failed or in-progress refresh can briefly expose empty profile/ladder data. |
| Item-slot and starting-loadout summaries | Refresh deletes the current context before inserting replacement rows. Exact build panels could look thin during a refresh. |
| Win-condition metrics | Refresh deletes patch/platform rows before inserting replacement rows. Live win-condition mode could fall back or look empty during a rebuild. |

These are not causing obvious production failures right now, but they are the highest-value reliability cleanup left in the read-model layer.

## Recommendations

1. Add short response caching for summoner leaderboard and summoner profile endpoints.
2. Convert summoner-profile, item-slot/loadout, and win-condition refreshes to staged insert-then-cleanup.
3. Inspect champion-page prewarm coverage because the worker skipped most prewarm candidates even though cold champion-page misses are still expensive.
4. Keep `ops/perf-smoke.sh` as the quick production guardrail, but add occasional `WINRIFT_PERF_WARMUPS=0` runs to catch cold misses.
5. Add a small storage/log check to the ops routine so ClickHouse logs and raw timeline growth do not surprise us.

