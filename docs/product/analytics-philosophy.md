# Analytics Philosophy

WinRift should be a context engine, not a command engine.

The goal is to help a League player understand the shape of a live game: what each team composition is naturally good at, what item paths have historically worked in the champion matchup, how much data supports those patterns, and where the uncertainty is. The app should not pretend it can issue perfect live instructions.

## Product Promise

WinRift should answer questions like:

- What kind of game does this team composition want?
- What kind of game does the enemy composition want?
- Which build paths have worked for this champion into this opponent?
- Is this pattern backed by enough games to trust, or is it thin?
- Does the game-length history suggest a short-game or long-game bias?

WinRift should avoid saying things like:

- "Buy this now."
- "Force Baron."
- "This composition automatically wins."
- "This item is best" without sample context.

## Three Truth Layers

### Collected Facts

These come from Riot payloads and normalized ClickHouse rows.

Examples:

- Match id, patch, platform, queue, duration, winner.
- Champion, team, role, summoner spells, runes, items.
- Timeline item purchases, participant frames, combat events, objective events.
- Cached summoner ranks and Riot ID aliases.

This layer should stay as objective as possible.

### Hand-Authored Strategy

These are human-authored interpretations of champion identity.

Examples:

- Champion win-condition profiles.
- SplitPush, Pick, Siege, Control, and TeamFight scores.
- Individual archetype labels such as TeamFight or Flex.

This layer is valuable because it encodes League understanding that raw data will not infer cleanly at small sample sizes. It must be versioned, reviewed, and labeled as authored strategy data.

### Empirical Outcomes

These are aggregates derived from collected facts plus optional strategy annotations.

Examples:

- Champion-vs-opponent item slot winrates.
- Build signature winrates.
- Win-condition pairing winrates.
- Game-length bucket outcomes.
- Confidence and sample-size warnings.

This layer is where the app earns trust. It should surface uncertainty instead of hiding it.

## Display Rules

- Prefer "historically" and "in collected matches" language.
- Show game counts beside percentages.
- Use confidence or sample warnings when data is thin.
- Make fallback scope visible when possible, such as all regions, all ranks, or all roles.
- Keep live-game UX policy-safe: contextual stats are fine; direct live shot-calling is not the product.

## Current Product Direction

The live match screen should combine:

- familiar summoner lookup and team card layout,
- player rank/champion context,
- best observed item patterns for each champion matchup,
- win-condition identity for both teams,
- deterministic match reads that explain the stats in plain language.

This combination is the point of differentiation. Many tools can show generic champion builds. WinRift should make composition identity and opponent-specific build context feel understandable in one glance.
