# Build Guides

WinRift now has a non-live build guide surface alongside live match lookup.

The guide page is inspired by public champion guide pages such as U.GG, but it should stay honest about what WinRift actually collects. The current version can show:

- champion, role, patch, rank, and optional opponent filters
- champion win rate, match sample, pick-rate estimate, confidence, and role rank within the stored sample
- rune page aggregates from stored `rune_signature` values
- summoner spell aggregates from stored `spell_signature` values
- toughest and favorable opponent matchups from stored participant matchup rows
- item slot panels from the precomputed `item_slot_analytics` read model

Known gaps:

- Skill order is not normalized yet. The UI marks it as pending instead of guessing.
- Ban rate is not available from Match-V5 and should not be shown unless another trustworthy source is added.
- Pick rate is a sample-relative estimate from stored participant rows, not global Riot-wide popularity.
- Item paths are currently slot aggregates, not full path sequence aggregates. This is useful for matchup-specific item choice, but it is not identical to a U.GG full build path.

The build guide page should remain a reference surface. Live match mode can point users toward contextual matchup stats, while guide mode lets users explore champion-wide and matchup-specific patterns calmly before queueing.
