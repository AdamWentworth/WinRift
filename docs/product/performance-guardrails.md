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

Champion guide refreshes should stage new rows before deleting older compiled rows. The current lane inserts a fresh snapshot into the ReplacingMergeTree read models, then removes rows with an older `compiled_at`. Public reads keep seeing the previous snapshot during the rebuild, and a failed refresh leaves the old snapshot intact instead of briefly exposing empty tables.

After the champion-guide lane refreshes, the worker can also prewarm champion-page bundles into `champion_page_bundle_cache`. This is controlled by `CHAMPION_PAGE_PREWARM_*` env vars and is ClickHouse/local cache work only; it does not spend Riot API budget. The prewarm limit should be broad enough to cover normal champion browsing, not only the five highest-played champions per role. Prewarming covers the retained patch window so the UI's default previous-patch view can stay fast while the current patch is still thin.

Exact opponent-filtered champion pages are the expensive cold path because they assemble the same guide bundle plus matchup-specific item panels. The worker therefore prewarms a bounded number of common opponent bundles per selected champion/role. Keep `CHAMPION_PAGE_PREWARM_MAX_MATCHUP_BUNDLES` conservative; the default is `100` per patch. Warm responses should be effectively instant, but the first build of an uncached exact matchup can still take seconds.

## Profiling Checklist

Before treating a page as polished, test the deployed API with `curl` timing output:

```bash
curl -o /tmp/winrift.out -s -w \
  'status=%{http_code} ttfb=%{time_starttransfer} total=%{time_total} size=%{size_download}\n' \
  'http://192.168.1.77:8000/api/summoners/leaderboard?platform=NA1&limit=50'
```

Or run the project smoke script:

```bash
WINRIFT_PERF_BASE_URL=http://192.168.1.77:8000 \
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
WINRIFT_ROUTE_PERF_API_URL=http://192.168.1.77:8000 \
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
- Server API at `http://192.168.1.77:8000`.
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
