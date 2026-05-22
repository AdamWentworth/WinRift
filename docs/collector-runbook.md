# Collector Runbook

For server deployment, storage, backups, and production lifecycle notes, also see [Deployment And Operations](ops-deployment.md) and [Storage Policy](storage-policy.md).

## Start Infrastructure

```bash
make up
```

## Seed From API

Call the dev-only endpoint while `ENVIRONMENT=development`:

```bash
curl -X POST http://localhost:8000/api/dev/collector/seed \
  -H "Content-Type: application/json" \
  -d '{
    "riotIds": [{"gameName": "Example", "tagLine": "NA1", "platform": "NA1"}],
    "matchCount": 5,
    "maxRequests": 20
  }'
```

This resolves seeds, inserts them into `collector_frontier`, immediately collects a small batch, and inserts discovered match participants back into the frontier.

Watch API-triggered collection live in another terminal:

```bash
docker compose logs -f api
```

The logs show seed resolution, match id lookup, already-ingested skips, match/timeline fetches, normalization counts, inserts, and final request counters. Rank enrichment is handled by the worker's separate rank lane, not by API-triggered match insertion.

Use `frontierOnly` when you want to seed without collecting immediately:

```json
{"riotIds": [{"gameName": "Example", "tagLine": "NA1", "platform": "NA1"}], "frontierOnly": true}
```

## Seed Worker

Set one or both in `.env`:

```text
COLLECTOR_SEED_RIOT_IDS=Example#NA1
COLLECTOR_SEED_PUUIDS=
COLLECTOR_PLATFORMS=NA1
RIOT_MIN_REQUEST_INTERVAL_MS=75
RIOT_RATE_LIMIT_MAX_RETRIES=3
RIOT_RATE_LIMIT_MAX_SLEEP_SECONDS=120
RIOT_AUTH_FAILURE_EXIT=true
RIOT_AUTH_FAILURE_MARKER_PATH=/run/winrift/riot-auth-failed
COLLECTOR_INTERVAL_SECONDS=120
COLLECTOR_CURRENT_PATCH=16.10
COLLECTOR_PATCH_RETENTION_COUNT=2
COLLECTOR_PRUNE_OLD_PATCHES_ON_START=false
COLLECTOR_IDLE_SLEEP_SECONDS=15
COLLECTOR_FRONTIER_BATCH_SIZE=3
COLLECTOR_MAX_REQUESTS_PER_PASS=0
COLLECTOR_RATE_LIMIT_REQUESTS=100
COLLECTOR_RATE_LIMIT_WINDOW_SECONDS=120
COLLECTOR_RATE_LIMIT_RESERVE_REQUESTS=10
COLLECTOR_RECHECK_HOURS=24
COLLECTOR_DISCOVERY_DELAY_MINUTES=60
COLLECTOR_AUTO_SEED_CHALLENGER=false
COLLECTOR_AUTO_SEED_LIMIT_PER_PLATFORM=3
RANK_ENRICHMENT_ENABLED=false
RANK_SNAPSHOT_TTL_HOURS=24
RANK_ENRICHMENT_MAX_REQUESTS_PER_PASS=5
ACCOUNT_ALIAS_ENRICHMENT_ENABLED=true
ACCOUNT_ALIAS_MAX_REQUESTS_PER_PASS=3
ITEM_SLOT_ANALYTICS_REFRESH_ENABLED=true
ITEM_SLOT_ANALYTICS_REFRESH_INTERVAL_MINUTES=10
WIN_CONDITION_ANALYTICS_REFRESH_ENABLED=true
WIN_CONDITION_ANALYTICS_REFRESH_INTERVAL_MINUTES=15
```

Then run:

```bash
make up-worker
```

For a detached worker, follow progress with:

```bash
make logs-worker
```

To pause only the collector and leave ClickHouse/API running:

```bash
make stop-worker
```

That is just a wrapper for:

```bash
docker compose --profile worker stop worker
```

## Full Shutdown

Use this when putting the project down, especially before leaving the laptop unattended:

```bash
make down
```

