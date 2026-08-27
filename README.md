<p align="center">
  <img src="apps/web/public/images/brand/winrift-icon-wide-2.png" alt="WinRift" width="560">
</p>

<h1 align="center">Fast, Patch-Aware League Analytics</h1>

<p align="center">
  <a href="https://github.com/AdamWentworth/WinRift/actions/workflows/ci-core.yml"><img src="https://github.com/AdamWentworth/WinRift/actions/workflows/ci-core.yml/badge.svg?branch=master" alt="Core CI"></a>
  <a href="https://github.com/AdamWentworth/WinRift/actions/workflows/ci-web.yml"><img src="https://github.com/AdamWentworth/WinRift/actions/workflows/ci-web.yml/badge.svg?branch=master" alt="Web CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-all%20rights%20reserved-00c8dc" alt="All rights reserved"></a>
  <a href="https://github.com/AdamWentworth/WinRift/releases"><img src="https://img.shields.io/github/v/release/AdamWentworth/WinRift?display_name=tag" alt="Latest release"></a>
</p>

WinRift is a solo portfolio application designed, built, and operated by **Adam Wentworth**. It is a production-capable, full-stack analytics project for League of Legends ranked data: it collects Riot match and timeline payloads, normalizes player builds and matchup context into ClickHouse, and serves a React app focused on practical build, rune, champion, summoner, and win-condition analysis.

The repository is public so the engineering and product work can be reviewed. It is not an open-source project, is not accepting outside contributions, and grants no permission to run, reuse, redistribute, deploy, or derive work from the code. See [LICENSE](LICENSE).

The current rebuild is centered on one product idea: the best build is often matchup-specific. WinRift compares what champions build overall with what performs into the champion they are actually facing.

The original 2023 prototype has been retired from the working tree. Its useful product ideas, especially the win-condition model, are preserved in docs while the active runtime is the Go + ClickHouse + React implementation below.

---

## 📸 Screenshots

These screenshots use a deterministic local mock API for public documentation; production data is collected privately through the worker.

| Universal lookup | Champion guide |
|------------------|----------------|
| ![WinRift universal lookup homepage](docs/assets/screenshots/homepage.png) | ![WinRift champion guide page](docs/assets/screenshots/champion-guide.png) |

| Live match scout |
|------------------|
| ![WinRift live match analysis page](docs/assets/screenshots/live-match.png) |

---

## 📦 Monorepo Structure

```plaintext
WinRift/
├── apps/
│   └── web/                 # Vite + React + TypeScript frontend
├── core/                    # Go API, collector worker, monitor, patch archive tool
├── docs/                    # Architecture, product notes, runbooks, data docs
├── ops/
│   └── prod/                # Private server Docker Compose and deploy notes
├── .github/workflows/       # CI/CD for core images and private deployment
├── docker-compose.yml       # Local ClickHouse/API/web/worker stack
├── Makefile                 # Common local Docker commands
└── .env.example             # Safe local configuration template
```

---

## 🧰 Tech Stack Overview

| Layer | Tech | Notes |
|-------|------|-------|
| Frontend | Vite, React, TypeScript, TanStack Query | Live match, champion guides, tier list, summoner profiles |
| Core API | Go `net/http` | Typed handlers, Riot proxying, static-data cache, analytics read APIs |
| Collector | Go worker | Riot rate-limit aware match, timeline, rank, and alias ingestion |
| Monitor | Go worker-side monitor | API health, auth marker, worker container state, heartbeat logging, and SMTP alerting |
| Analytics Store | ClickHouse | Raw payload retention plus normalized tables and precomputed read models |
| Deployment | Docker Compose, GHCR, GitHub Actions | Private-LAN web, API, worker, monitoring, and ClickHouse deployment to a lightweight home server |
| Static Assets | Riot Data Dragon CDN | Champion, item, rune, spell, splash, and profile icon images |

Go is used for the API and collector because the target deployment is a lightweight home server: small binaries, low idle memory, straightforward concurrency, and explicit control over Riot API rate limits.

---

## ✨ Product Surface

