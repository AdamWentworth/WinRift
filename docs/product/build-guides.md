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
- alternative build variants grouped by meaningful early core items, with player-facing labels such as AP, On Hit, Tank, Crit, Lethality, AD Bruiser, Enchanter, and Support Tank
- starting-item and completed-item slot panels from the precomputed `item_slot_analytics` read model

Backend contracts:

- `GET /api/analytics/build-advice`: unified build payload for one champion, optional opponent, role, patch, and rank scope. It returns matchup item slots, champion-wide baseline item slots, top build signatures, runes, spells, item paths for diagnostics, sample quality, and fallback notes in one response.
- `GET /api/analytics/champion-guides`: index of champion summaries for the current role/patch/rank scope. This powers champion coverage and lets the UI expose every champion we have stored data for.
- `GET /api/analytics/champion-guide`: focused champion guide payload for one champion and role.
- `GET /api/analytics/item-slots`: slot-level item read model with matchup-aware fallback.
- `POST /api/dev/analytics/champion-guides/refresh`: local/dev refresh endpoint. With `backfill: true`, it derives participant performance, skill events, and champion bans from retained raw payloads before rebuilding guide read models.

Build variant labeling:

- Variants should come from WinRift's own match corpus, not from a runtime scrape of another stats site.
- External guide pages can be used as research inspiration for common player language, but the production path should remain our own classifier plus curated overrides where the data needs champion-specific wording.
- The first pass ignores boots, starter items, jungle pets, support quest items, consumables, and common components when deciding the variant identity.
- Variant labels are intentionally broad. If several core item families resolve to the same label, such as multiple Katarina AP paths, those rows should be summed into one player-facing build family.
- The `Recommended` tab is not a variant lane. It should use the broad champion/matchup build-advice data and the highest-support aggregate setup, while alternative tabs intentionally narrow to their detected family.
- Recommended item slots use a support-aware score, with samples ramping toward full trust around 200 games, so tiny hot samples do not outrank much larger, well-supported choices.
- The champion page should keep the lower item panels as the primary build presentation. The first panel is opening purchases from the first two minutes; core, fourth, fifth, and sixth panels are finished-item slots. It should dedupe core items and avoid showing the same core item again as a late option unless no better option exists.
- Future refinement: add a small curated champion override map for cases where community jargon is specific and stable, such as Katarina AD/On Hit/Tank or Shyvana AP/Tank.

Known gaps:

- Ban rate is sample-relative: `champion bans / stored ranked matches` for the selected patch. Riot does not expose a separate global champion-ban-rate endpoint, so this should be labeled as our stored sample.
- Pick rate is a sample-relative estimate from stored participant rows, not global Riot-wide popularity.
- Tier-list impact is still a correlation-heavy score. It now uses final participant performance fields, but those signals should be validated as the corpus grows. See `docs/product/tier-list-ranking.md`.
- Item paths use timeline-derived first-three completed item signatures where available, then final inventory signatures for the completed build. Final inventory order is still Riot inventory order, not guaranteed purchase order.
- Slot panels remain useful for matchup-specific item choice, especially when a complete path sample is too thin. Slot `0` is reserved for starting items; slots `1-6` remain completed-item purchase order.
- Live matchup slot panels should not mix champion-wide fallback rows into the matchup card. Exact-matchup scope can widen across stored patches, while champion-wide baseline stays in the separate overall card.
- Displayed slot rows apply a small sanity pass for player readability, including suppressing duplicate boots. The underlying API still returns the candidate rows; the UI chooses a plausible one-boot display.
- The build-advice endpoint is the preferred contract for future profile/live/champion build UI work. Lower-level item-slot and champion-guide endpoints can stay available for debugging and specialty pages.
- Skill paths are real timeline-derived paths, but they are still aggregated by champion/role/rank/patch rather than matchup-specific skill paths.

The build guide page should remain a reference surface. Live match mode can point users toward contextual matchup stats, while guide mode lets users explore champion-wide and matchup-specific patterns calmly before queueing.
