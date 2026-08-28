# Collector Runbook

This runbook covers collection, patch rollover, and failure recovery. Configuration defaults live in [`.env.example`](../.env.example); production-only paths and deployment values live in [`ops/prod/winrift.env.example`](../ops/prod/winrift.env.example). Those files are authoritative for environment keys.

For server deployment and rollback, see [Production Operations](../ops/prod/README.md). For retention behavior, see [Storage Policy](storage-policy.md).

## Local Startup

Start ClickHouse, the API, and the development web app:

```bash
make up
```

Run the worker only when collection is intentional:

```bash
make up-worker
make logs-worker
```

The worker is profile-gated so ordinary UI development cannot silently consume Riot API budget. Pause it without stopping the API or ClickHouse:

```bash
make stop-worker
```

## Seed Collection

In development, seed a Riot ID through the API:

```bash
curl -X POST http://localhost:8000/api/dev/collector/seed \
  -H "Content-Type: application/json" \
  -d '{
    "riotIds": [{"gameName": "Example", "tagLine": "NA1", "platform": "NA1"}],
    "matchCount": 5,
    "maxRequests": 20
  }'
```

This resolves the account, inserts it into `collector_frontier`, collects a bounded batch, and adds discovered match participants back to the frontier. Use `"frontierOnly": true` to enqueue the seed without collecting immediately.

Worker startup can also read `COLLECTOR_SEED_RIOT_IDS`, `COLLECTOR_SEED_PUUIDS`, or a bounded Challenger ladder seed. Keep explicit seed and request counts small while using a Riot development key.

## Worker Cycle

Each sweep:

1. selects due frontier rows by platform and priority;
2. resolves recent Match-V5 IDs;
3. fetches missing match and timeline payloads;
4. writes raw payloads and normalized participant, matchup, item, skill, combat, and objective facts;
5. returns discovered participants to the frontier;
6. runs bounded rank and account-alias enrichment lanes;
7. refreshes due analytics read models and summoner-profile summaries;
8. records a heartbeat, refresh status, and regional request-budget events.

Live lookups can enqueue low-sample participants with `source='live-backfill'`. That improves later champion context without blocking the live response.

## Patch Window and Prewarming

`COLLECTOR_CURRENT_PATCH` is the explicit ingestion boundary. With the default retention count of two, a current value of `16.17` accepts `16.17` and `16.16`; older matches stop the frontier walk. Change the current patch only during an intentional rollover.

Analytics refreshes do not consume Riot API budget. They read local ClickHouse data and rebuild compact item-slot, guide, win-condition, and profile summaries.

Champion-page readiness has priority at worker startup:

- every canonical champion is warmed for the current patch;
- champions without enough current-patch guide data receive a mature fallback;
- every canonical champion is warmed for every selectable archived patch;
- automatic-role and resolved-role aliases are stored;
- changing retained patches are refreshed on schedule;
- immutable archived bundles remain reusable for 30 days;
- common retained-patch matchup bundles are warmed within a hard cap.

Worker readiness markers and the full champion/patch performance audit prevent a production deploy from completing while canonical pages are cold, incomplete, or slow.

## Production Key Rotation and Rollover

After obtaining a new Riot development key, run on the production host:

```bash
riotkey
```

The helper hides the entered key, displays only a masked preview, and requires explicit confirmation. It preserves the running immutable image, pauses worker/monitor activity, updates the server-local env file, checks Data Dragon for a newer patch, archives data outside the new retention window in bounded batches, recreates the API, and restarts monitoring and collection.

Useful exceptions:

- `refresh-riot-key --no-patch-rollover` performs a deliberate key-only refresh.
- `refresh-riot-key --show-key` reveals the full key only when explicitly requested.
- `refresh-riot-key --no-restart` is intended for isolated env-file tests.

Patch maintenance is resumable. Completed batches are detected and skipped on retry. If interrupted, the helper restores monitoring and leaves the worker stopped rather than collecting under an uncertain patch marker.

## Failure Behavior

| Condition | Behavior |
|---|---|
| Riot `401` or `403` | Write the auth marker; stop the worker; keep cached/static/analytics API routes available. |
| Riot `404` | Treat as ordinary absence, such as an unknown Riot ID or no active game. |
| Riot `429` | Honor `Retry-After` within the configured ceiling; otherwise defer the region until a later cycle. |
| Regional budget exhausted | Wait for that region's window while other regions or local refresh work may continue. |
| Analytics refresh failure | Record the failure and keep the previous compiled snapshot readable. |
| Missing or down worker | Monitor container state and heartbeat; alert only when the worker is configured as required. |

The worker heartbeat and refresh-status files are shared with the monitor. In production, `MONITOR_WORKER_REQUIRED=true` and `MONITOR_WORKER_CONTAINER_NAME=winrift_worker` distinguish an intentionally disabled worker from an unexpected stop.

## Request Budget

For each region, usable requests per cycle are calculated as:

```text
usable = min(bucket_size, bucket_size * cycle / bucket_window) - reserve
```

With the default 100-request, 120-second bucket and a 10-request reserve, the worker can use 90 requests per region per window. A frontier row costs one match-ID request plus two requests for each new match: one match payload and one timeline payload. Rank and alias enrichment are separately capped but share the same regional budget.

The reserve protects interactive account and live-game requests. Do not increase collector throughput merely because a short test pass succeeds; sustained Riot limits and daily development-key behavior are the real constraints.

## Shutdown

Use the canonical shutdown path after collection:

```bash
make down
make status
```

`make down` includes the worker and monitor profiles and removes orphan containers. Avoid profile-less `docker compose down` as the normal path after running the collector, because it can leave a profile-gated worker behind.
