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

### Route-Level Static Metadata

The root app still loads champion metadata and splash metadata eagerly because those power search, routing, and the global background. It does not eagerly load item, rune, and summoner-spell metadata on surfaces that do not need them.

Eager:

- champion metadata,
- champion splash metadata,
- patch list,
- champion role rates after champion ids are known.

Lazy/gated:

- item metadata,
- rune metadata,
- summoner spell metadata.

Those load only on champion and summoner/live surfaces where the UI actually renders builds, spells, or runes.

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
WINRIFT_PERF_BASE_URL=http://192.168.1.77:8000 \
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
WINRIFT_ROUTE_PERF_API_URL=http://192.168.1.77:8000 \
npm run perf:routes
```

This starts a production preview build, opens the core routes with Playwright, records route ready time and API request count, and writes `test-results/route-performance.json`.

## Next Improvements

- Consider prefetching champion-page bundles on visible links only, not every hover, if the app starts over-prefetching.
- Add route-level bundle splitting if the frontend bundle becomes large enough to affect first paint.
- Promote perf smoke thresholds from warning to strict only after we collect stable baseline runs from the server.
