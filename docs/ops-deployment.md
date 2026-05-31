# Deployment And Operations

This doc is the practical "how do we run this without surprising ourselves later" guide. The intended production shape is simple: develop on the laptop, deploy code to the home server, and let the home server collect data once the worker is trusted.

## Runtime Shape

WinRift has five Docker services:

- `clickhouse`: persistent analytics database.
- `api`: Go HTTP API for the frontend, static metadata, live lookups, and read models.
- `worker`: Go collector loop. This is the only service that continuously spends Riot API budget.
- `monitor`: Go health monitor that watches API health, the Riot auth-failure marker, the worker container state, and the worker heartbeat.
- `web`: Vite/React frontend.

The `worker` is behind the Compose `worker` profile and has `restart: "no"`. That is intentional. A bad or expired Riot key should stop collection instead of creating an infinite retry loop.
The `monitor` does not call Riot. It reads local runtime state and sends SMTP alerts when the API is unhealthy, the Riot key has failed, or the worker container is down. Stale heartbeat observations are logged, not emailed.

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

Start and watch the local monitor profile:

```bash
make up-monitor
make logs-monitor
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

## Private Server Deployment

The production-like setup follows the same private-runner pattern used by the other home-server projects:

- CI builds and pushes `ghcr.io/adamwentworth/winrift-core` from `core/Dockerfile`.
- A manual GitHub Actions deploy runs on the self-hosted runner labeled `self-hosted`, `linux`, `x64`, `prod`.
- Durable server state lives at `/srv/winrift`.
- ClickHouse data, logs, and backups live on the mounted storage SSD under `/mnt/storage/clickhouse`.
- The API is bound to the private LAN by default. Do not router-forward it.
- The worker starts only after the API passes health.

This intentionally does not create a public web domain. For now, use the laptop frontend against the server API over the private home network.
The web container is intentionally not part of the production Compose file yet because the current frontend Dockerfile is optimized for local Vite development. When we want a private or public hosted frontend, add a production static-web image and either bind it privately or put it behind the same reverse-proxy pattern used by the rest of the home-server stack.

Server layout:

```text
/srv/winrift/
  .env
  schema.sql
  runtime/
  deployments/

/mnt/storage/clickhouse/
  data/
  logs/
  backups/
