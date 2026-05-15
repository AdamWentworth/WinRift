# Collector Runbook

## Start Infrastructure

```bash
docker compose up --build clickhouse api
```

## Seed From API

Call the dev-only endpoint while `ENVIRONMENT=development`:

```bash
curl -X POST http://localhost:8000/api/dev/collector/seed \
  -H "Content-Type: application/json" \
  -d '{
    "riotIds": [{"gameName": "Example", "tagLine": "NA1", "platform": "NA1"}],
    "matchCount": 5,
    "maxRequests": 20
  }'
```

This resolves seeds, inserts them into `collector_frontier`, immediately collects a small batch, and inserts discovered match participants back into the frontier.

Watch API-triggered collection live in another terminal:

```bash
docker compose logs -f api
```

The logs show seed resolution, match id lookup, already-ingested skips, match/timeline fetches, normalization counts, rank cache hits/misses, rank snapshot writes, inserts, and final request counters.

Use `frontierOnly` when you want to seed without collecting immediately:

```json
{"riotIds": [{"gameName": "Example", "tagLine": "NA1", "platform": "NA1"}], "frontierOnly": true}
```

## Seed Worker

Set one or both in `.env`:

```text
COLLECTOR_SEED_RIOT_IDS=Example#NA1
COLLECTOR_SEED_PUUIDS=
COLLECTOR_PLATFORMS=NA1
RIOT_MIN_REQUEST_INTERVAL_MS=75
RIOT_RATE_LIMIT_MAX_RETRIES=3
RIOT_RATE_LIMIT_MAX_SLEEP_SECONDS=120
RIOT_AUTH_FAILURE_EXIT=true
RIOT_AUTH_FAILURE_MARKER_PATH=/run/winrift/riot-auth-failed
COLLECTOR_INTERVAL_SECONDS=120
COLLECTOR_FRONTIER_BATCH_SIZE=3
COLLECTOR_MAX_REQUESTS_PER_PASS=0
COLLECTOR_RATE_LIMIT_REQUESTS=100
COLLECTOR_RATE_LIMIT_WINDOW_SECONDS=120
COLLECTOR_RATE_LIMIT_RESERVE_REQUESTS=10
COLLECTOR_RECHECK_HOURS=24
COLLECTOR_DISCOVERY_DELAY_MINUTES=60
COLLECTOR_AUTO_SEED_CHALLENGER=false
COLLECTOR_AUTO_SEED_LIMIT_PER_PLATFORM=3
RANK_ENRICHMENT_ENABLED=false
RANK_SNAPSHOT_TTL_HOURS=24
RANK_ENRICHMENT_MAX_REQUESTS_PER_PASS=5
```

Then run:

```bash
docker compose --profile worker up --build worker
```

For a detached worker, follow progress with:

```bash
docker compose logs -f worker
```

At startup, the worker resolves env seeds into `collector_frontier`. If `COLLECTOR_AUTO_SEED_CHALLENGER=true`, it also seeds each configured platform from that platform's Challenger Solo/Duo ladder. Each pass walks `COLLECTOR_PLATFORMS`, pulls due frontier rows per platform, collects recent ranked matches, stores normalized rows, queues discovered participants, updates counters/status, and sleeps using `COLLECTOR_INTERVAL_SECONDS`. The default local cadence is 120 seconds.

For broad multi-platform collection, use smaller per-platform budgets. For example:

```text
COLLECTOR_PLATFORMS=NA1,EUW1,EUN1,KR,BR1,LA1,LA2,JP1,OC1,TR1,RU,SG2,TW2,VN2
COLLECTOR_FRONTIER_BATCH_SIZE=1
COLLECTOR_MAX_REQUESTS_PER_PASS=0
RANK_ENRICHMENT_MAX_REQUESTS_PER_PASS=5
COLLECTOR_AUTO_SEED_CHALLENGER=true
```

`PH2` and `TH2` are known Riot platform values, but their platform API hostnames did not resolve from the local Docker network during testing. Leave them out unless those hosts resolve in your environment.

## Safety

Keep match counts low on a development key. The client honors Riot 429 `Retry-After` with bounded retries, but large crawl seeds can still consume a daily development-key window quickly.

