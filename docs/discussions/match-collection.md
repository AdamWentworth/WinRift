# Match Collection Discussion

The collection system is fundamentally sound, but naturally constrained by Riot API limits.

## What Feels Solid

- Go is a good fit for the collector: small binary, low idle memory, simple concurrency, and predictable deployment on a lightweight server.
- ClickHouse is a good storage fit. The timeline and raw JSON data compress well, and append-first tables match the workload.
- The worker/frontier model is the right base shape. It can start from seed PUUIDs, collect recent ranked matches, discover participants, and keep expanding.
- Riot rate limits are respected through regional request budgets, request spacing, 429 handling, and bounded retries.
- Expired API key behavior is safe. A 401/403 writes the auth-failure marker and stops worker activity instead of retrying forever, while the API remains up for cached/static/analytics responses.
- Patch retention is explicit: current plus previous patch for raw/normalized collection, with closed-patch metrics retained separately.
- Rank and account alias enrichment are separate lanes, which keeps metadata useful without turning every match insert into a burst of extra API calls.

## What Is Hampered By External Limits

The dev key budget is the main bottleneck. One new match usually costs:

```text
1 match-history request
+1 match payload
+1 timeline payload
= 3-ish requests per collected match path
```

Discovered matches can be deduped, but fresh collection is still API-expensive.

Rank snapshots and Riot ID aliases also consume budget, so they should stay capped and cache-heavy.

## Known Biases

- Frontier expansion starts from the seeds we provide, so early data can skew toward high elo, streamer networks, or specific platform clusters.
- Challenger auto-seeding improves match quality but reinforces high-elo skew.
- Rank enrichment is incomplete and current-ish, not a perfect historical rank-at-match record.
- Different regions may have different metas, but pooling regions is useful until samples are dense.
- Current collection focuses ranked Solo/Duo Summoner's Rift, queue `420`, which is correct for MVP but intentionally excludes normals, flex, and ARAM.

## What We Can Test

- Collection throughput by region per hour.
- Deduplication rate after frontier expansion matures.
- API request cost per inserted match.
- Rank coverage over time.
- Alias coverage over time.
- Patch distribution and whether older-patch stopping is working.
- Storage growth per 10,000 matches.

## Refinement Ideas

- Gap-aware frontier scheduling: prioritize champion/opponent/rank/region buckets that are under-sampled.
- Platform balancing: rotate regions based on stored-match targets rather than only due frontier rows.
- Better rank targeting: selectively enrich ranks for players whose matches contribute to UI-visible samples.
- Match quality tiers: store all eligible matches, but label high-confidence analytics by rank/sample quality.
- Production-key math: document new request budgets when Riot grants a production key.

## Current Position

Keep the worker as-is structurally. Improve targeting later. The collector does not appear architecturally wrong; it is mostly limited by API budget and by the need to decide which samples matter most.
