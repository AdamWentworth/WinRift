# Build Guides

WinRift now has a non-live build guide surface alongside live match lookup.

The guide page can learn from public champion guide pages, but it should keep WinRift's own tone: darker match-room panels, visible sample coverage, matchup-first framing, and clear caveats when the data is thin.

The current version can show:

- champion, role, patch, rank, and optional opponent filters
- stored coverage for every champion in the selected role/patch/rank scope
- champion win rate, match sample, pick-rate estimate, confidence, and role rank within the stored sample
- rune page aggregates from stored `rune_signature` values
- summoner spell aggregates from stored `spell_signature` values
- toughest and favorable opponent matchups from stored participant matchup rows
- item slot panels from the precomputed `item_slot_analytics` read model

Backend contracts:

- `GET /api/analytics/champion-guides`: index of champion summaries for the current role/patch/rank scope. This powers champion coverage and lets the UI expose every champion we have stored data for.
- `GET /api/analytics/champion-guide`: focused champion guide payload for one champion and role.
- `GET /api/analytics/item-slots`: slot-level item read model with matchup-aware fallback.

Known gaps:

- Skill order is not normalized yet. The UI marks it as pending instead of guessing.
- Ban rate is not available from Match-V5 and should not be shown unless another trustworthy source is added.
- Pick rate is a sample-relative estimate from stored participant rows, not global Riot-wide popularity.
- Item paths are currently slot aggregates, not full path sequence aggregates. This is useful for matchup-specific item choice, but it is not identical to a U.GG full build path.
- Raw timelines probably contain enough information to derive skill order through `SKILL_LEVEL_UP` events, but those event fields are not normalized yet. We should add a dedicated table/read model before presenting skill paths as real data.

The build guide page should remain a reference surface. Live match mode can point users toward contextual matchup stats, while guide mode lets users explore champion-wide and matchup-specific patterns calmly before queueing.
