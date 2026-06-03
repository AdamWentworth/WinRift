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

Status: first pass implemented as `ops/perf-smoke.sh` plus the manual `perf-smoke` GitHub Actions workflow. It reports threshold warnings by default and can be made strict later. The script now samples each endpoint multiple times, supports a patch selector, and can emit JSONL for trend capture.

Browser route timing is also in place through `apps/web` Playwright checks and the manual `route-perf` workflow. This covers page-level ready time and API request counts, which the API-only smoke cannot see.

Post-deploy smoke is now also part of `deploy-core-prod`: after recreation it checks API/monitor/worker container state, `/api/health`, leaderboard/profile response-cache hits, and worker refresh-status JSON readability when the status file exists.

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

### `core`

Previous shape:

```text
services/
└── core/
```

Status: complete. The Go module now lives at top-level `core/`.

This was less convincing than `apps/web`. The Go module contains multiple runtime entrypoints, but they are one product core:

- API
- worker
- monitor
- patch archive command

They share config, ClickHouse access, Riot client code, analytics, and read models. That is not really a set of separate services yet.

Decision: flatten `services/core` to `core`.

Rationale:

- The repo does not currently have multiple backend services.
- The core module already holds several binaries behind one shared domain model.
- The flatter path is easier to scan in README, CI, Docker, and docs.

Revisit only if we add truly independent backend services later, such as `services/inference`, `services/assets`, or a separate public API gateway.

### `ops/prod`

Keep this.

The `prod` folder is an environment boundary, not empty ceremony. Production Compose, environment examples, migration docs, and deploy helpers belong together. Future `ops/staging` or `ops/local-server` is plausible.

## Priority 3: Frontend Maintainability

### Split `app.css`

Current pass complete: `apps/web/src/styles/app.css` is now an import manifest, and the former single-file stylesheet is split into `apps/web/src/styles/app/` by page surface and stable primitives:

```text
styles/app/
├── foundation.css
├── summoner-profile.css
├── background-stage.css
├── lookup-search.css
├── live-layout.css
├── live-win-conditions.css
├── live-player-cards.css
├── live-focused-builds.css
├── navigation.css
├── champion-directory.css
├── tier-list.css
├── build-guide.css
└── responsive.css
```

Guardrail remains: split by product surface and stable primitives, not by tiny components. The goal is easier scanning, not CSS fragmentation.

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

- item panels and item selection helpers: first pass complete in `components/build-guide/GuideItemPanels.tsx`
- build variant tabs and family grouping: first pass complete in `components/build-guide/BuildVariantTabs.tsx`
- rune, spell, matchup, and skill cards: first pass complete in `components/build-guide/GuideDataCards.tsx`
- hero/header

Summoner profile extraction candidates:

- summoner search hub, leaderboard, and shared profile message: first pass complete in `components/summoner-profile/`
- stored profile tabs, rows, and freshness helpers: second pass complete. `ProfileSections.tsx` owns tab state and summaries, `ProfileRows.tsx` owns champion/build/match row markup, and `profileFormatters.ts` owns shared date/name/freshness formatting.
- profile header: first pass complete in `components/summoner-profile/ProfileHeader.tsx`

Live match extraction candidates:

- focused build participant picker: first pass complete in `components/live-match/BuildParticipantPicker.tsx`, with shared live participant display labeling in `participantLabels.ts`.

### Route-Level Bundle Splitting

Status: first pass complete.

The root shell now lazy-loads the heavy route pages:

- champion guides,
- champion directory,
- live match analysis,
- summoner profiles,
- tier list.

The same pass moved routing, analytics patch storage, champion role helpers, and patch parsing into small `lib/` modules. This keeps `App.tsx` focused on shell orchestration and avoids pulling page code into the first home/search bundle.

Measured production build change from the first split pass:

- main JS bundle: about 389 kB to about 259 kB,
- main gzip size: about 115 kB to about 81 kB.

Next guardrail: keep future page-only features inside their route files or page-local components. Shared shell code should be limited to navigation, search, routing, background, and metadata that is genuinely needed everywhere.

## Priority 4: Backend Maintainability

### Split `server.go`

Current pass complete: `core/internal/api/server.go` now only owns the `Server` type, constructor, route wiring, and health handler. Endpoint logic has been moved into same-package files by domain:

```text
internal/api/
├── server.go              # type, constructor, route wiring, health
├── account_handlers.go
├── analytics_handlers.go
├── dev_handlers.go
├── http_helpers.go
├── live_game_handlers.go
├── summoner_handlers.go
├── win_conditions.go
├── static_handlers.go
└── *_test.go
```

Guardrail remains: do not invent a new framework or heavy abstraction. Keep the existing `net/http` style and same-package helpers until a real abstraction earns its keep.

### Split ClickHouse Query Files Further

The ClickHouse package has grown into the real heart of the app. It is mostly organized already, but large query files should keep moving toward read-model domains:

- champion guides
- item slots and loadouts
- summoner identities/profiles
- win conditions
- patch lifecycle
- performance backfills

This makes it easier to enforce the performance rule: public page endpoints should read summaries, not raw payloads.

Status update: first maintainability split complete.

- `repository.go` now owns only the connection/repository constructor and basic existence check.
- `analytics_scopes.go` owns shared role/opponent scope helpers.
- `ingestion.go` owns normalized match inserts.
- `build_analytics.go` owns build-signature reads.
- `item_slot_analytics.go` owns item slot and starting loadout reads/refreshes.
- `patch_lifecycle.go` owns patch archive/prune/compile helpers.
- `champion_guide.go` owns guide summaries, index, ranking, and impact scoring.
- `champion_guide_builds.go` owns guide item paths and build variants.
- `champion_guide_rows.go` owns matchup, rune/spell/signature, and base SQL helpers.

Build advice now has `build_signature_analytics`, a compact current-patch build-signature read model refreshed by the champion-guide lane. `QueryBuilds` and champion-guide item paths prefer it before falling back to archived `patch_build_metrics` or retained normalized rows. Hot champion-guide pages are also prewarmed into `champion_page_bundle_cache` after guide refreshes, using canonical cache keys so equivalent query params reuse the same bundle.

Remaining smell: `item_slot_analytics.go`, `champion_guide.go`, and `champion_guide_derived.go` are still large because the SQL itself is large. Split further only when there is a real domain boundary, not just to chase a smaller line count.

### Split Worker Lanes

Status: first pass complete.

`core/cmd/worker/main.go` now stays focused on startup and sweep orchestration. Lane-specific code moved into:

```text
cmd/worker/
├── enrichment_lanes.go  # rank and Riot ID alias enrichment
├── frontier_seed.go     # env/challenger frontier seeding
├── refresh_lanes.go     # summary/read-model refresh scheduling
├── request_budget.go    # regional request budget ledger
├── state.go             # auth/rate-limit/heartbeat/frontier helpers
└── main.go              # startup and sweep orchestration
```

Guardrail: keep the worker as one binary while these lanes share rate limits, config, and ClickHouse state. Consider separate processes only if one lane needs independent deployment or a different runtime schedule.

## Priority 5: Test Coverage Gaps

### Frontend Regression Tests

Good candidates:

- summoner ladder renders profile icons when `profileIconId` is present,
- region selector overlay stays anchored,
- champion page default role picks the champion's main role,
- build variants update item/rune/skill panels,
- starting item panel does not show impossible duplicate openers,
- live-match modes gate heavy queries until selected.

Status: first pass in place. App-level and focused component tests now cover the summoner ladder/profile icon path, the anchored summoner region selector, champion main-role routing, build variant item/rune/skill switching shape, impossible duplicate item display, and live-mode query gating.

### Frontend Request And Cache Guardrails

Status: first pass complete. See `docs/product/frontend-performance-audit.md`.

The frontend now has shared React Query defaults, abort-aware API calls, route-gated item/rune/spell metadata, and documented smells for avoiding request waterfalls.

### Backend Query Tests

Good candidates:

- summoner leaderboard uses `summoner_identity_summary` and does not require raw JSON scanning,
- identity summary refresh picks profile icons from account snapshots first and raw profile icons second,
- champion guide bundle cache returns warmed payloads,
- build-advice exact matchup rows do not silently borrow champion-wide data,
- current/recent patch retention preserves summaries before pruning raw data.

Status: expanded coverage is in place for the main public read paths. Repository tests now assert champion guide indexes use the precomputed guide/scope/ban summary tables when present, patch stats include compiled `patch_snapshots` rows after raw rows are pruned, and the page-facing paths use the intended cache/summary tables. Tests also pin champion-page persistent bundle cache reads, summoner leaderboard/profile/champion summaries, build item-slot and starting-loadout summaries, direct win-condition metric reads, summoner recent matches/builds, champion guide matchups/signatures, and build-signature analytics. The suite is intentionally table-name focused so future regressions back toward raw/request-time scans are noisy.

## Priority 6: Observability

### API Timing Logs

Add lightweight request timing logs for API endpoints:

- route,
- status,
- duration,
- response bytes if easy,
- cache hit/miss for bundled endpoints.

This should be structured enough to grep from Docker logs, without adding a full logging stack yet.

Status: first pass complete. The API now logs method, route, status, duration, byte count, cache marker, and slow flag behind `API_REQUEST_LOGS_ENABLED`.

### Worker Refresh Metrics

Worker logs are useful, but we should make refresh health easier to inspect:

- last successful refresh time per summary table,
- row counts written,
- duration,
- last error.

This can start as a ClickHouse table or a small runtime-state JSON file. It would make monitor alerts more precise.

Status: first pass complete. The worker now writes `WORKER_REFRESH_STATUS_PATH` with latest start/success/failure timestamps, duration, row/context counts where available, and last error per refresh lane.