- **Universal lookup**: search a champion or Riot ID from the homepage.
- **Live match scout**: resolve a Riot ID, detect live-game status, display participants, ranks, roles, spells, runes, and focused modes.
- **Focused build mode**: compare a searched player’s champion-wide build baseline against matchup-specific item paths.
- **Champion guides**: show role-aware runes, spells, starting items, item paths, skill orders, matchup context, and build variants.
- **Champion directory**: browse champions alphabetically with guide links.
- **Tier list**: rank champions by role using winrate, pick/ban pressure, and normalized performance signals.
- **Summoner profiles**: show stored ranked summaries, champion performance, and recently used builds.
- **Win-condition mode**: revive the legacy five-axis model: split push, pick, siege, control, and teamfight.

---

## 🧭 Project Status

WinRift `v0.1.0` is a working private-LAN product and Adam Wentworth's public, view-only source portfolio. The server runs the production web application, API, ClickHouse, monitoring, and Riot-aware collector; GitHub Actions test, scan, package, deploy, and performance-check the system.

The app is not yet a public internet service. Current production use is private-LAN first: the server owns the production web application, API, ClickHouse, monitoring, and worker collection, while Vite remains available for laptop development against that private API.

Near-term work:

- tighten public deployment/auth policy before any internet-facing API,
- continue validating tier-list and win-condition scoring against larger samples,
- keep current and archived champion-page performance inside the enforced deploy budgets,
- expand focused product validation without reintroducing request waterfalls or on-demand analytics scans.

### Performance Baseline

The August 27, 2026 production audit exercised every selectable champion/patch page combination after startup prewarming:

| Requests | Cache hits | Average | p95 | Maximum |
|----------|------------|--------:|----:|--------:|
| 1,557 | 1,557 | 11.6 ms | 20.3 ms | 40.1 ms |

The production deployment also runs a strict Playwright route gate for champion guides and other high-traffic pages. A failed readiness, cache-hit, completeness, or latency assertion blocks the web deployment. See [Performance Guardrails](docs/product/performance-guardrails.md).

---

## 🚀 Getting Started

### 1. Configure Environment

```bash
cp .env.example .env
editor .env
```

Set at least:

```env
RIOT_API_KEY=replace_with_your_riot_development_key
COLLECTOR_CURRENT_PATCH=16.17
```

The real `.env` is intentionally ignored. Do not commit Riot keys, ClickHouse passwords, or production LAN settings. `COLLECTOR_CURRENT_PATCH` is an operator-controlled ingestion boundary; update it deliberately when Riot advances to a new patch.

### 2. Start Local App Without Collector

```bash
make up
```

Open:

- Web app: `http://localhost:5173`
- API health: `http://localhost:8000/api/health`
- ClickHouse HTTP: `http://localhost:8123`

The collector is not started by default. That protects Riot API budget during normal frontend/backend development.

### 3. Start Collection Explicitly

```bash
make up-worker
make logs-worker
```

Stop only the worker while leaving ClickHouse and API up:

```bash
make stop-worker
```

Stop the full local stack, including the profiled worker:

```bash
make down
```

Run the local monitor profile when testing alert behavior:

```bash
make up-monitor
make logs-monitor
```

---

## 🧪 Local Development

### Core API

```bash
cd core
go run ./cmd/api
```

### Collector Worker

```bash
cd core
go run ./cmd/worker
```

### Patch Archive Tool

```bash
cd core
go run ./cmd/patchctl -action archive -patch 16.9 -platform ALL -queue 420 -retain-days 0
```

### Web App

```bash
cd apps/web
npm install
npm run dev
```

To run the frontend locally against a server-hosted private-LAN API:

```bash
cd apps/web
VITE_API_URL=http://SERVER_LAN_IP:8000 npm run dev
```

For day-to-day laptop work, put that value in ignored local Vite env:

```bash
cp apps/web/.env.example apps/web/.env.local
```

---

## ⚙️ Environment Overview

