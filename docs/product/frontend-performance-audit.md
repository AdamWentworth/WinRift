# Frontend Performance Audit

WinRift's frontend should avoid request waterfalls and should keep stored analytics pages feeling immediate. The backend now has bundled/read-model endpoints for the heavy pages; the frontend should preserve those wins by using stable query keys, useful stale times, and route-level metadata loading.

## Current Guardrails

### Shared Query Defaults

The app-level `QueryClient` uses `apps/web/src/lib/queryPolicies.ts`.

Defaults:

- no automatic refetch on window focus,
- retry transient/server failures up to two times,
- do not retry normal 4xx responses, except `429`,
- keep inactive queries around long enough for back/forward navigation.

The important bit is user feel: navigating back to a champion, tier list, or profile page should usually reuse warm data rather than flashing through another full loading cycle.

### Abort-Aware API Client

`apps/web/src/api/client.ts` accepts `AbortSignal` for all exported requests. React Query passes a signal into the query function, and the pages now forward that signal to `fetch`.

This matters when users:

- type quickly in search fields,
- switch champions rapidly,
- change patch/rank/role filters,
- leave a route before the old request finishes.

Stale requests should be canceled instead of racing the current page state.

### Route-Level Static Metadata And Bundles

The root app loads only the metadata needed for first interaction immediately. Heavy page code is split by route, and the larger splash catalog is delayed so it does not compete with first paint.

Eager:

- champion metadata,
- patch list,
- a small background fallback pool from champion metadata.

Lazy/gated:

- item metadata,
- rune metadata,
- summoner spell metadata.
- champion splash metadata after the app is already settled.

Those load only on champion and summoner/live surfaces where the UI actually renders builds, spells, or runes.

Route-level chunks are now in place for:

- champion guide pages,
- champion directory,
- live-match page,
- summoner profile page,
- tier list.

The main production bundle dropped from about 389 kB to about 259 kB uncompressed, and from about 115 kB to about 81 kB gzip, in the first split pass. Keep new page-scale features in their route chunk unless they must run on the home/search shell.

### Bundled Analytics Pages

Champion guide pages use one bundled endpoint:

```text
GET /api/analytics/champion-page
```

That endpoint returns summary, guide data, matchup rows, rune/spell patterns, skill data, build advice, build variants, and guide index context. The frontend should not reintroduce separate always-on requests for each panel unless there is a specific interaction that deserves its own fetch.

## Smells To Watch For

- Adding a new `useQuery` inside a page section that loads on every route render.
- Passing raw objects in query keys when a stable primitive key would do.
- Setting very low `staleTime` on stored analytics.
- Fetching item/rune/spell metadata from pages that only need champion names or portraits.
- Making live-match mode fetch build or win-condition data before that mode is selected.
- Using frontend retries for 404/403 style responses where retrying cannot help.

## Manual Checks

Run the frontend tests:

```bash
cd apps/web
npm test -- --run
```

Run the production API smoke from the repo root:

```bash
WINRIFT_PERF_BASE_URL=http://SERVER_LAN_IP:8000 \
WINRIFT_PERF_PATCH=16.10 \
WINRIFT_PERF_RUNS=3 \
ops/perf-smoke.sh
```

Optional JSONL output:

```bash
WINRIFT_PERF_JSONL=/tmp/winrift-perf.jsonl ops/perf-smoke.sh
```

The smoke script samples each endpoint multiple times and warns on the slowest observed request. It is intentionally a guardrail, not a full benchmark suite.

Run browser route timing from `apps/web`:

```bash
WINRIFT_ROUTE_PERF_API_URL=http://SERVER_LAN_IP:8000 \
npm run perf:routes
```

This starts a production preview build, opens the core routes with Playwright, records route ready time and API request count, and writes `test-results/route-performance.json`.

## Next Improvements

- Consider prefetching champion-page bundles on visible links only, not every hover, if the app starts over-prefetching.
- Continue extracting page-local helpers out of large route files when the split makes ownership clearer.
- Promote perf smoke thresholds from warning to strict only after we collect stable baseline runs from the server.
