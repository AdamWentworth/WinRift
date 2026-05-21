# Remaining Work

This is the next-session list from the live app rebuild work. The project is moving in the right direction, but a few areas now deserve cleanup before we keep stacking features on top.

## Frontend Structure

Status: items 1-5 have been addressed in the current cleanup pass. Item 6 is the next product/data pass.

1. Split `LiveMatchups.tsx` into smaller live-match modules. It currently owns the page shell, mode rail, champion cards, build view, win-condition view, charting, and matchup helpers.
2. Extract shared UI pieces for stat tiles, sample chips, champion rows, role selectors, and card shells so the live page, champion guide, and tier list feel consistently built instead of copied forward.
3. Document `GlobalBackgroundStage` as the global background system, including the champion-filtered mode for champion pages.
4. Tune the global background contrast by page type so the art is atmospheric without making dense data screens harder to read.

## Product Surfaces

5. Improve the tier-list ranking formula. Current data is useful, but tiering should eventually include sample size, pick rate, winrate confidence, ban signal, and role population. Current pass complete: ranking now uses strict role buckets, sample-shrunk winrate, Wilson confidence, smaller ban weight, role-relative impact, and top-lane durability pressure. Follow-up notes are captured in `docs/product/tier-list-ranking.md`.
6. Expand champion guide backend data for skill order, matchup counters, rune paths, summoner spell pairs, and item-path summaries. First pass complete: skill orders, matchups, rune/spell signatures, ban rates, item slots, and full item-path summaries are now available.
7. Add a future summoner profile page that can show stored match history, live-game redirect state, aliases, champion comfort, and ranked form.
8. Refine live-match modes so Match, Builds, and Win Conditions each have a clear job and do not silently load hidden heavy analytics.

## Analytics

9. Keep moving high-traffic frontend reads onto summary/read-model tables instead of computing repeated aggregates on page load.
10. Continue validating win-condition confidence, rating thresholds, timing buckets, and synergy effects against the stored match corpus.
11. Revisit build-matchup filtering as the dataset grows: champion-vs-champion first, then optional role, rank, region, and patch filters once sample size can support them.

## Ops

12. Document production deployment flow for the home server: code deploys from dev to prod, but match collection should run on prod only once stable.
13. Add clearer worker lifecycle commands or docs so `down`, worker stop, key expiry handling, and restart behavior are obvious.
14. Add storage policy docs for ClickHouse raw retention, summary-table retention, backups, and NAS/off-box archive options.