If Riot returns 401 or 403, the process that saw it writes `RIOT_AUTH_FAILURE_MARKER_PATH`. The worker exits immediately, and the API watches the same marker and exits too. This prevents an expired or unauthorized development key from being retried every collector interval. The frontier row is marked `blocked` when the failure happens during a collection pass.

If Riot returns 429, the client sleeps for `Retry-After` and retries up to `RIOT_RATE_LIMIT_MAX_RETRIES`. If Riot asks for a longer wait than `RIOT_RATE_LIMIT_MAX_SLEEP_SECONDS`, the collector defers that region for the rest of the pass and resumes on the next cycle.

Safety knobs:

- `COLLECTOR_FRONTIER_BATCH_SIZE`: max PUUIDs checked per worker pass.
- `COLLECTOR_PLATFORMS`: comma-separated platform routing values to collect, such as `NA1,EUW1,KR`.
- `RIOT_MIN_REQUEST_INTERVAL_MS`: process-local spacing between Riot requests. `75`ms stays under the `20 requests / 1 second` bucket.
- `RIOT_RATE_LIMIT_MAX_RETRIES`: max immediate sleeps/retries for Riot 429 responses.
- `RIOT_RATE_LIMIT_MAX_SLEEP_SECONDS`: longest 429 `Retry-After` sleep before deferring the region to the next collector cycle.
- `RIOT_AUTH_FAILURE_EXIT`: when true, the API and worker stop when a Riot auth failure marker appears.
- `RIOT_AUTH_FAILURE_MARKER_PATH`: shared marker file path used by API and worker to coordinate an auth-failure stop.
- `COLLECTOR_INTERVAL_SECONDS`: sleep time between worker cycles. `120` seconds lines up with the personal/development `100 requests / 2 minutes` bucket.
- `COLLECTOR_RATE_LIMIT_REQUESTS`: Riot application request bucket size for one region.
- `COLLECTOR_RATE_LIMIT_WINDOW_SECONDS`: Riot application request bucket window.
- `COLLECTOR_RATE_LIMIT_RESERVE_REQUESTS`: requests held back per region as safety headroom.
- `COLLECTOR_MAX_REQUESTS_PER_PASS`: optional manual cap on match-history/match/timeline requests per platform. `0` means auto-calculate from the regional budget.
- `COLLECTOR_AUTO_SEED_CHALLENGER`: seed each platform from its Challenger Solo/Duo ladder on worker startup.
- `COLLECTOR_AUTO_SEED_LIMIT_PER_PLATFORM`: max Challenger ladder entries to seed per platform at startup.
- `COLLECTOR_DISCOVERY_DELAY_MINUTES`: delay before newly discovered participants are eligible.
- `COLLECTOR_RECHECK_HOURS`: delay before revisiting a checked PUUID.
- `RANK_ENRICHMENT_ENABLED`: when true, rank buckets are refreshed from cached rank snapshots or Riot League-V4.
- `RANK_ENRICHMENT_MAX_REQUESTS_PER_PASS`: max rank refresh requests per platform. These are subtracted from the same regional Riot request budget as match collection.
- `RANK_SNAPSHOT_TTL_HOURS`: freshness window before a rank snapshot can be refreshed.

Rank enrichment is off by default. Turn it on only after basic match ingestion is working; otherwise the first crawl spends extra Riot requests on player metadata before we know the match pipeline is healthy.

## Request Formula

For each region, the worker computes a usable cycle budget:

```text
usable_region_requests = min(rate_limit_requests, rate_limit_requests * interval / rate_limit_window) - reserve_requests
```

With the local defaults, that is `100 * 120 / 120 - 10 = 90` Riot requests per region per cycle. The worker splits that regional budget across the active platforms in the same region, reserves up to `RANK_ENRICHMENT_MAX_REQUESTS_PER_PASS` for rank snapshots, and gives the remainder to match collection.

Match collection costs `1 + (2 * matches)` requests per frontier row: one match-id lookup, then one match payload and one timeline payload per new ranked match. Rank enrichment can cost up to 10 extra requests per match on a cold cache, so the worker caps it separately and caches snapshots for `RANK_SNAPSHOT_TTL_HOURS`.
