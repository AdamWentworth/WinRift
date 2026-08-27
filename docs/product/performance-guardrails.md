# Performance Guardrails

WinRift should feel instant for stored analytics pages. Riot-dependent live lookups can take longer, but pages backed by collected data should not repeatedly scan raw match JSON or rebuild large aggregates during a request.

## Request-Path Rules

- Public page endpoints should read from compact read models, summary tables, or explicit response caches.
- Request handlers should not parse `raw_matches.raw_json` or `raw_timelines.raw_json`.
- Request handlers should not rebuild whole-patch aggregates.
- Expensive ClickHouse work belongs in the worker refresh lanes, patch archive commands, or explicit dev/admin refresh jobs.
- Frontend pages should prefer bundled endpoints over request waterfalls when several panels always load together.

## Current Read Models

- Champion pages: `champion_page_bundle_cache`, champion role analytics, champion guide summary/scope analytics, team-kill summaries, matchup analytics, rune/spell signature analytics, build-signature analytics, item slot analytics, starting loadout analytics, skill analytics, ban analytics, and build variant analytics.
- Champion directory, role detection, tier lists, and champion guide indexes: `champion_role_analytics`, `champion_guide_summary_analytics`, and `champion_guide_scope_analytics` for role/rank/patch-scoped counts.
- Summoner profiles: `summoner_profile_summary`, `summoner_champion_summary`, `summoner_champion_role_summary`, `summoner_recent_match_summary`, and `summoner_build_summary`.
- Summoner identity and ladder rows: `summoner_identity_summary`, rank snapshots, and profile summaries.
- Win conditions: `match_team_win_conditions` and `patch_win_condition_metrics`.

The champion guide summary read model uses `team_kill_summary` for true kill participation. Keep that helper table refreshed before `champion_guide_summary_analytics`; otherwise the API will still work, but impact scoring loses one of its better context signals.

Build advice and champion item paths use `build_signature_analytics` for current/recent patches and `patch_build_metrics` for archived patches. Keep this table refreshed with the champion-guide lane so first-load champion pages do not scan `participant_matchups` just to assemble common build signatures.

Summary refreshes should stage new rows before deleting older compiled rows. Champion-guide, summoner-profile, item-slot/loadout, and win-condition lanes now insert a fresh snapshot into ReplacingMergeTree read models, then remove rows with an older `compiled_at`. Public reads keep seeing the previous snapshot during the rebuild, and a failed refresh leaves the old snapshot intact instead of briefly exposing empty tables.

At startup, before Riot collection, the worker prewarms every canonical current-patch champion-page bundle into `champion_page_bundle_cache` with bounded concurrency. It reads the completed bundle metadata and warms the mature fallback only for champions whose current guide has no games—the exact set the frontend can request automatically. After current-patch readiness, it prewarms every canonical champion page for every selectable archived patch. Each canonical bundle is also stored under an automatic-role alias, allowing the normal no-role frontend request to hit persistent cache without first querying ClickHouse for role resolution. The API bulk-hydrates every selectable patch and shares immutable payload bytes between resolved-role and automatic-role aliases. Production item-slot and opening-loadout reads are summary-only, so a public cache miss never launches a raw timeline scan; development and explicit maintenance jobs retain the diagnostic fallback. Closed patches also treat an empty compiled result as authoritative after raw payload retirement. This is ClickHouse/local cache work only; it does not spend Riot API budget. Retained-patch bundles expire after two hours so they follow refreshes; archived-patch bundles remain reusable for 30 days because their source data is stable. The deploy and scheduled performance gates test every champion/patch combination and require a cache hit under the configured latency ceiling.

Exact opponent-filtered champion pages are the expensive cold path because they assemble the same guide bundle plus matchup-specific item panels. The worker therefore prewarms a bounded number of common opponent bundles for retained patches only. Keep `CHAMPION_PAGE_PREWARM_MAX_MATCHUP_BUNDLES` conservative; the default is `100` per patch. Simultaneous misses for the same page are coalesced into one build, so a burst of first visitors cannot multiply the same ClickHouse work. Warm responses should be effectively instant, but the first build of an obscure, uncached exact matchup can still take seconds.

## Profiling Checklist

Before treating a page as polished, test the deployed API with `curl` timing output:

```bash
curl -o /tmp/winrift.out -s -w \
  'status=%{http_code} ttfb=%{time_starttransfer} total=%{time_total} size=%{size_download}\n' \
  'http://SERVER_LAN_IP:8000/api/summoners/leaderboard?platform=NA1&limit=50'
```

Or run the project smoke script:

```bash
WINRIFT_PERF_BASE_URL=http://SERVER_LAN_IP:8000 \
WINRIFT_PERF_PATCH=16.10 \
WINRIFT_PERF_RUNS=3 \
ops/perf-smoke.sh
```

There is also a manual GitHub Actions workflow, `perf-smoke`, that runs the same script from the private self-hosted production runner. It defaults to warning on threshold misses rather than failing, because this is currently a regression detector and tuning aid. Set `strict_thresholds=true` when the measured endpoints are consistently under their targets.