| Key | Purpose |
|-----|---------|
| `RIOT_API_KEY` | Riot development or production key. Never committed. |
| `COLLECTOR_PLATFORMS` | Comma-separated Riot platforms to collect from. |
| `COLLECTOR_CURRENT_PATCH` | Current patch collection target. |
| `COLLECTOR_PATCH_RETENTION_COUNT` | Number of recent raw patches to retain. |
| `COLLECTOR_RATE_LIMIT_REQUESTS` | Per-window request budget per Riot route region. |
| `COLLECTOR_RATE_LIMIT_WINDOW_SECONDS` | Rate-limit window, commonly `120`. |
| `RIOT_AUTH_FAILURE_EXIT` | Makes the worker stop on 401/403 auth failures. |
| `API_REQUEST_LOGS_ENABLED` | Enables lightweight API timing logs. |
| `API_SLOW_REQUEST_MS` | Marks API request log lines as slow after this duration. |
| `MONITOR_WORKER_STALE_AFTER_MINUTES` | Heartbeat age before stale-worker log observations. |
| `WORKER_REFRESH_STATUS_PATH` | Runtime JSON file with latest worker summary-refresh health. |
| `ANALYTICS_REFRESH_SCHEDULER_INTERVAL_SECONDS` | Background refresh cadence; the worker runs at most one due summary/prewarm family per tick. |
| `MONITOR_WORKER_CONTAINER_NAME` | Optional Docker container name for actual worker-down alerts. |
| `MONITOR_STARTUP_GRACE_SECONDS` | Grace window that avoids worker-down email noise during monitor/deploy startup. |
| `ALERT_EMAIL_ENABLED` | Enables SMTP alerts from the monitor. |
| `CLICKHOUSE_*` | ClickHouse host, port, database, user, and password. |
| `CORS_ORIGINS` | Browser origins allowed to call the API. |
| `VITE_API_URL` | Frontend API base URL. |

See [.env.example](.env.example) and [ops/prod/winrift.env.example](ops/prod/winrift.env.example) for the full set.

---

## 🔁 Data Flow

```mermaid
flowchart LR
  Seeds[Riot IDs / frontier PUUIDs] --> Worker[Go collector worker]
  Worker --> Riot[Riot APIs]
  Riot --> Raw[(raw_matches / raw_timelines)]
  Raw --> Normalized[(participants / matchups / events)]
  Normalized --> Summaries[(ClickHouse read models)]
  Summaries --> API[Go API]
  API --> Web[React frontend]

  Worker -->|401/403| Stop[auth marker + worker exit]
  Stop --> Monitor[monitor email alert]
  Worker -->|429 Retry-After| Backoff[rate-limit sleep]
```

Collection stores enough raw data for current and recent patches, but the app is designed to rely on summary/read-model tables so old raw payloads can be archived or pruned without losing MVP analytics.

---

## 🔌 API Surface

| Endpoint | Purpose |
|----------|---------|
| `GET /api/health` | Health check |
| `POST /api/account/resolve` | Riot ID to PUUID/profile resolution |
| `GET /api/live-game` | Live game lookup by Riot ID |
| `GET /api/summoner/profile` | Stored summoner overview and build usage |
| `GET /api/analytics/champion-page` | Bundled champion page payload |
| `GET /api/analytics/champion-guides` | Champion guide index and tier-list source data |
| `GET /api/analytics/build-advice` | Champion-wide and matchup-specific build advice |
| `GET /api/analytics/builds` | Aggregate build/rune/spell rows |
| `POST /api/analytics/win-conditions` | Win-condition pairing read model |
| `GET /api/static/{kind}` | Data Dragon champions, items, runes, summoner spells |
| `POST /api/dev/collector/seed` | Local/dev seed endpoint |

Development-only endpoints are intended for local or private-LAN use, not public traffic.

---

## 📚 Documentation Map

| Workspace | README |
|-----------|--------|
| Release history | [CHANGELOG.md](CHANGELOG.md) |
| Documentation index | [docs/README.md](docs/README.md) |
| Web app | [apps/web/README.md](apps/web/README.md) |
| Go core service | [core/README.md](core/README.md) |
| Production ops | [ops/prod/README.md](ops/prod/README.md) |

| Area | Docs |
|------|------|
| System architecture | [docs/architecture.md](docs/architecture.md) |
| Riot API behavior | [docs/riot-api.md](docs/riot-api.md) |
| Data model | [docs/data-dictionary.md](docs/data-dictionary.md) |
| Collector operations | [docs/collector-runbook.md](docs/collector-runbook.md) |
| ClickHouse queries | [docs/clickhouse-queries.md](docs/clickhouse-queries.md) |
| Patch retention | [docs/patch-lifecycle.md](docs/patch-lifecycle.md) |
| Deployment | [docs/ops-deployment.md](docs/ops-deployment.md) |
| Storage policy | [docs/storage-policy.md](docs/storage-policy.md) |
| Live UX policy | [docs/policy-safe-live-ux.md](docs/policy-safe-live-ux.md) |
| Public repo readiness | [docs/public-release-readiness.md](docs/public-release-readiness.md) |
| Product roadmap | [docs/product/remaining-work.md](docs/product/remaining-work.md) |