This includes the `worker` Compose profile and removes orphan containers:

```bash
docker compose --profile worker down --remove-orphans
```

Do not use profile-less stop/down commands as the canonical shutdown path when the collector has been running. The worker is profile-gated, and leaving it out can leave a collector container alive while ClickHouse/API are stopped. Verify with:

```bash
make status
```

At startup, the worker resolves env seeds into `collector_frontier` and stores their Riot ID aliases. If `COLLECTOR_AUTO_SEED_CHALLENGER=true`, it also seeds each configured platform from that platform's Challenger Solo/Duo ladder. Each sweep walks `COLLECTOR_PLATFORMS`, pulls due frontier rows per platform, collects recent ranked matches, stores normalized rows, queues discovered participants, runs a separate rank lane for participants that lack a fresh rank snapshot, runs an account-alias lane for stored participant PUUIDs that do not yet have a saved `gameName#tagLine`, periodically refreshes current patch item-slot/champion-guide/win-condition read models, refreshes summoner profile summaries, and records Riot requests in a rolling regional budget ledger. Live lookups can also nudge low-sample live participants into `collector_frontier` with `source='live-backfill'`, letting champion-specific card stats improve in the background without blocking the lookup. It only sleeps when no useful work was done or when all regional budgets are temporarily full.

Patch retention is intentionally tied to `COLLECTOR_CURRENT_PATCH`. For example, `COLLECTOR_CURRENT_PATCH=16.11` with `COLLECTOR_PATCH_RETENTION_COUNT=2` stores `16.11` and `16.10`; when it is bumped to `16.12`, the active window becomes `16.12` and `16.11`, so `16.10` is no longer eligible and can be pruned on startup if `COLLECTOR_PRUNE_OLD_PATCHES_ON_START=true`.

For broad multi-platform collection, use smaller per-platform budgets. For example:

```text
COLLECTOR_PLATFORMS=NA1,EUW1,EUN1,KR,BR1,LA1,LA2,JP1,OC1,TR1,RU,SG2,TW2,VN2
COLLECTOR_CURRENT_PATCH=16.10
COLLECTOR_PATCH_RETENTION_COUNT=2
COLLECTOR_PRUNE_OLD_PATCHES_ON_START=true
COLLECTOR_FRONTIER_BATCH_SIZE=1
COLLECTOR_MAX_REQUESTS_PER_PASS=0
RANK_ENRICHMENT_MAX_REQUESTS_PER_PASS=5
ACCOUNT_ALIAS_ENRICHMENT_ENABLED=true
ACCOUNT_ALIAS_MAX_REQUESTS_PER_PASS=3
COLLECTOR_AUTO_SEED_CHALLENGER=true
```

`PH2` and `TH2` are known Riot platform values, but their platform API hostnames did not resolve from the local Docker network during testing. Leave them out unless those hosts resolve in your environment.

## Safety

Keep match counts low on a development key. The client honors Riot 429 `Retry-After` with bounded retries, but large crawl seeds can still consume a daily development-key window quickly.

If Riot returns 401 or 403, the process that saw it writes `RIOT_AUTH_FAILURE_MARKER_PATH`. The worker exits immediately. The API stays online, but Riot-dependent endpoints return `503 RIOT_API_KEY_UNAVAILABLE` and the Riot client short-circuits additional Riot calls while the marker exists. Cached analytics, static metadata, and health checks remain available. This prevents an expired or unauthorized development key from being retried every collector interval without making the whole app look dead. The frontier row is marked `blocked` when the failure happens during a collection pass.

Riot 404s are different: they mean the requested resource is absent, such as an unknown Riot ID or a player not currently being in a live game. They do not write the auth-failure marker.

If Riot returns 429, the client sleeps for `Retry-After` and retries up to `RIOT_RATE_LIMIT_MAX_RETRIES`. If Riot asks for a longer wait than `RIOT_RATE_LIMIT_MAX_SLEEP_SECONDS`, the collector defers that region for the rest of the pass and resumes on the next cycle.

Safety knobs:

