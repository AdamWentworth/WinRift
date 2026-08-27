# Build Guides

WinRift now has a non-live build guide surface alongside live match lookup.

The guide page can learn from public champion guide pages, but it should keep WinRift's own tone: darker match-room panels, visible sample coverage, matchup-first framing, and clear caveats when the data is thin.

The current version can show:

- champion, role, patch, rank, and optional opponent filters
- stored coverage for every champion in the selected role/patch/rank scope
- champion win rate, match sample, pick-rate estimate, confidence, and role rank within the stored sample
- tier-list scoring signals: WinRift score, win score, sample score, pick score, ban score, and role-relative impact score
- rune page aggregates from stored `rune_signature` values
- summoner spell aggregates from stored `spell_signature` values
- skill priority/path aggregates from stored `SKILL_LEVEL_UP` timeline events
- ban-rate estimates derived from champion bans in stored Match-V5 payloads
- toughest and favorable opponent matchups from stored participant matchup rows
- complete item-path summaries from `core3_signature` and `final_items_signature` as backend/debug data, not as a separate primary champion-page panel
- precomputed alternative build variants grouped by meaningful early core items, with player-facing labels such as AP, On Hit, Tank, Crit, Lethality, AD Bruiser, Enchanter, and Support Tank
- completed-item slot panels from the precomputed `item_slot_analytics` read model
- opening purchase bundles from the precomputed `starting_loadout_analytics` read model, so the starting-items panel can show legal combinations like Doran's Ring plus two potions instead of only the main starter item

Backend contracts:

- `GET /api/analytics/champion-page`: bundled champion guide payload for the frontend champion page. It combines the focused guide, build-advice data, role-rate hints, and guide index in one cached response so champion pages avoid a browser-side request waterfall.
- `GET /api/analytics/build-advice`: unified build payload for one champion, optional opponent, role, patch, and rank scope. It returns matchup item slots, champion-wide baseline item slots, opening purchase loadouts, top build signatures, runes, spells, item paths for diagnostics, sample quality, and fallback notes in one response.
- `GET /api/analytics/champion-guides`: index of champion summaries for the current role/patch/rank scope. This powers champion coverage and lets the UI expose every champion we have stored data for.
- `GET /api/analytics/champion-guide`: focused champion guide payload for one champion and role.
- `GET /api/analytics/item-slots`: slot-level item read model with matchup-aware fallback.
- `POST /api/dev/analytics/champion-guides/refresh`: local/dev refresh endpoint. With `backfill: true`, it derives participant performance, skill events, and champion bans from retained raw payloads before rebuilding guide read models.

Read models:

- `build_signature_analytics`: current/recent patch build signatures used by build advice, top builds, and item-path summaries before falling back to archived patch metrics.
- `champion_role_analytics`: champion role distribution used for default role resolution and role-rate hints before falling back to retained participant rows.
- `champion_build_variant_analytics`: precomputed champion-guide alternative build tabs, representative runes/spells/items, and build-specific skill orders where the sample floor is met.
- `champion_skill_analytics`: champion/role skill order rows.
- `champion_ban_analytics`: sample-relative ban-rate rows.
- `item_slot_analytics`: completed-item slot rows plus a single-starting-item fallback used by guide and live build panels.
- `starting_loadout_analytics`: legal fountain-opener bundles used by the starting-items panel before falling back to retained timeline scans.
- `champion_page_bundle_cache`: short-lived persisted JSON bundles for exact champion-page requests. The API still keeps an in-memory cache, but this table lets warmed pages survive API restarts and keeps repeated page loads close to instant.
- Worker prewarming: when `CHAMPION_PAGE_PREWARM_ENABLED=true`, the champion-guide refresh lane stores canonical champion and high-volume role page bundles for every selectable patch. It also warms a bounded set of common champion/opponent bundles for retained patches, because matchup-filtered item advice is the cold path users notice most. Archived bundles use a long-lived cache because their source data no longer changes.

Build variant labeling:

- Variants should come from WinRift's own match corpus, not from a runtime scrape of another stats site.
- External guide pages can be used as research inspiration for common player language, but the production path should remain our own classifier plus curated overrides where the data needs champion-specific wording.
- The first pass ignores boots, starter items, jungle pets, support quest items, consumables, and common components when deciding the variant identity.
- Variant labels are intentionally broad. If several core item families resolve to the same label, such as multiple Katarina AP paths, those rows should be summed into one player-facing build family.
- The `Recommended` tab is not a variant lane. It should use the broad champion/matchup build-advice data and the highest-support aggregate setup, while alternative tabs intentionally narrow to their detected family.
- Recommended item slots use a support-aware score, with samples ramping toward full trust around 200 games, so tiny hot samples do not outrank much larger, well-supported choices.
- The champion page should keep the lower item panels as the primary build presentation. The first panel is the legal fountain opener from the first shop burst, while core, fourth, fifth, and sixth panels are finished-item slots. It should dedupe core items and avoid showing the same core item again as a late option unless no better option exists.
- Future refinement: add a small curated champion override map for cases where player terminology is specific and stable, such as Katarina AD/On Hit/Tank or Shyvana AP/Tank.

Known gaps:

- Ban rate is sample-relative: `champion bans / stored ranked matches` for the selected patch. Riot does not expose a separate global champion-ban-rate endpoint, so this should be labeled as our stored sample.
- Pick rate is a sample-relative estimate from stored participant rows, not global Riot-wide popularity.
- Tier-list impact is still a correlation-heavy score. It now uses final participant performance fields, but those signals should be validated as the corpus grows. See `docs/product/tier-list-ranking.md`.
- Item paths use build-signature summaries for normal production reads. The first-three completed item signatures still come from timeline purchase order where available, while final inventory signatures are Riot inventory order and not guaranteed purchase order.
- Slot panels remain useful for matchup-specific item choice, especially when a complete path sample is too thin. Slot `0` is reserved for starting items; slots `1-6` remain completed-item purchase order.
- Starting-item display prefers opening loadout signatures from the first purchase burst only. The full bundle must fit under the normal starting-gold cap, which preserves legal potion/control-ward openers while filtering out early recall buys such as impossible Doran plus Long Sword bundles. If no loadout sample exists, the UI falls back to the older slot `0` single-item rows.
- Live matchup slot panels should not mix champion-wide fallback rows into the matchup card. Exact-matchup scope can widen across stored patches, while champion-wide baseline stays in the separate overall card.
- Displayed slot rows apply a small sanity pass for player readability, including suppressing duplicate boots. The underlying API still returns the candidate rows; the UI chooses a plausible one-boot display.
- The champion-page bundle is the preferred contract for the champion guide screen. The build-advice endpoint remains the focused contract for live matchup panels and specialty build surfaces. Lower-level item-slot and champion-guide endpoints can stay available for debugging and specialty pages.
- Skill paths are real timeline-derived paths. Recommended builds use champion/role/rank/patch skill order, while alternative build tabs can show a build-family-specific skill order when retained timeline data exists. A build family currently needs at least 10 retained skill-path matches, or the higher requested guide `minGames` value, before its own skill order is shown. Below that floor, the UI falls back to the champion-level skill order.

The build guide page should remain a reference surface. Live match mode can point users toward contextual matchup stats, while guide mode lets users explore champion-wide and matchup-specific patterns calmly before queueing.
