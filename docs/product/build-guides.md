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
- complete item-path summaries from `core3_signature` and `final_items_signature`
- item slot panels from the precomputed `item_slot_analytics` read model

Backend contracts:

- `GET /api/analytics/build-advice`: unified build payload for one champion, optional opponent, role, patch, and rank scope. It returns matchup item slots, champion-wide baseline item slots, top build signatures, runes, spells, item paths, sample quality, and fallback notes in one response.
- `GET /api/analytics/champion-guides`: index of champion summaries for the current role/patch/rank scope. This powers champion coverage and lets the UI expose every champion we have stored data for.
- `GET /api/analytics/champion-guide`: focused champion guide payload for one champion and role.
- `GET /api/analytics/item-slots`: slot-level item read model with matchup-aware fallback.
- `POST /api/dev/analytics/champion-guides/refresh`: local/dev refresh endpoint. With `backfill: true`, it derives participant performance, skill events, and champion bans from retained raw payloads before rebuilding guide read models.

Known gaps:

- Ban rate is sample-relative: `champion bans / stored ranked matches` for the selected patch. Riot does not expose a separate global champion-ban-rate endpoint, so this should be labeled as our stored sample.
- Pick rate is a sample-relative estimate from stored participant rows, not global Riot-wide popularity.
- Tier-list impact is still a correlation-heavy score. It now uses final participant performance fields, but those signals should be validated as the corpus grows. See `docs/product/tier-list-ranking.md`.
- Item paths use timeline-derived first-three completed item signatures where available, then final inventory signatures for the completed build. Final inventory order is still Riot inventory order, not guaranteed purchase order.
- Slot panels remain useful for matchup-specific item choice, especially when a complete path sample is too thin.
- The build-advice endpoint is the preferred contract for future profile/live/champion build UI work. Lower-level item-slot and champion-guide endpoints can stay available for debugging and specialty pages.
- Skill paths are real timeline-derived paths, but they are still aggregated by champion/role/rank/patch rather than matchup-specific skill paths.

The build guide page should remain a reference surface. Live match mode can point users toward contextual matchup stats, while guide mode lets users explore champion-wide and matchup-specific patterns calmly before queueing.