```

Create the server env from the committed example:

```bash
sudo mkdir -p /srv/winrift
sudo chown -R $USER:$USER /srv/winrift
sudo cp ops/prod/winrift.env.example /srv/winrift/.env
sudo editor /srv/winrift/.env
```

Set the real `RIOT_API_KEY`, `CLICKHOUSE_PASSWORD`, current patch, and platform list in `/srv/winrift/.env`. Keep this file server-local and uncommitted.
The self-hosted GitHub runner user needs write access to `/srv/winrift` and Docker permission.

Before starting ClickHouse on the server, verify the storage mount:

```bash
findmnt /mnt/storage
df -hT /mnt/storage
sudo mkdir -p /mnt/storage/clickhouse/{data,logs,backups}
sudo chown -R $USER:$USER /mnt/storage/clickhouse
```

The deploy workflow also checks that `/mnt/storage` is mounted and refuses to start ClickHouse if the mount is missing. That protects the OS disk from silently receiving database files.

The production Compose file is [ops/prod/docker-compose.yml](../ops/prod/docker-compose.yml). It runs:

- `winrift_clickhouse`
- `winrift_api`
- `winrift_worker`
- `winrift_monitor`

The worker uses `restart: "no"` on purpose. If the Riot key expires and the worker exits, Docker should not repeatedly restart it and spend requests against a bad key.
The monitor uses `restart: unless-stopped` because it is the thing that tells you the worker stopped.

## CI/CD

Core CI is [ci-core.yml](../.github/workflows/ci-core.yml):

1. Run Go tests, vet, and `govulncheck`.
2. Build the core image.
3. Validate the production Compose file.
4. Run Trivy scans and upload an SBOM.
5. On `main`/`master`, push `ghcr.io/adamwentworth/winrift-core:sha-<commit>` and `ghcr.io/adamwentworth/winrift-core:latest` through GitHub Container Registry.

Core deploy is [deploy-core-prod.yml](../.github/workflows/deploy-core-prod.yml):

1. Runs manually through `workflow_dispatch`.
2. Checks `/srv/winrift/.env`.
3. Copies the current ClickHouse schema to `/srv/winrift/schema.sql`.
4. Pulls the requested image.
5. Stops the worker.
6. Starts ClickHouse.
7. Recreates the API and waits for `/api/health`.
8. Recreates the monitor.
9. Starts the worker if `start_worker=true`.
10. Writes deployment metadata to `/srv/winrift/deployments/core.json`.

The deploy input `image_ref` accepts `latest`, `sha-<commit>`, or a full image reference.

For the one-time laptop database bootstrap, use [ops/prod/data-migration.md](../ops/prod/data-migration.md). The important rule is to deploy with `start_worker=false`, verify restored match counts, and only then start the worker.

## Laptop Frontend Against Server API

With `WINRIFT_API_BIND=0.0.0.0`, the API listens on the server's private LAN address. Do not expose or router-forward this port.

Run the frontend locally against the server:

```bash
cd apps/web
VITE_API_URL=http://SERVER_LAN_IP:8000 npm run dev
```

The browser origin is still the laptop's Vite server, so this production CORS value is enough for normal local development:

```text
CORS_ORIGINS=http://localhost:5173,http://127.0.0.1:5173
```

If you want stricter access later, set `WINRIFT_API_BIND=127.0.0.1` and use an SSH tunnel instead.

## Environment

Production needs a server-local `.env`. Do not commit it.

Core values to verify before starting the worker:

```text
RIOT_API_KEY=...
ENVIRONMENT=production
CORS_ORIGINS=http://localhost:5173,http://127.0.0.1:5173
WINRIFT_API_BIND=0.0.0.0
WINRIFT_API_PORT=8000
WINRIFT_STORAGE_MOUNT=/mnt/storage
WINRIFT_CLICKHOUSE_DATA_DIR=/mnt/storage/clickhouse/data
WINRIFT_CLICKHOUSE_LOG_DIR=/mnt/storage/clickhouse/logs
WINRIFT_CLICKHOUSE_BACKUP_DIR=/mnt/storage/clickhouse/backups
WINRIFT_RUNTIME_STATE_DIR=/srv/winrift/runtime
COLLECTOR_PLATFORMS=NA1,EUW1,EUN1,KR,BR1,LA1,LA2,JP1,OC1,TR1,RU,SG2,TW2,VN2
COLLECTOR_CURRENT_PATCH=16.10
COLLECTOR_PATCH_RETENTION_COUNT=2
COLLECTOR_PRUNE_OLD_PATCHES_ON_START=false
RIOT_AUTH_FAILURE_EXIT=true
RIOT_AUTH_FAILURE_MARKER_PATH=/run/winrift/riot-auth-failed
MONITOR_WORKER_REQUIRED=true
MONITOR_WORKER_STALE_AFTER_MINUTES=15
MONITOR_WORKER_CONTAINER_NAME=winrift_worker
MONITOR_DOCKER_SOCKET_PATH=/var/run/docker.sock
MONITOR_STARTUP_GRACE_SECONDS=120
ALERT_EMAIL_ENABLED=true
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=...
SMTP_PASSWORD=...
SMTP_FROM=...
SMTP_TO=you@example.com
```

Keep `COLLECTOR_PRUNE_OLD_PATCHES_ON_START=false` until the patch was archived or backed up. Turn it on deliberately only after `patchctl -action archive` has marked old platforms `closed`; startup pruning now deletes raw payload/timeline detail only, but an explicit archive command is easier to audit.

## Riot Key Failure Behavior

The API and worker share an auth-failure marker at `RIOT_AUTH_FAILURE_MARKER_PATH`.

If Riot returns 401 or 403:

- The component that saw it writes the marker.
- The worker exits when the marker appears.
- The worker writes a heartbeat file under `/run/winrift/worker-heartbeat.json` on startup and after every sweep.
- The API stays up.
- Riot-backed endpoints return a service-unavailable response instead of repeatedly hitting Riot.
- Cached analytics, static pages, and database-backed reads can keep working.
- The monitor sees the marker or stopped worker container and sends an email, then suppresses repeats until `MONITOR_ALERT_COOLDOWN_MINUTES` elapses. Stale heartbeat observations are log-only.

After refreshing the key:

```bash
make up
make up-worker
```

Both API and worker clear the marker on startup. If the API is already running and you only restart the worker, the shared marker is still cleared from the runtime volume, but a full `make up` is the cleanest reset after a key refresh.

If you intentionally pause collection, set `MONITOR_WORKER_REQUIRED=false` or stop the monitor too. Otherwise production treats a stopped worker container as an alert condition.

Riot 404s are not auth failures. A missing Riot ID or a summoner not being in a live game should return a normal not-found/live-game-absent response.

Riot 429s are rate limits. The client sleeps for `Retry-After` when the wait is reasonable, or defers that region when Riot asks for too long.

## Monitor And Email Alerts

The monitor is a tiny Go process built into the core image as `/winrift-monitor`. It checks:

- `GET /api/health`
- `RIOT_AUTH_FAILURE_MARKER_PATH`
- `MONITOR_WORKER_CONTAINER_NAME` through the Docker socket when configured
- `MONITOR_WORKER_HEARTBEAT_PATH`

It sends an alert when an email-worthy check fails and writes a small state file at `MONITOR_ALERT_STATE_PATH` so it does not email every minute. Repeat alerts use `MONITOR_ALERT_COOLDOWN_MINUTES`. `MONITOR_STARTUP_GRACE_SECONDS` suppresses worker-down alerts briefly after the monitor starts so deploy ordering does not send noise. Recoveries are logged and clear monitor state, but they do not send email.

For Gmail or another SMTP provider, use an app password rather than your normal account password:

```text
ALERT_EMAIL_ENABLED=true
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your_email@gmail.com
SMTP_PASSWORD=your_app_password
SMTP_FROM=your_email@gmail.com
SMTP_TO=your_email@gmail.com
```

The monitor container should stay up even when the worker exits. It does not spend Riot API budget.

The worker also writes a compact refresh-health snapshot to `WORKER_REFRESH_STATUS_PATH`. On the server this defaults to `/run/winrift/worker-refresh-status.json` and records the latest start/success/failure time, duration, row/context counts, and last error for each summary refresh lane. It is intentionally simple JSON so it can be inspected with `docker exec`, `ssh`, or a future monitor/API endpoint.

## Deployment Flow

The CI/CD deploy should be the normal server path. A manual emergency deploy can still be:

```bash
cd /srv/winrift
docker compose --project-directory /srv/winrift \
  -f /path/to/repo/ops/prod/docker-compose.yml \
  --env-file /srv/winrift/.env \
  stop worker
