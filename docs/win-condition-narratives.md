# Win-Condition Narratives

WinRift now has a deterministic narrative layer for win-condition matchup reads.

The source lives in `core/internal/winconditions/narratives.go`.

It covers:

- 5 team conditions by 5 opponent conditions.
- Every rating pairing from `D-` through `S+`.
- Primary-vs-primary, primary-vs-alternative, alternative-vs-primary, and alternative-vs-alternative reads.
- Runtime stats: winrate, games, Wilson confidence, and game-length buckets.

The API attaches a `script` object to every win-condition metric row. The frontend middle panel reads that object directly as the current "Match Read."

`AllNarrativeScripts()` can generate the full deterministic corpus for a future AI summarizer or prompt-grounding pass without committing a huge generated JSON file.