Product-specific docs:

- [Build Guides](docs/product/build-guides.md)
- [Tier List Ranking](docs/product/tier-list-ranking.md)
- [Summoner Profiles](docs/product/summoner-profiles.md)
- [Analytics Philosophy](docs/product/analytics-philosophy.md)
- [Global Background System](docs/product/global-background-system.md)

Discussion notes:

- [Match Collection](docs/discussions/match-collection.md)
- [Build Matchup Analytics](docs/discussions/build-matchup-analytics.md)
- [Win Conditions](docs/discussions/win-conditions.md)
- [Legacy Win Condition Audit](docs/legacy-win-condition-audit.md)

---

## 🧪 Verification

Core tests:

```bash
cd core
go test ./...
```

Web tests and production build:

```bash
cd apps/web
npm test
npm run build
```

Production route timing against an available API:

```bash
cd apps/web
npm run perf:routes
```

Local Docker smoke:

```bash
make up
curl http://localhost:8000/api/health
```

---

## 🔐 Security And API Safety

- Real Riot keys live only in local/server `.env` files.
- The worker exits on Riot auth failure when `RIOT_AUTH_FAILURE_EXIT=true`.
- The monitor can email on Riot auth failure, API failure, or a worker container that is actually down. Stale heartbeat observations stay in logs.
- Riot 429 responses honor `Retry-After` and back off before continuing.
- The collector has per-platform request budgeting and reserve capacity.
- The public app should present contextual statistics, not automated real-time commands.
- Production API/worker deployment is currently private-LAN oriented, not a public internet service.

WinRift is not endorsed by Riot Games and does not reflect the views or opinions of Riot Games or anyone officially involved in producing or managing Riot Games properties.

This repository is visible for portfolio review, but it is not open source and is not accepting contributions. Copyright and project ownership belong to Adam Wentworth. Running, cloning for use, deployment, redistribution, or derivative work requires prior written permission; see [LICENSE](LICENSE).

Riot-owned names, icons, splash art, and screenshots containing Riot imagery are not licensed by this repository; see [NOTICE.md](NOTICE.md).

---

## 🗃️ Storage And Patch Lifecycle

WinRift keeps raw match/timeline payloads for the current and most recent patches, while summary tables preserve app-facing analytics for archived patches. This keeps ClickHouse storage practical on small hardware while allowing trends to remain visible by patch.

Typical lifecycle:

1. Collect current patch raw data.
2. Refresh read models every few minutes.
3. When a new patch arrives, retain the prior patch as recent history.
4. Archive older raw payloads after summaries are materialized.
5. Continue serving archived patch summaries from smaller read-model tables.

---

## 🌐 Deployment Model

The current production shape is intentionally private:

- The production web app, ClickHouse, API, and worker run on the home server.
- The monitor runs beside them and sends private SMTP alerts.
- The worker collects from the server only.
- The web app is available on the private LAN and proxies `/api` to the internal API service.
- The frontend can still be developed on a laptop while pointing to the server API.
- GitHub Actions build, scan, publish, and deploy immutable core and web images through a self-hosted runner.

See [ops/prod/README.md](ops/prod/README.md) and [docs/ops-deployment.md](docs/ops-deployment.md).

---

## 🧠 Built By Adam Wentworth

I designed and built WinRift as a solo portfolio project, from its product direction and frontend through its Go services, analytics model, collection pipeline, performance guardrails, deployment, and operations. It started as an experiment in modeling League team win conditions. The rebuild keeps that strategic idea, but puts the foundation on matchup-specific builds, trustworthy data collection, and precomputed analytics that can load quickly in a real app.

The first production milestone is complete. The next phase is less about proving the architecture and more about maintaining trustworthy data, fast pages, disciplined public exposure, and strong product judgment as the corpus grows.