- `COLLECTOR_FRONTIER_BATCH_SIZE`: max PUUIDs checked per worker pass.
- `COLLECTOR_PLATFORMS`: comma-separated platform routing values to collect, such as `NA1,EUW1,KR`.
- `RIOT_MIN_REQUEST_INTERVAL_MS`: process-local spacing between Riot requests. `75`ms stays under the `20 requests / 1 second` bucket.
- `RIOT_RATE_LIMIT_MAX_RETRIES`: max immediate sleeps/retries for Riot 429 responses.
- `RIOT_RATE_LIMIT_MAX_SLEEP_SECONDS`: longest 429 `Retry-After` sleep before deferring the region to the next collector cycle.
- `RIOT_AUTH_FAILURE_EXIT`: when true, the worker stops when a Riot auth failure marker appears. The API does not exit; it reports Riot-backed endpoints as unavailable.
- `RIOT_AUTH_FAILURE_MARKER_PATH`: shared marker file path used by API and worker to coordinate auth-failure behavior.
- `COLLECTOR_INTERVAL_SECONDS`: two-minute budget window used for budget-exhausted frontier retry timing.
- `COLLECTOR_CURRENT_PATCH`: current patch bucket, such as `16.10`. When set with the default two-patch retention window, the collector stores only the current patch and previous patch.
- `COLLECTOR_PATCH_RETENTION_COUNT`: number of same-season patch buckets to keep eligible for ingestion. With `COLLECTOR_CURRENT_PATCH=16.10` and `COLLECTOR_PATCH_RETENTION_COUNT=2`, the collector accepts `16.10` and `16.9`, then stops the current PUUID as soon as it sees `16.8` or older. When Riot moves to `16.11`, bump `COLLECTOR_CURRENT_PATCH` to `16.11` so the active window becomes `16.11` and `16.10`.
- `COLLECTOR_PRUNE_OLD_PATCHES_ON_START`: when true, worker startup deletes ClickHouse rows from raw, normalized, timeline, live aggregate, and compiled metric tables for patches outside the active retention window. Keep this false anywhere you want to preserve old patch history.
- `COLLECTOR_IDLE_SLEEP_SECONDS`: short pause when a sweep does no Riot work and no regional rate-limit wait is required.
- `COLLECTOR_RATE_LIMIT_REQUESTS`: Riot application request bucket size for one region.
- `COLLECTOR_RATE_LIMIT_WINDOW_SECONDS`: Riot application request bucket window.
- `COLLECTOR_RATE_LIMIT_RESERVE_REQUESTS`: requests held back per region as safety headroom.
- `COLLECTOR_MAX_REQUESTS_PER_PASS`: optional manual cap on match-history/match/timeline requests per platform. `0` means auto-calculate from the regional budget.
- `COLLECTOR_AUTO_SEED_CHALLENGER`: seed each platform from its Challenger Solo/Duo ladder on worker startup.
- `COLLECTOR_AUTO_SEED_LIMIT_PER_PLATFORM`: max Challenger ladder entries to seed per platform at startup.
- `COLLECTOR_DISCOVERY_DELAY_MINUTES`: delay before newly discovered participants are eligible.
- `COLLECTOR_RECHECK_HOURS`: delay before revisiting a checked PUUID.
- `RANK_ENRICHMENT_ENABLED`: when true, the worker runs a separate rank lane after each platform's match lane.
- `RANK_ENRICHMENT_MAX_REQUESTS_PER_PASS`: max rank lane requests per platform. These are subtracted from the same regional Riot request budget as match collection, but they are not spent inside individual match ingestion.
- `RANK_SNAPSHOT_TTL_HOURS`: freshness window before a rank snapshot can be refreshed.
- `ACCOUNT_ALIAS_ENRICHMENT_ENABLED`: when true, the worker runs a separate account-alias lane after the match/rank lanes. It resolves stored participant PUUIDs into Riot ID aliases and saves them in `riot_account_aliases` for tagless frontend lookup.
- `ACCOUNT_ALIAS_MAX_REQUESTS_PER_PASS`: max account-alias requests per platform. These are subtracted from the same regional Riot request budget as match collection.
- `ITEM_SLOT_ANALYTICS_REFRESH_ENABLED`: when true, the worker refreshes the current patch `item_slot_analytics` summary at startup and then after collector sweeps when the interval has elapsed.
- `ITEM_SLOT_ANALYTICS_REFRESH_INTERVAL_MINUTES`: minutes between scheduled current-patch item-slot summary refreshes. This is ClickHouse work plus one cached Data Dragon item metadata lookup, not Riot match/league API budget.
- `CHAMPION_GUIDE_ANALYTICS_REFRESH_ENABLED`: when true, the worker refreshes champion-guide read models, including champion skill paths, ban rates, and alternative build variants.
- `CHAMPION_GUIDE_ANALYTICS_REFRESH_INTERVAL_MINUTES`: minutes between scheduled champion-guide read-model refreshes. This is ClickHouse/local aggregation work, not Riot API budget.
- `WIN_CONDITION_ANALYTICS_REFRESH_ENABLED`: when true, the worker refreshes the current patch `match_team_win_conditions` and `patch_win_condition_metrics` summaries at startup and then after collector sweeps when the interval has elapsed.
- `WIN_CONDITION_ANALYTICS_REFRESH_INTERVAL_MINUTES`: minutes between scheduled current-patch win-condition summary refreshes. This is ClickHouse/local profile work, not Riot API budget. The default is longer than item slots because it rebuilds per-platform strategy metrics and then the combined `ALL` aggregate.
- `SUMMONER_PROFILE_ANALYTICS_REFRESH_ENABLED`: when true, the worker refreshes compact summoner profile and champion-comfort summaries from retained participant rows.
- `SUMMONER_PROFILE_ANALYTICS_REFRESH_INTERVAL_MINUTES`: minutes between scheduled summoner profile read-model refreshes. This is ClickHouse/local aggregation work, not Riot API budget.

