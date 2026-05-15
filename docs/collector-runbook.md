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
COLLECTOR_FRONTIER_BATCH_SIZE=3
COLLECTOR_MAX_REQUESTS_PER_PASS=60
COLLECTOR_RECHECK_HOURS=24
COLLECTOR_DISCOVERY_DELAY_MINUTES=60
RANK_ENRICHMENT_ENABLED=false
RANK_SNAPSHOT_TTL_HOURS=24
RANK_ENRICHMENT_MAX_REQUESTS_PER_PASS=20
```

Then run:

```bash
docker compose --profile worker up --build worker
```

For a detached worker, follow progress with:

```bash
docker compose logs -f worker
```

At startup, the worker resolves env seeds into `collector_frontier`. Each pass pulls due frontier rows, collects recent ranked matches, stores normalized rows, queues discovered participants, updates counters/status, and sleeps using `COLLECTOR_INTERVAL_SECONDS`.

## Safety

Keep match counts low on a development key. The client honors Riot 429 `Retry-After`, but large crawl seeds can still consume a daily development-key window quickly.

If Riot returns 401 or 403, the worker exits immediately. This prevents an expired or unauthorized development key from being retried every collector interval. The frontier row is marked `blocked` when the failure happens during a collection pass.

Safety knobs:

- `COLLECTOR_FRONTIER_BATCH_SIZE`: max PUUIDs checked per worker pass.
- `COLLECTOR_MAX_REQUESTS_PER_PASS`: approximate Riot request budget per pass.
- `COLLECTOR_DISCOVERY_DELAY_MINUTES`: delay before newly discovered participants are eligible.
- `COLLECTOR_RECHECK_HOURS`: delay before revisiting a checked PUUID.
- `RANK_ENRICHMENT_ENABLED`: when true, rank buckets are refreshed from cached rank snapshots or Riot League-V4.
- `RANK_ENRICHMENT_MAX_REQUESTS_PER_PASS`: separate request budget for rank refreshes.
- `RANK_SNAPSHOT_TTL_HOURS`: freshness window before a rank snapshot can be refreshed.

Rank enrichment is off by default. Turn it on only after basic match ingestion is working; otherwise the first crawl spends extra Riot requests on player metadata before we know the match pipeline is healthy.
