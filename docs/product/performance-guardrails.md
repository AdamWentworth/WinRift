# Performance Guardrails

WinRift should feel instant for stored analytics pages. Riot-dependent live lookups can take longer, but pages backed by collected data should not repeatedly scan raw match JSON or rebuild large aggregates during a request.

## Request-Path Rules

- Public page endpoints should read from compact read models, summary tables, or explicit response caches.
- Request handlers should not parse `raw_matches.raw_json` or `raw_timelines.raw_json`.
- Request handlers should not rebuild whole-patch aggregates.
- Expensive ClickHouse work belongs in the worker refresh lanes, patch archive commands, or explicit dev/admin refresh jobs.
- Frontend pages should prefer bundled endpoints over request waterfalls when several panels always load together.

## Current Read Models

- Champion pages: `champion_page_bundle_cache`, champion guide summary/scope analytics, matchup analytics, rune/spell signature analytics, item slot analytics, starting loadout analytics, skill analytics, ban analytics, and build variant analytics.
- Tier lists and champion guide indexes: `champion_guide_summary_analytics` plus `champion_guide_scope_analytics` for role/rank/patch-scoped counts.
- Summoner profiles: `summoner_profile_summary`, `summoner_champion_summary`, `summoner_champion_role_summary`, `summoner_recent_match_summary`, and `summoner_build_summary`.
- Summoner identity and ladder rows: `summoner_identity_summary`, rank snapshots, and profile summaries.
- Win conditions: `match_team_win_conditions` and `patch_win_condition_metrics`.

The champion guide summary read model intentionally treats kill participation as a neutral score for now. ClickHouse rejects the nested team-kill aggregate in the same refresh query, and the endpoint still has KDA, gold, CS, damage, tanking, vision, utility, objectives, survivability, pick/ban, winrate, and confidence. Add a dedicated team-kills read model before reintroducing kill participation to this fast path.

## Profiling Checklist

Before treating a page as polished, test the deployed API with `curl` timing output:

```bash
curl -o /tmp/winrift.out -s -w \
  'status=%{http_code} ttfb=%{time_starttransfer} total=%{time_total} size=%{size_download}\n' \
  'http://192.168.1.77:8000/api/summoners/leaderboard?platform=NA1&limit=50'
```

Or run the project smoke script:

```bash
WINRIFT_PERF_BASE_URL=http://192.168.1.77:8000 ops/perf-smoke.sh
```

There is also a manual GitHub Actions workflow, `perf-smoke`, that runs the same script from the private self-hosted production runner. It defaults to warning on threshold misses rather than failing, because this is currently a regression detector and tuning aid. Set `strict_thresholds=true` when the measured endpoints are consistently under their targets.

Targets for private-LAN reads:

- Static metadata: under 100 ms after warmup.
- Summary-backed lists: under 300 ms after warmup.
- Bundled champion/profile pages: under 500 ms after warmup.
- Live Riot lookups: allowed to be slower, but must respect Riot auth/rate-limit guardrails.

## Regression Smell

If an analytics endpoint suddenly takes seconds, first check for:

- a raw JSON table in the hot query,
- a missing summary refresh,
- a browser request waterfall,
- a cache key that is too specific or not being reused,
- a `FINAL` query over a large table where a compact summary would work.

The fix should usually be a new summary/read-model refresh, not more frontend loading spinners.
