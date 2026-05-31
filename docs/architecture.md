# Architecture

WinRift now separates collection, analytics storage, public API, and UI.

## Services

- `web`: Vite React app in `apps/web` for build exploration and live-match context.
- `api`: Go HTTP process from `core`, exposing account, live-game, static-data, collector, and analytics endpoints.
- `worker`: optional collector loop that pulls due PUUIDs from `collector_frontier` and ingests recent ranked matches within a request budget.
- `clickhouse`: append-first analytics store.

`api`, `worker`, and `patchctl` are separate runtime entrypoints built from the same `core` module. They share Riot routing, normalization, ClickHouse access, and patch lifecycle logic, so this is intentionally a service-oriented monorepo rather than a microservice split.

## Data Flow

1. A Riot ID resolves through Account-V1 to PUUID.
2. Seeds are stored in `collector_frontier`.
3. The collector fetches due PUUIDs, recent Match-V5 IDs, match payloads, and timeline payloads.
4. The normalizer filters ranked Solo/Duo Summoner's Rift games and writes raw plus normalized facts.
5. Stored matches enqueue discovered participant PUUIDs back into `collector_frontier`.
6. Timeline frames/events are extracted into curated tables for item timings, combat, objectives, and power-curve analysis.
7. `participant_matchups` creates one participant-opponent row per player, using same-position opponent when available.
8. `build_analytics_mv` groups matchup rows by champion, role, opponent, patch, rank, item signatures, rune signature, and spell signature.
9. The web app requests ranked aggregate patterns and displays counts, winrate, and confidence.

## Legacy

The old Mongo/Express/CRA/Python prototype has been retired from the working tree. The useful product and analytics ideas are captured in [Legacy Win Condition Audit](legacy-win-condition-audit.md), [Win Conditions](discussions/win-conditions.md), and the current Go win-condition/profile implementation.
