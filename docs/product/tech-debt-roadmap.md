# Tech Debt Roadmap

WinRift is now far enough along that the main risk is not missing features. The main risk is letting working prototype surfaces harden into awkward architecture. This document tracks cleanup that would make the repo easier to show, maintain, and extend.

## Priority 1: Public-Repo Polish

### Frontend CI

Add a dedicated web workflow for `apps/web`:

- `npm ci`
- `npm test -- --run`
- `npm run build`

The Go core has good CI now, but the frontend is a major part of the product story. A recruiter or collaborator should see both sides protected.

Status: implemented as `.github/workflows/ci-web.yml`.

Suggested workflow triggers:

- `apps/web/**`
- `.github/workflows/ci-web.yml`
- shared docs/config only when needed

### Automated Performance Smoke Checks

Turn the manual timing habit from `performance-guardrails.md` into a lightweight script or CI/manual workflow.

Status: first pass implemented as `ops/perf-smoke.sh` plus the manual `perf-smoke` GitHub Actions workflow. It reports threshold warnings by default and can be made strict later.

Useful first checks:

- deployed `/api/health`
- `/api/summoners/leaderboard?platform=NA1&limit=50`
- `/api/analytics/champion-guides?patch=16.10&rankBucket=ALL`
- one representative `/api/analytics/champion-page`

This should not block normal commits at first. Make it a manual/private-LAN workflow or local script that reports timings. Once the API is public or consistently reachable from CI, promote it into a gate.

### Secret/Public Readiness Recheck

Before showing the repo widely:

- run a secret scanner,
- confirm ignored local files are not tracked,
- decide whether home-server operational details in docs are acceptable,
- keep production deploy manual and self-hosted-runner-bound.

See `docs/public-release-readiness.md`.

## Priority 2: Repo Shape

### `apps/web`

Current shape:

```text
apps/
└── web/
```

This is a little nested for a one-app repo, but it is defensible if a mobile/native app is genuinely plausible later.

Recommendation: keep `apps/web` for now.

Rationale:

- A future `apps/mobile` is plausible once the web product feels stable.
- The frontend is already documented around `apps/web`.
- Flattening it would touch docs, Compose, Docker, CI, Dependabot, README paths, and local scripts for mostly cosmetic gain.

Revisit if we decide there will not be a mobile app.

### `services/core`

Current shape:

```text
services/
└── core/
```

This is less convincing than `apps/web`. The Go module contains multiple runtime entrypoints, but they are one product core:

- API
- worker
- monitor
- patch archive command

They share config, ClickHouse access, Riot client code, analytics, and read models. That is not really a set of separate services yet.

Options:

1. Keep `services/core`.
   - Best if we expect true sibling services later, such as `services/inference`, `services/assets`, or a separate public API gateway.
   - Least disruptive.

2. Rename `services/core` to `core`.
   - Cleaner and more honest for the current repo.
   - Requires updating Go module/import paths from `winrift/services/core` to `winrift/core`.
   - Requires updating Dockerfiles, Compose files, CI, deploy workflow, Dependabot, docs, Makefile, and README references.

Recommendation: rename to `core` only if we do a deliberate structure cleanup pass. It is not urgent, but it would make the repo feel less over-scaffolded.

### `ops/prod`

Keep this.

The `prod` folder is an environment boundary, not empty ceremony. Production Compose, environment examples, migration docs, and deploy helpers belong together. Future `ops/staging` or `ops/local-server` is plausible.

## Priority 3: Frontend Maintainability

### Split `app.css`

`apps/web/src/styles/app.css` is large enough that it is becoming a maintenance liability.

Suggested split:

```text
styles/
├── base.css
├── background.css
├── forms.css
├── navigation.css
├── live-match.css
├── champion-guide.css
├── summoner-profile.css
├── tier-list.css
└── utilities.css
```

Guardrail: split by product surface and stable primitives, not by tiny components. The goal is easier scanning, not CSS fragmentation.

### Extract Large Page Sections

Large current candidates:

- `BuildGuidePage.tsx`
- `SummonerProfilePage.tsx`

Suggested direction:

- keep page files as orchestration and route-state owners,
- move display sections into focused components,
- move data shaping helpers into `lib/` or page-local helper files,
- keep shared UI primitives small and boring.

Build guide extraction candidates:

