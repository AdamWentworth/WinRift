# Architecture

WinRift now separates collection, analytics storage, public API, and UI.

## Services

- `web`: Vite React app in `apps/web` for build exploration and live-match context.
- `api`: Go HTTP process from `services/core`, exposing account, live-game, static-data, collector, and analytics endpoints.
- `worker`: optional collector loop that resolves configured seeds and ingests recent ranked matches.
- `clickhouse`: append-first analytics store.

`api`, `worker`, and `patchctl` are separate runtime entrypoints built from the same `services/core` module. They share Riot routing, normalization, ClickHouse access, and patch lifecycle logic, so this is intentionally a service-oriented monorepo rather than a microservice split.

## Data Flow

1. A Riot ID resolves through Account-V1 to PUUID.
2. The collector fetches recent Match-V5 IDs, match payloads, and timeline payloads.
3. The normalizer filters ranked Solo/Duo Summoner's Rift games and writes raw plus normalized facts.
4. Timeline frames/events are extracted into curated tables for item timings, combat, objectives, and power-curve analysis.
5. `participant_matchups` creates one participant-opponent row per player, using same-position opponent when available.
6. `build_analytics_mv` groups matchup rows by champion, role, opponent, patch, rank, item signatures, rune signature, and spell signature.
7. The web app requests ranked aggregate patterns and displays counts, winrate, and confidence.

## Legacy

The old Mongo/Express/CRA/Python scripts are preserved in `legacy/`. They are no longer part of the runtime path.