Rank enrichment is off by default. Turn it on only after basic match ingestion is working. When enabled, the rank lane chooses distinct participant PUUIDs from ClickHouse that do not already have a fresh `summoner_rank_snapshots` row, prioritizing players attached to the most `UNKNOWN` participant rows. Account-alias enrichment is on by default with a small cap because it improves the lookup UX and does not need to be refreshed often.

## Request Formula

For each region, the worker computes a usable cycle budget:

```text
usable_region_requests = min(rate_limit_requests, rate_limit_requests * interval / rate_limit_window) - reserve_requests
```

With the local defaults, that is `100 * 120 / 120 - 10 = 90` Riot requests per region per rolling two-minute window. The worker splits currently available regional budget across active platforms in that region, reserves up to `RANK_ENRICHMENT_MAX_REQUESTS_PER_PASS` for the rank lane, then up to `ACCOUNT_ALIAS_MAX_REQUESTS_PER_PASS` for the alias lane, and gives the remainder to match collection. If a region spends all 90 requests in 30 seconds, it waits about 90 seconds for that region; if it spends them in 60 seconds, it waits about 60 seconds.

Match collection costs `1 + (2 * matches)` requests per frontier row: one match-id lookup, then one match payload and one timeline payload per new ranked match. Rank enrichment costs one request per rank candidate and account-alias enrichment costs one request per PUUID alias candidate. Both are capped separately and subtracted from the same regional request budget, so they can steadily improve metadata coverage without turning every collected match into a burst of extra Riot calls.

Live champion stat backfill is controlled by:

- `LIVE_BACKFILL_ENABLED`: enables queueing low-sample live participants into the collector frontier.
- `LIVE_BACKFILL_MIN_CHAMPION_GAMES`: minimum collected games on the current champion before no backfill is queued.
- `LIVE_BACKFILL_MAX_SEEDS`: max frontier nudges per live lookup.
- `LIVE_BACKFILL_PRIORITY`: frontier priority for live backfill nudges.
- `LIVE_BACKFILL_DELAY_SECONDS`: optional delay before the backfill row is due.