```

Then deploy through GitHub Actions, or pull an image and run the same Compose file manually. In both cases:

1. Build and publish images, or pull the repo and build on the server.
2. Start/update ClickHouse, API, and web.
3. Verify `/api/health`.
4. Start the monitor.
5. Start the worker last.
6. Watch `make logs-worker` long enough to confirm platforms, patch window, request budget, and refresh intervals.

The worker should be stopped before risky migrations or config changes. API/web changes can usually deploy while the worker is paused.

## Schema Changes

The current Compose file mounts `core/internal/clickhouse/schema.sql` into ClickHouse init. That init file runs automatically for a fresh database volume.

For an existing production volume, schema changes need an explicit migration step. The Go repository has idempotent table/column creation paths for many runtime tables, but do not rely on that as the only production migration strategy forever.

Near-term rule:

- For additive schema changes, start API against staging/dev first and verify it creates or uses the expected columns.
- For production, pause the worker, deploy API, verify health and logs, then restart the worker.
- For destructive changes, write a one-off migration/runbook before applying it to the production volume.

Longer-term improvement: add a formal migration command so CI/CD can run `migrate up` before starting the API.

## Old Patch Archive

When a patch falls outside the active two-patch collection window, close it on the server before deleting raw payloads:

```bash
cd /srv/winrift
docker compose --env-file /srv/winrift/.env run --rm api \
  /winrift-patchctl -action archive -patch 16.9 -platform ALL -queue 420 -retain-days 0
```

Archive refreshes summaries first, then prunes only bulky raw match/timeline/event rows. It intentionally keeps `participants`, `participant_matchups`, and `participant_performance` until every app page has a dedicated closed-patch read model.

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

For production, [ops/prod/docker-compose.yml](../ops/prod/docker-compose.yml) already uses a stable host path:

```text
WINRIFT_CLICKHOUSE_DATA_DIR=/mnt/storage/clickhouse/data
WINRIFT_CLICKHOUSE_LOG_DIR=/mnt/storage/clickhouse/logs
WINRIFT_CLICKHOUSE_BACKUP_DIR=/mnt/storage/clickhouse/backups
```

Keep hot ClickHouse data on local storage when possible. Use the NAS for backups and archives first. A network mount as the primary ClickHouse data directory can work for hobby traffic, but it is more fragile under write load and network hiccups.

Current local reference point: the dev ClickHouse Docker volume is about `56G`, while active ClickHouse parts are much smaller. Some of that gap is likely merge/temp/detached overhead from heavy ingest and interrupted runs. The server data SSD is about `119G`, so it can run the initial private deployment but does not leave a lot of room for unbounded retention. Before a long unsupervised collection run, prefer either:

- enforce two-patch retention and put backups/closed-patch exports on the NAS, or
- add a larger dedicated local SSD/HDD later and move `WINRIFT_CLICKHOUSE_DATA_DIR` to that mount.

See [Storage Policy](storage-policy.md) for retention, backup, and NAS guidance.
