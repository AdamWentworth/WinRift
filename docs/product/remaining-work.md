# Remaining Work

This is the next-session list from the live app rebuild work. The project is moving in the right direction, but a few areas now deserve cleanup before we keep stacking features on top.

## Frontend Structure

Status: items 1-10 and the first ops/documentation pass have been addressed. The next product pass should be validation, performance polish, or targeted read-model cleanup, not basic page scaffolding.

1. Split `LiveMatchups.tsx` into smaller live-match modules. Current pass complete: `LiveMatchups.tsx` now stays focused on live-state orchestration, with player cards, mode context copy, card-grid rendering, and lane-order helpers moved into `components/live-match/`.
2. Extract shared UI pieces for stat tiles, sample chips, champion rows, role selectors, and card shells so the live page, champion guide, and tier list feel consistently built instead of copied forward. Current pass complete: profile tabs/sort buttons now use a shared segmented control, profile role filters use shared role tabs, profile metrics use the shared metric tile/mini-stat primitives, and profile/guide empty states share one component shape.
3. Document `GlobalBackgroundStage` as the global background system, including the champion-filtered mode for champion pages. Current pass complete: `docs/product/global-background-system.md` now covers image sources, page scoping, contrast profiles, motion rules, CDN/deployment policy, and future tuning.
4. Tune the global background contrast by page type so the art is atmospheric without making dense data screens harder to read.

## Product Surfaces

5. Improve the tier-list ranking formula. Current data is useful, but tiering should eventually include sample size, pick rate, winrate confidence, ban signal, and role population. Current pass complete: ranking now uses strict role buckets, sample-shrunk winrate, Wilson confidence, smaller ban weight, role-relative impact, and top-lane durability pressure. Follow-up notes are captured in `docs/product/tier-list-ranking.md`.
6. Expand champion guide backend data for skill order, matchup counters, rune paths, summoner spell pairs, and item-path summaries. First pass complete: skill orders, matchups, rune/spell signatures, ban rates, item slots, and full item-path summaries are now available.
7. Build the first summoner profile page. Current pass complete: Riot ID searches resolve saved aliases, live players jump into the match room, and non-live profiles show cached rank, stored match form, champion comfort, summoner-owned builds, and recent stored matches.
8. Refine the summoner profile page now that the base surface exists. Current pass complete: stable tab width, clearer stored-data freshness, champion-name filtering, role-aware champion comfort filters, cleaner empty states, improved recent-match rows, and profile-specific background art based on recently played champions.
9. Refine live-match modes so Match, Builds, and Win Conditions each have a clear job and do not silently load hidden heavy analytics. Current pass complete: the mode rail has explicit context copy, Builds and Win Conditions keep their heavy queries gated to their active modes, image fallbacks no longer emit empty `src` warnings, and Builds mode now focuses on one selected player/opponent pair with compact selectors.

## Analytics

10. Keep moving high-traffic frontend reads onto summary/read-model tables instead of computing repeated aggregates on page load. Current pass complete: summoner profile summary tables now provide one compact row per summoner, compact champion-comfort rows, and role-aware champion-comfort rows, refreshed by the worker.
11. Continue validating win-condition confidence, rating thresholds, timing buckets, and synergy effects against the stored match corpus.
12. Revisit build-matchup filtering as the dataset grows: champion-vs-champion first, then optional role, rank, region, and patch filters once sample size can support them. Current pass complete for MVP semantics: matchup cards no longer borrow champion-wide rows, champion-wide baseline lives in its own card, and displayed slot reads suppress impossible duplicate-boot builds.

## New Follow-Ups

13. Validate build-card ranking against a larger set of champion/opponent examples. We fixed one integrity class, but still need spot checks for jungle starters, support items, transformed/Ornn-style item IDs, item reworks, and unusual inventory events.
14. Decide how to present missing exact-matchup slots. Current behavior leaves slots blank once exact matchup data runs out. That is honest, but the UI may eventually need a softer "baseline available below" treatment.
15. Add diagnostics for build-advice scope composition. A debug-only panel or endpoint should show how many displayed slots came from current-patch exact matchup versus all-patch exact matchup, without cluttering the player-facing card.
16. Revisit actual full build paths once samples are denser. Slot reads are useful, but a completed path is easier for players to digest when the sample is strong enough.
17. Add a small frontend regression test for duplicate boots in displayed build slots.

## Ops

18. Document production deployment flow for the home server: code deploys from dev to prod, but match collection should run on prod only once stable. Current pass complete: see `docs/ops-deployment.md`.
19. Add clearer worker lifecycle commands or docs so `down`, worker stop, key expiry handling, and restart behavior are obvious. Current pass complete: see `docs/ops-deployment.md` and `docs/collector-runbook.md`.
20. Add storage policy docs for ClickHouse raw retention, summary-table retention, backups, and NAS/off-box archive options. Current pass complete: see `docs/storage-policy.md`.