Useful local env vars:

- `WINRIFT_PERF_BASE_URL`: API base URL.
- `WINRIFT_PERF_PATCH`: patch used by champion/tier analytics checks. Defaults to `16.10` while the current patch is still filling.
- `WINRIFT_PERF_RUNS`: measured runs per endpoint. Defaults to `3`.
- `WINRIFT_PERF_WARMUPS`: warmup requests before measured runs. Defaults to `1`.

For exact matchup cold-path checks, temporarily run with `WINRIFT_PERF_WARMUPS=0`. A cold miss can still be slower than a warmed page, but common matchup pages should stop showing multi-second first loads after the worker prewarm lane has run.
- `WINRIFT_PERF_JSONL`: optional file path for machine-readable JSONL output.
- `WINRIFT_PERF_STRICT`: fail on threshold warnings when set to `1` or `true`.

The strict all-champion audit checks the actual canonical request used by `/champions/<champion>` for every champion in static metadata. It requires an API cache hit, a complete bundled response, a resolved role, and a total response time at or below 500 ms. When the current-patch guide is empty, it also checks the same mature-patch fallback the frontend will request and requires that fallback to contain guide data.

```bash
python3 ops/prod/champion-page-perf-audit.py \
  --base-url http://SERVER_LAN_IP:8000 \
  --max-ms 500 \
  --concurrency 4 \
  --json champion-page-performance.json
```

`deploy-core-prod` waits for explicit canonical-startup prewarm completion logs for both the current patch and mature fallback, then runs this audit as a required deployment gate. The manual `perf-smoke` workflow runs it too. A missing prewarm entry, slow response, incomplete bundle, or empty fallback fails the workflow rather than being downgraded to a warning.

Targets for private-LAN reads:

- Static metadata: under 100 ms after warmup.
- Summary-backed lists: under 300 ms after warmup.
- Bundled champion/profile pages: under 500 ms after warmup.
- Live Riot lookups: allowed to be slower, but must respect Riot auth/rate-limit guardrails.

## Regression Smell

If an analytics endpoint suddenly takes seconds, first check for:

- a raw JSON table in the hot query,
- a missing summary refresh,
- a delete-before-insert refresh that briefly exposes an empty read model,
- a browser request waterfall,
- a cache key that is too specific or not being reused,
- a `FINAL` query over a large table where a compact summary would work.

The fix should usually be a new summary/read-model refresh, not more frontend loading spinners.

Frontend-specific request and cache rules are tracked in [Frontend Performance Audit](./frontend-performance-audit.md).

## Browser Route Timing

API timing alone is not enough. A route can have fast endpoints and still feel slow if the browser creates request waterfalls, waits on unnecessary metadata, or remounts too much UI.

Run the Playwright route timing smoke from `apps/web`:

```bash
WINRIFT_ROUTE_PERF_API_URL=http://SERVER_LAN_IP:8000 \
npm run perf:routes
```

The route smoke builds the production frontend, serves it through a tiny local static/proxy server, opens the core routes in Chromium, waits for page-specific ready markers, records API request counts, and writes:

```text
apps/web/test-results/route-performance.json
```

Useful env vars:

- `WINRIFT_ROUTE_PERF_API_URL`: backend API URL proxied by the local route-perf server. Use the server API from the dev laptop.
- `WINRIFT_ROUTE_PERF_CLIENT_API_URL`: optional API URL injected directly into the browser build. Leave unset for the same-origin proxy path.
- `WINRIFT_ROUTE_PERF_STRICT`: fail if route ready time or request count exceeds its budget.
- `WINRIFT_ROUTE_PERF_JSON`: output path for the JSON timing report.
- `WINRIFT_ROUTE_PERF_PORT`: local browser timing server port. Defaults to `4173`.
- `WINRIFT_ROUTE_PERF_SKIP_SERVER`: set to `1` when testing against an already-running frontend.
- `WINRIFT_ROUTE_PERF_REUSE_SERVER`: set to `1` only when you intentionally want Playwright to reuse an existing frontend server.

There is also a manual `route-perf` GitHub Actions workflow for the private self-hosted runner. Keep this manual for now; Chromium install and private-LAN access are heavier than the normal web CI path.

The current frontend route split keeps page-scale JavaScript out of the initial home/search shell. The largest always-loaded bundle should stay small enough for quick first interaction, while champion guides, live match analysis, profiles, and tier lists load in their own chunks. The global background still renders immediately from lightweight metadata, and the full splash-art catalog is fetched after roughly 10 seconds so route timing does not pay for art discovery before the page is usable.

## Baseline: 2026-05-31

Environment:

- Dev laptop frontend perf runner.
- Server API at `http://SERVER_LAN_IP:8000`.
- Patch scope `16.10`.
- API smoke used 2 warmups and 5 measured runs.
- Browser route timing used the production web build through the Playwright perf server.

API smoke:

| Endpoint | Avg | Max | Notes |
| --- | ---: | ---: | --- |
| Health | 3 ms | 4 ms | Clear |
| Patch list | 60 ms | 71 ms | Clear |
| Summoner leaderboard | 72 ms | 85 ms | Clear |
| Champion role rates | 129 ms | 194 ms | Slowest sampled API endpoint, still under budget |
| Champion guide index | 39 ms | 42 ms | Clear |
| Aatrox champion page | 13 ms | 13 ms | Warm cached route with explicit role |

Browser route timing, second pass:

| Route | Ready | API Requests | Slowest Request | Notes |
| --- | ---: | ---: | ---: | --- |
| Home search | 703 ms | 2 | 87 ms | Clear |
| Champion directory | 1,939 ms | 2 | 218 ms | Clear on second pass; first pass was 3,866 ms and warned |
| Aatrox guide | 1,393 ms | 6 | 442 ms | Clear; no loading item panels by ready marker |
| Tier list | 1,133 ms | 3 | 102 ms | Clear |
| Summoners hub | 939 ms | 3 | 91 ms | Clear |

Interpretation:

- Stored analytics API reads are no longer the obvious bottleneck.
- Champion pages are benefiting from response caching/prewarming. A matching no-role Aatrox page request measured around 183 ms immediately after the route run, which is still under the bundled-page target.
- Browser route timing still shows some cold variability, especially champion directory and patch-list requests through the proxy. Treat one-off cold warnings as investigation leads, not failures.
- Do not make thresholds strict yet. Collect a few more baseline runs after normal worker refresh/prewarm cycles before turning perf warnings into hard CI gates.

## Production Refresh Audit: 2026-05-31

A private-server audit is captured in `docs/product/production-performance-audit-2026-05-31.md`.

Important takeaways:

- Worker collection and all summary refresh lanes were healthy.
- Warm champion-page requests were in the low tens of milliseconds.
- Cold champion-page and matchup bundle requests still reached 1.5-5 seconds.
- Summoner profile cold reads reached 2.8-13.1 seconds.
- Follow-up work added summoner profile/leaderboard response caching and staged insert-then-cleanup refreshes for summoner-profile, item-slot/loadout, and win-condition summaries.

Keep performance investigations focused on cold misses, cache-key coverage, and refresh windows rather than adding more frontend loading states.

## API Smoke Baseline: 2026-06-01

Environment:

- Server API at `http://SERVER_LAN_IP:8000`.
- Run after `deploy-core-prod` gained post-deploy smoke checks, response-cache checks, and refresh-status readability.
- Each API smoke used 5 measured runs.
- Warm mode used 2 warmup requests before measurement.
- No-warmup mode used `WINRIFT_PERF_WARMUPS=0`; this does not purge persistent champion-page bundle cache, so it represents normal production first-click behavior after worker prewarming rather than a fully empty cache.

Result: all sampled endpoints passed with zero warnings on both retained patches.

### Patch 16.10

| Endpoint | Warm avg/max | No-warmup avg/max | Read |
| --- | ---: | ---: | --- |
| Health | 3 / 4 ms | 3 / 5 ms | Good |
| Patch list | 89 / 112 ms | 167 / 197 ms | Good |
| Summoner leaderboard | 5 / 6 ms | 6 / 8 ms | Good; response cache is working |
| Champion role rates | 38 / 49 ms | 23 / 29 ms | Good |
| Champion guide index | 58 / 78 ms | 59 / 84 ms | Good |
| Aatrox champion page | 14 / 17 ms | 14 / 16 ms | Good |
| Kled matchup page | 13 / 14 ms | 17 / 32 ms | Good |
| Lee Sin matchup page | 14 / 18 ms | 14 / 17 ms | Good |

### Patch 16.11

| Endpoint | Warm avg/max | No-warmup avg/max | Read |
| --- | ---: | ---: | --- |
| Health | 6 / 10 ms | 3 / 4 ms | Good |
| Patch list | 105 / 145 ms | 75 / 99 ms | Good |
| Summoner leaderboard | 6 / 8 ms | 6 / 10 ms | Good; response cache is working |
| Champion role rates | 38 / 43 ms | 23 / 26 ms | Good |
| Champion guide index | 62 / 70 ms | 41 / 45 ms | Good |
| Aatrox champion page | 13 / 14 ms | 12 / 13 ms | Good |
| Kled matchup page | 12 / 14 ms | 12 / 14 ms | Good |
| Lee Sin matchup page | 12 / 13 ms | 10 / 11 ms | Good |

Interpretation:

- The common champion-page and exact-matchup bundle paths are no longer visibly slow in the sampled production path.
- The summoner leaderboard now behaves like an in-memory cached endpoint after the first request, dropping from the previous occasional hundreds-of-milliseconds behavior to single-digit milliseconds.
- Patch list remains the slowest sampled no-warmup path, but it is still comfortably under the 500 ms target.
- Keep strict perf gates off for now. These numbers are strong enough to be a baseline, but a few more days of collection and refresh cycles should pass before turning warnings into deploy blockers.
