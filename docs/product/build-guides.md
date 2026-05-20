# Build Guides

WinRift now has a non-live build guide surface alongside live match lookup.

The guide page can learn from public champion guide pages, but it should keep WinRift's own tone: darker match-room panels, visible sample coverage, matchup-first framing, and clear caveats when the data is thin.

The current version can show:

- champion, role, patch, rank, and optional opponent filters
- stored coverage for every champion in the selected role/patch/rank scope
- champion win rate, match sample, pick-rate estimate, confidence, and role rank within the stored sample
- tier-list scoring signals: WinRift score, win score, sample score, pick score, ban score, and KDA-based impact score
- rune page aggregates from stored `rune_signature` values
- summoner spell aggregates from stored `spell_signature` values
- skill priority/path aggregates from stored `SKILL_LEVEL_UP` timeline events
- ban-rate estimates derived from champion bans in stored Match-V5 payloads
- toughest and favorable opponent matchups from stored participant matchup rows
- item slot panels from the precomputed `item_slot_analytics` read model

Backend contracts:

- `GET /api/analytics/champion-guides`: index of champion summaries for the current role/patch/rank scope. This powers champion coverage and lets the UI expose every champion we have stored data for.
- `GET /api/analytics/champion-guide`: focused champion guide payload for one champion and role.
- `GET /api/analytics/item-slots`: slot-level item read model with matchup-aware fallback.
- `POST /api/dev/analytics/champion-guides/refresh`: local/dev refresh endpoint. With `backfill: true`, it derives skill events and champion bans from retained raw payloads before rebuilding guide read models.

Known gaps:

- Ban rate is sample-relative: `champion bans / stored ranked matches` for the selected patch. Riot does not expose a separate global champion-ban-rate endpoint, so this should be labeled as our stored sample.
- Pick rate is a sample-relative estimate from stored participant rows, not global Riot-wide popularity.
- Tier-list impact currently uses KDA because final damage, final CS, final gold, and vision are not yet normalized into participant summaries. See `docs/product/tier-list-ranking.md`.
- Item paths are currently slot aggregates, not full path sequence aggregates. This is useful for matchup-specific item choice, but it is not identical to a U.GG full build path.
- Skill paths are real timeline-derived paths, but they are still aggregated by champion/role/rank/patch rather than matchup-specific skill paths.

The build guide page should remain a reference surface. Live match mode can point users toward contextual matchup stats, while guide mode lets users explore champion-wide and matchup-specific patterns calmly before queueing.