Follow-up audit: `docs/product/production-performance-audit-2026-05-31.md` records the first private-server refresh/timing pass. The deployed worker is healthy and warm analytics reads are fast. The first follow-up pass added short in-memory response caching for summoner profile/leaderboard reads and moved summoner-profile, item-slot/loadout, and win-condition refreshes to staged insert-then-cleanup semantics.

## Priority 7: Product/Data Debt

### Read-Model Coverage Audit

Create a table of every frontend page and the backend read model it relies on.

Goal:

- no page should need raw payload scans,
- no page should fan out into many avoidable requests,
- old patch raw pruning should not break visible historical analytics.

Recent progress: champion guide index, tier-list style reads, and selected champion headers now use `champion_guide_summary_analytics` plus `champion_guide_scope_analytics` instead of aggregating `participant_matchups` on every request. Champion guide matchup rows, rune pages, spell pairs, and build signatures now use read models after the worker refresh runs. Hot champion-page response bundles are prewarmed for common champion/role pages and a bounded set of common exact matchup pages.

Status: first pass complete in `docs/product/read-model-coverage-audit.md`.

Outstanding: keep watching exact matchup cold timings as traffic grows. If the bounded prewarm cap is too small, raise it before adding another read model.

Production timing note: warm champion-page bundles are now fast, but the 2026-05-31 audit still saw several cold or cache-miss champion pages take 1.5-5 seconds. The first follow-up inspection found that most prewarm `skipped` rows were actually unexpired persistent-cache hits, not failed prewarms. Worker logs and refresh status now expose cached counts separately so future audits can distinguish healthy cache reuse from real misses.

### Patch-Scope UX

Patch controls belong only where the visible analytics are actually patch-scoped. They should not live in the global top header because home, summoner lookup, and several live surfaces are not primarily patch-browser pages.

Status: first pass complete for the main patch-browsing surfaces:

- champion guide pages,
- tier list.

The control labels the selected data patch, shows whether the user is on the current patch or intentionally viewing a previous patch, and exposes indexed match counts so thin current-patch samples are not mistaken for empty or broken data.

Future follow-up: decide whether live build mode needs its own small patch context label. If it does, keep it inside the Builds mode panel rather than returning it to the app-wide header.

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

Completed:

- Add frontend CI.
- Add a manual/local performance smoke script.
- Split `app.css` by page surface.
- Split `server.go` by handler domain.
- Extract `BuildGuidePage.tsx` sections.
- Extract `SummonerProfilePage.tsx` sections.
- Add first-pass observability for API timing and worker refresh health.
- Rename `services/core` to `core`.
- Add first-pass frontend regression tests for the highest-risk UI surfaces.
- Add first-pass and expanded backend query tests for read-model usage and patch-retained summaries.
- Add first-pass frontend request/cache guardrails.
- Add first-pass Playwright browser route timing.
- Create the read-model coverage audit.
- Add summoner recent-match and summoner build summaries.
- Add champion matchup/rune/spell read models for cold champion-page loads.
- Reuse champion guide summary read models for selected champion headers.
- Add team-kill summaries for true kill participation in the fast champion guide path.
- Add build-signature analytics for build-advice and champion item paths.
- Add hot champion-page bundle prewarming, including common exact matchup pages.
- Move patch-scope controls out of the global header and into champion guide/tier-list pages.
- Add first-pass win-condition validation endpoint and documentation.
- Split worker lanes by orchestration concern.
- Split the largest ClickHouse repository/query files by read-model domain.
- Add champion role-rate summaries for role discovery and default-role resolution.
- Change champion guide read-model refreshes to insert fresh rows before deleting older compiled rows, avoiding transient empty guide tables during rebuilds.
- Add short response caching for summoner leaderboard/profile reads.
- Apply staged insert-then-cleanup refreshes to summoner-profile, item-slot/loadout, and win-condition lanes.
- Clarify champion-page prewarm status by reporting cached persistent bundle hits separately from newly stored bundles.
- Add post-deploy smoke checks to `deploy-core-prod`.
- Rerun production API perf smoke in warm and no-warmup modes for retained patches after deploy smoke landed.
- Verify the next worker refresh status after the post-deploy smoke rollout; champion guide, item-slot/loadout, summoner-profile, and win-condition lanes completed successfully, with champion-page prewarm reporting cached versus stored bundle counts and zero errors.
- Inspect production win-condition validation output and improve the report so primary strategy matchups are ranked by Wilson-backed directional signal instead of mostly by sample size.
- Add first-pass champion-pair residuals to the win-condition validation report so strategy-pair edges can be checked for teammate/opponent pair artifacts.

Next:

1. Split `champion_guide_derived.go` if the derived refresh logic starts getting touched often.
2. Use high-signal primary strategy and champion-pair residual rows as leads for role-specific overrides and timeline validation.
