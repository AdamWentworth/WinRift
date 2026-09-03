# Performance Guardrails

Stored analytics must feel immediate. Riot-dependent live lookups may take longer, but champion, profile, tier-list, and archived-patch pages must not scan raw payloads or rebuild patch aggregates during a request.

## Hot-Path Rules

- Page endpoints read compact ClickHouse summaries or explicit response caches.
- Request handlers do not parse `raw_matches.raw_json` or `raw_timelines.raw_json`.
- Expensive aggregation belongs in worker refresh lanes or explicit owner maintenance commands.
- Multi-panel pages use bundled endpoints instead of frontend request waterfalls.
- A failed refresh leaves the previous compiled snapshot readable.

## Champion-Page Cache

`champion_page_bundle_cache` stores the complete response used by `/champions/<champion>`.

At worker startup WinRift:

1. resolves the current patch from analytics metadata,
2. prewarms every canonical champion for the current patch,
3. verifies mature-patch fallback data where the current guide is empty,
4. prewarms every canonical champion for every selectable archived patch,
5. records both resolved-role and automatic-role aliases.

Retained-patch bundles expire after two hours so scheduled analytics refreshes become visible. Archived-patch bundles remain reusable for 30 days because their source data is immutable. Exact opponent bundles are prewarmed only for a bounded set of common retained-patch matchups; simultaneous misses for the same uncached page are coalesced into one build.

This is ClickHouse and local-cache work. Prewarming does not consume Riot API budget.

## API Smoke

`ops/perf-smoke.sh` checks health, patch metadata, leaderboard, champion-role, guide-index, champion-page, and representative exact-matchup endpoints.

```bash
WINRIFT_PERF_BASE_URL=http://SERVER_LAN_IP:8000 \
WINRIFT_PERF_STRICT=1 \
ops/perf-smoke.sh
```

Unless `WINRIFT_PERF_PATCH` is explicitly provided, the script resolves `currentPatch` from `/api/analytics/patches?queueId=420`. It does not carry a hard-coded historical default.

Useful overrides:

- `WINRIFT_PERF_RUNS`: measured requests per endpoint; default `3`.
- `WINRIFT_PERF_WARMUPS`: warmups before measurement; default `1`.
- `WINRIFT_PERF_TIMEOUT_SECONDS`: per-request timeout; default `15`.
- `WINRIFT_PERF_JSONL`: optional machine-readable output path.
- `WINRIFT_PERF_STRICT`: fail on threshold warnings when `1` or `true`.

Private-LAN targets:

| Path class | Maximum |
|---|---:|
| Health and static metadata | 250 ms |
| Patch metadata and champion roles | 500 ms |
| Summoner leaderboard | 750 ms |
| Champion guide index | 1,000 ms |
| Bundled champion page | 500 ms |
| Representative exact matchup | 750 ms |

## Full Champion/Patch Audit

`ops/prod/champion-page-perf-audit.py` tests the actual canonical request used by the frontend for every champion across every selectable patch.

```bash
python3 ops/prod/champion-page-perf-audit.py \
  --base-url http://SERVER_LAN_IP:8000 \
  --max-ms 500 \
  --concurrency 4 \
  --json champion-page-performance.json
```

Each response must:

- return HTTP 200,
- contain a complete champion-page bundle,
- resolve a role,
- report `X-WinRift-Cache: hit`,
- finish within 500 ms,
- contain mature fallback guide data when the current-patch guide is empty.

A missing cache entry, incomplete response, empty required fallback, or latency breach fails the deployment gate.

## Browser Route Gate

API speed alone is insufficient. `npm run perf:routes` builds the production frontend, serves it through the route-performance proxy, opens core routes in Chromium, waits for page-specific ready markers, and counts API requests.

```bash
cd apps/web
WINRIFT_ROUTE_PERF_API_URL=http://SERVER_LAN_IP:8000 \
WINRIFT_ROUTE_PERF_STRICT=1 \
npm run perf:routes
```

| Route | Ready budget | API request ceiling |
|---|---:|---:|
| `/` | 2,500 ms | 3 |
| `/champions` | 3,000 ms | 3 |
| `/champions/Aatrox` | 4,500 ms | 7 |
| `/tier-list` | 3,500 ms | 3 |
| `/summoners` | 3,500 ms | 3 |

Strict mode fails on route-readiness or request-count regressions. Results are written to `apps/web/test-results/route-performance.json` by default.

## Deployment Enforcement

- Private HomeOps core deployment waits for startup prewarm completion and runs the full champion/patch audit extracted from the approved core image.
- Private HomeOps web deployment verifies health, same-origin API proxying, deep routes, cache behavior, and the strict Playwright route gate extracted from the approved web image.
- Normal CI runs Go tests/vet, frontend tests/build, CodeQL, container vulnerability scans, and SBOM generation.

## Current Production Baseline

The August 27, 2026 audit covered every selectable champion/patch combination:

| Requests | Cache hits | Average | p95 | Maximum |
|---:|---:|---:|---:|---:|
| 1,557 | 1,557 | 11.6 ms | 20.3 ms | 40.1 ms |

If a stored analytics route returns to multi-second loads, inspect cache-key coverage, refresh health, raw-table access, `FINAL` use on large tables, and frontend request count before adding loading UI.
