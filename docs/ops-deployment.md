# Deployment And Operations

This doc is the practical "how do we run this without surprising ourselves later" guide. The intended production shape is simple: develop on the laptop, deploy code to the home server, and let the home server collect data once the worker is trusted.

## Runtime Shape

WinRift has four Docker services:

- `clickhouse`: persistent analytics database.
- `api`: Go HTTP API for the frontend, static metadata, live lookups, and read models.
- `worker`: Go collector loop. This is the only service that continuously spends Riot API budget.
- `web`: Vite/React frontend.

The `worker` is behind the Compose `worker` profile and has `restart: "no"`. That is intentional. A bad or expired Riot key should stop collection instead of creating an infinite retry loop.

## Local Development

Normal local app work:

```bash
make up
```

That starts ClickHouse, API, and web only. It does not start the collector.

Start collection explicitly:

```bash
make up-worker
```

Watch collection:

```bash
make logs-worker
```

Pause only collection:

```bash
make stop-worker
```

Fully bring down the project:

```bash
make down
```

Use `make down` instead of a profile-less `docker compose down` when the worker may have been running. The worker is profile-gated, so leaving the profile out can leave a collector container behind.

## Production Direction

Once the app is stable enough, production should collect on the server, not on the dev laptop.

Recommended model:

1. Dev laptop is for code, UI iteration, and small API checks.
2. Server owns the ClickHouse production volume.
3. Server owns the always-on worker.
4. CI/CD deploys code/images to the server.
5. Dev data is not routinely pushed into production. Move dev data only as a one-time bootstrap or emergency restore.

This avoids awkward "which machine has the real dataset?" problems. The server becomes the source of truth for collected matches.

## Environment

Production needs a server-local `.env`. Do not commit it.

Core values to verify before starting the worker:

```text
RIOT_API_KEY=...
ENVIRONMENT=production
CORS_ORIGINS=https://your-domain.example
COLLECTOR_PLATFORMS=NA1,EUW1,EUN1,KR,BR1,LA1,LA2,JP1,OC1,TR1,RU,SG2,TW2,VN2
COLLECTOR_CURRENT_PATCH=16.10
COLLECTOR_PATCH_RETENTION_COUNT=2
COLLECTOR_PRUNE_OLD_PATCHES_ON_START=false
RIOT_AUTH_FAILURE_EXIT=true
RIOT_AUTH_FAILURE_MARKER_PATH=/run/winrift/riot-auth-failed
```

Keep `COLLECTOR_PRUNE_OLD_PATCHES_ON_START=false` until the patch was compiled or backed up. Turn it on deliberately when the retention policy is ready to prune old raw rows.

## Riot Key Failure Behavior

The API and worker share an auth-failure marker at `RIOT_AUTH_FAILURE_MARKER_PATH`.

If Riot returns 401 or 403:

- The component that saw it writes the marker.
- The worker exits when the marker appears.
- The API stays up.
- Riot-backed endpoints return a service-unavailable response instead of repeatedly hitting Riot.
- Cached analytics, static pages, and database-backed reads can keep working.

After refreshing the key:

```bash
make up
make up-worker
```

Both API and worker clear the marker on startup. If the API is already running and you only restart the worker, the shared marker is still cleared from the runtime volume, but a full `make up` is the cleanest reset after a key refresh.

Riot 404s are not auth failures. A missing Riot ID or a summoner not being in a live game should return a normal not-found/live-game-absent response.

Riot 429s are rate limits. The client sleeps for `Retry-After` when the wait is reasonable, or defers that region when Riot asks for too long.

## Deployment Flow

A simple home-server deploy can be:

```bash
make stop-worker
git pull
make up
make up-worker
```

For CI/CD, the same idea applies:

1. Build and publish images, or pull the repo and build on the server.
2. Start/update ClickHouse, API, and web.
3. Verify `/api/health`.
4. Start the worker last.
5. Watch `make logs-worker` long enough to confirm platforms, patch window, request budget, and refresh intervals.

The worker should be stopped before risky migrations or config changes. API/web changes can usually deploy while the worker is paused.

## Schema Changes

The current Compose file mounts `services/core/internal/clickhouse/schema.sql` into ClickHouse init. That init file runs automatically for a fresh database volume.

For an existing production volume, schema changes need an explicit migration step. The Go repository has idempotent table/column creation paths for many runtime tables, but do not rely on that as the only production migration strategy forever.

Near-term rule:

- For additive schema changes, start API against staging/dev first and verify it creates or uses the expected columns.
- For production, pause the worker, deploy API, verify health and logs, then restart the worker.
- For destructive changes, write a one-off migration/runbook before applying it to the production volume.

Longer-term improvement: add a formal migration command so CI/CD can run `migrate up` before starting the API.

## Health Checks

Useful commands:

```bash
make status
make logs-api
make logs-worker
docker compose exec clickhouse clickhouse-client --query "SELECT count() FROM winrift.raw_matches"
```

Things to check after a restart:

- The worker logs the expected patch and platform list.
- `region_request_budget` looks sane for the current Riot key.
- `auth_failed=false` in collector summaries.
- Refresh intervals are visible for item slots, champion guides, win conditions, and summoner profiles.
- Match counts increase only while the worker is running.

## Server Storage Layout

The default Compose file uses the named volume `clickhouse_data`. That is fine for development.

For production, prefer a stable host path or dedicated disk mount so backups and disk monitoring are obvious. Example Compose override:

```yaml
services:
  clickhouse:
    volumes:
      - /srv/winrift/clickhouse:/var/lib/clickhouse
      - ./services/core/internal/clickhouse/schema.sql:/docker-entrypoint-initdb.d/001_schema.sql:ro
```

Keep hot ClickHouse data on local storage when possible. Use the NAS for backups and archives first. A network mount as the primary ClickHouse data directory can work for hobby traffic, but it is more fragile under write load and network hiccups.

See [Storage Policy](storage-policy.md) for retention, backup, and NAS guidance.
