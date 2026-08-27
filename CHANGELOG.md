# Changelog

All notable WinRift milestones are recorded here. The project follows milestone-oriented semantic versioning while the live deployment remains private.

## [0.1.0] - 2026-08-27

### Product

- Shipped champion guides, champion directory, tier list, summoner profiles, live-match scouting, focused matchup builds, and composition win-condition analysis.
- Added role-aware, patch-aware champion pages with current and archived patch selection.
- Fixed client-side navigation so deep champion routes begin at the top of the page.

### Performance

- Added persistent champion-page bundle caching and startup prewarming for every canonical champion across current and selectable archived patches.
- Added bounded cache reads, retry behavior under ClickHouse pressure, and patch-metadata prewarming.
- Enforced API cache-hit, completeness, and latency budgets plus browser route-readiness gates during deployment.
- Verified 1,557 champion/patch requests at 100% cache hits with 11.6 ms average, 20.3 ms p95, and 40.1 ms maximum latency on the private production LAN.

### Operations and Security

- Deployed the React frontend, Go API, collector, monitor, and ClickHouse on the home server through separate core and web deployment workflows.
- Added immutable GHCR image tags, rollback metadata, container health checks, vulnerability scans, SBOM artifacts, and production smoke tests.
- Hardened route-performance proxying against absolute-URL request forgery and rejected out-of-range numeric identifiers at API boundaries.
- Added Riot authentication failure handling, worker health state, SMTP alerting, rate-limit budgeting, and scheduled read-model refreshes.

### Repository

- Added branded GitHub presentation, current documentation navigation, contribution guidance, issue/PR templates, CodeQL, and hardened dependency automation.
- Brought the maintained Go and web dependency sets current, including ClickHouse, React, Vite, Vitest, TypeScript, jsdom, and Testing Library updates.
- Established the first source-available milestone release.