- hero/header
- rune panel
- item slots
- matchup cards
- build variants
- skill path

Summoner profile extraction candidates:

- profile header
- overview metrics
- champion table
- recent matches
- build usage
- ladder/search hub

## Priority 4: Backend Maintainability

### Split `server.go`

`services/core/internal/api/server.go` is large. It still works, but it is hard to navigate.

Suggested split:

```text
internal/api/
├── server.go              # route wiring, middleware, shared response helpers
├── account_handlers.go
├── live_game_handlers.go
├── summoner_handlers.go
├── champion_handlers.go
├── build_handlers.go
├── win_condition_handlers.go
├── static_handlers.go
└── dev_handlers.go
```

Guardrail: do not invent a new framework or heavy abstraction. Keep the existing `net/http` style and move related handlers into smaller files.

### Split ClickHouse Query Files Further

The ClickHouse package has grown into the real heart of the app. It is mostly organized already, but large query files should keep moving toward read-model domains:

- champion guides
- item slots and loadouts
- summoner identities/profiles
- win conditions
- patch lifecycle
- performance backfills

This makes it easier to enforce the performance rule: public page endpoints should read summaries, not raw payloads.

## Priority 5: Test Coverage Gaps

### Frontend Regression Tests

Good candidates:

- summoner ladder renders profile icons when `profileIconId` is present,
- region selector overlay stays anchored,
- champion page default role picks the champion's main role,
- build variants update item/rune/skill panels,
- starting item panel does not show impossible duplicate openers,
- live-match modes gate heavy queries until selected.

### Backend Query Tests

Good candidates:

- summoner leaderboard uses `summoner_identity_summary` and does not require raw JSON scanning,
- identity summary refresh picks profile icons from account snapshots first and raw profile icons second,
- champion guide bundle cache returns warmed payloads,
- build-advice exact matchup rows do not silently borrow champion-wide data,
- current/recent patch retention preserves summaries before pruning raw data.

## Priority 6: Observability

### API Timing Logs

Add lightweight request timing logs for API endpoints:

- route,
- status,
- duration,
- response bytes if easy,
- cache hit/miss for bundled endpoints.

This should be structured enough to grep from Docker logs, without adding a full logging stack yet.

### Worker Refresh Metrics

Worker logs are useful, but we should make refresh health easier to inspect:

- last successful refresh time per summary table,
- row counts written,
- duration,
- last error.

This can start as a ClickHouse table or a small runtime-state JSON file. It would make monitor alerts more precise.

## Priority 7: Product/Data Debt

### Read-Model Coverage Audit

Create a table of every frontend page and the backend read model it relies on.

Goal:

- no page should need raw payload scans,
- no page should fan out into many avoidable requests,
- old patch raw pruning should not break visible historical analytics.

### Patch-Scope UX

We added patch filters, but this needs a consistency pass:

- champion pages,
- tier list,
- summoner profiles,
- live match build context,
- win-condition metrics.

The UI should make it obvious when the current patch has thin data and when the user is intentionally viewing the previous patch.

### Build Variant Validation

Build variants are powerful but need ongoing validation:

- labels should merge similar item families instead of duplicating names,
- recommended should use the strongest broad sample,
- alternative variants should intentionally stay in their own lane,
- skills/runes/items should update together when selecting variants,
- tiny samples should be visibly tentative.

## Priority 8: Nice Later

### Mobile App

If we build mobile later, keep it product-specific rather than a straight port of the dense web UI.

Likely mobile shape:

- live-game lookup,
- focused player build advice,
- compact win-condition read,
- saved summoners/champions,
- push notifications only if policy-safe and useful.

This is the main reason to keep `apps/web` instead of flattening it immediately.

### Asset Cache

The current background system loads Riot/Data Dragon art remotely. That is fine for MVP, but a deployed version may benefit from a server-side or mounted asset cache outside git.

See `docs/product/global-background-system.md`.

## Recommended Next Cleanup Order

1. Add frontend CI.
2. Add a manual/local performance smoke script.
3. Split `app.css` by page surface.
4. Split `server.go` by handler domain.
5. Extract `BuildGuidePage.tsx` sections.
6. Extract `SummonerProfilePage.tsx` sections.
7. Decide whether to rename `services/core` to `core`.
8. Add observability for API timing and worker refresh health.
