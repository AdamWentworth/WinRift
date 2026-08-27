# Current Next Steps

This is the active WinRift backlog after the `v0.1.0` private-production milestone. Completed implementation journals remain available in [Remaining Work](remaining-work.md) and [Tech Debt Roadmap](tech-debt-roadmap.md).

## Product Validation

1. Continue validating tier-list and win-condition scoring as each patch corpus matures.
2. Spot-check build ranking around support quest upgrades, transformed and Ornn-style items, item reworks, and unusual inventory events.
3. Decide whether missing exact-matchup item slots should stay intentionally blank or offer a clearly labeled champion-wide baseline.
4. Decide whether live build mode needs a compact patch-context label inside the mode panel.

## Performance and Data

1. Keep every selectable champion/patch page inside the strict cache-hit and latency deployment gates.
2. Investigate any regression at the cache key, prewarm coverage, or read-model layer before adding UI loading states.
3. Continue monitoring ClickHouse raw-timeline growth and archive closed patches only after summary coverage is verified.
4. Expand performance baselines when new high-traffic routes or archived patches become selectable.

## Public Exposure

1. Keep the current deployment private-LAN only until authentication, edge rate limiting, and public abuse controls are explicit.
2. Review Riot policy implications before exposing live-game or player lookup traffic publicly.
3. Keep production secrets, server addresses, backups, and raw match data out of the public repository.

## Repository Maintenance

1. Keep dependency updates small enough to review and require all repository checks before merge.
2. Refresh public screenshots after meaningful visual changes.
3. Publish milestone release notes when the product or deployment contract changes materially.
