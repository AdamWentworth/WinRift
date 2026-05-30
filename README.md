# WinRift

WinRift is being rebuilt as a League of Legends build-matchup analytics tool. The new MVP focuses on champion, opponent, role, item, rune, and summoner-spell outcomes from ranked match data.

The previous implementation is preserved under `legacy/` for reference and fixtures.

## Stack

- Go core service in `services/core/`, with separate `api`, `worker`, and `patchctl` entrypoints
- ClickHouse analytics database
- Vite React TypeScript web app in `apps/web/`
- Docker Compose for local development

Go is used for the API/collector because the target deployment is a lightweight home server: small binaries, low idle memory, simple concurrency, and clean rate-limit handling.

## Quick Start

1. Put your Riot key in the root `.env`.
2. Start the stack:

```bash
make up
```

3. Open the app:

```text
http://localhost:5173
```

4. API health is available at:

```text
http://localhost:8000/api/health
```

The collector worker is behind the `worker` Compose profile so normal local startup will not spend Riot API budget. Run it explicitly with `make up-worker`, or call `POST /api/dev/collector/seed` in development. The worker can collect multiple Riot platforms via `COLLECTOR_PLATFORMS`, with per-platform budgets and optional Challenger-ladder auto-seeding.

To pause collection while leaving ClickHouse/API up:

```bash
make stop-worker
```

To fully stop the project, including the profiled worker, use:

```bash
make down
```

This runs `docker compose --profile worker down --remove-orphans`. Avoid relying on profile-less stop/down commands when the worker may have been started, because the profiled worker can be left running by accident.

## Local API

```bash
cd services/core
go run ./cmd/api
```

The API reads `.env` from either the repo root or the core service directory. API keys are never logged by the app.

## Local Web

```bash
cd apps/web
npm install
npm run dev
```

To use a server-hosted API while keeping the frontend local, tunnel the private server API and point Vite at it:

```bash
ssh -N -L 8000:127.0.0.1:8000 your-server
cd apps/web
VITE_API_URL=http://127.0.0.1:8000 npm run dev
```

## Verification

```bash
cd services/core && go test ./...
cd apps/web && npm test && npm run build
```

## Docs

- [Architecture](docs/architecture.md)
- [Riot API Notes](docs/riot-api.md)
- [Data Dictionary](docs/data-dictionary.md)
- [Collector Runbook](docs/collector-runbook.md)
- [Deployment And Operations](docs/ops-deployment.md)
- [Storage Policy](docs/storage-policy.md)
- [Patch Lifecycle](docs/patch-lifecycle.md)
- [ClickHouse Queries](docs/clickhouse-queries.md)
- [Policy-Safe Live UX](docs/policy-safe-live-ux.md)
- [Live Match Experience Roadmap](docs/live-match-experience-roadmap.md)
- [Legacy Win Condition Audit](docs/legacy-win-condition-audit.md)
- [Analytics Philosophy](docs/product/analytics-philosophy.md)
- [Global Background System](docs/product/global-background-system.md)
- [Summoner Profiles](docs/product/summoner-profiles.md)
- [Remaining Work](docs/product/remaining-work.md)
- Discussions:
  - [Match Collection](docs/discussions/match-collection.md)
  - [Build Matchup Analytics](docs/discussions/build-matchup-analytics.md)
  - [Win Conditions](docs/discussions/win-conditions.md)
