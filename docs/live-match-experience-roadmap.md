# Live Match Experience Roadmap

This note captures near-term live match UX ideas. The goal is not to clone OP.GG feature for feature. WinRift should use familiar live-match context as the shell, then make matchup/build and team-strategy analytics the thing it does better.

Current live-game constraints:

- Riot live-game data gives champions, teams, spells, visible keystone, primary rune tree, secondary rune tree, queue, map, and game start time.
- Riot live-game data does not expose the full rune page, so the UI should not pretend those selections are known before the match is collected.
- Ranked records, champion records, recent history, build patterns, role confidence, and team strategic profiles must come from our stored data or secondary Riot requests with rate-limit budgeting.

## Current State

- Live lookup by Riot ID and platform.
- Tagless lookup through saved account aliases.
- Live board with champion cards, ranks, champion performance, spells, compact rune icons, drag reorder, and matchup item-slot stats.
- Role correction using one-Smite jungle detection plus champion role-rate data from ClickHouse.
- Live backfill seeding for players with low champion samples.
- Win-condition panel backed by the validated champion profile fixture and precompiled ClickHouse win-condition metrics.

## High-Value Additions

### Team Average Rank

Show a small team-level line:

- Blue average: Emerald 1
- Red average: Emerald 1
- Lane deltas where useful, for example top is +2 divisions or bot duo is -1 division.

Implementation notes:

- Use enriched `rank` objects already attached to live participants.
- Convert tier/division/LP to sortable values with the same rank bucket logic used by collection.
- Keep unknown/unranked participants visible in the denominator note, not hidden.

### Live Game Clock

Show elapsed time from `gameStartTime`, formatted like `15:06`.

Implementation notes:

- Compute client-side with a one-second timer.
- If `gameStartTime` is missing or zero, hide the clock.

### Role Confidence

Show why each card is in its lane:

- `Smite` for the unique Smite holder.
- `Role data 87%` when champion role-rate evidence is strong.
- `Fallback order` when data is thin.
- `Manual` after the user drags cards.

Implementation notes:

- The current role assignment function already computes enough information to expose this.
- Store role assignment metadata beside ordered participants instead of returning only a reordered array.

Current implementation:

- Live cards show a compact role-confidence chip.
- Unique Smite junglers display `Smite lock`.
- Strong role-rate evidence displays `Role data N%`.
- Thin role-rate evidence displays `Thin role N%`.
- Manually dragged cards display `Manual slot`.
- Fallback assignments display `Fallback order`.

### Recent Champion/Role History

Add an OP.GG-style recent-history strip per player:

- Last 10 ranked games, champion icon, spells, result, KDA, and role.
- Filter toggle: current champion, current role, or all ranked games.
- Use this to surface comfort rather than only raw account rank.

Implementation notes:

- This should come from stored participant rows where possible.
- If a live lookup queues backfill, show "history collecting" rather than making the live path spend heavy API budget.

### Comfort Flags

Compact flags that help interpret a player:

- Low sample on champion.
- Off-role pick.
- Champion specialist.
- Autofill-looking role.
- Recent poor sample warning.

These should be contextual, not insulting, and should avoid deterministic language.

Current implementation:

- Cards show at most two compact context flags.
- Champion comfort is based on stored champion sample size and winrate.
- Low or missing champion samples are labeled as sample limitations.
- Rank pending and strong ranked form are shown from the live enriched rank object.

### Matchup Build Context

Keep deepening the current item-slot panels:

- Matchup-specific first through sixth item patterns.
- Broader fallback tiers when matchup samples are low: champion vs role, champion overall in role, rank bucket, all ranks.
- Sample-size labels and data source labels.
- Summoner spell/rune headline patterns only where the data is available from collected matches.

### Win Condition Profile

Bring back the legacy team strategy idea as a live-match layer:

- Team profile bars for SplitPush, Pick, Siege, Control, and TeamFight.
- Primary and secondary strategic identities.
- Opposing strategic matchup history by duration bucket.
- Later, combine team strategy with build stats so item patterns can be viewed in context of the whole composition.

Current implementation notes:

- `POST /api/analytics/win-conditions` accepts the two live five-champion compositions and returns team profiles plus matchup stats.
- Raw match and participant rows are not tainted with hand-authored scores. The API scores compositions at read time from `services/core/internal/winconditions/champion_profiles.json`.
- Historical stats are currently derived from retained `participants` + `raw_matches` rows. Once old raw patches are compacted, this should move into patch-compiled win-condition metric tables.
- Duration buckets are labeled as game-length outcomes, not true minute-by-minute win probability.

See [Legacy Win Condition Audit](legacy-win-condition-audit.md).

## Nice-To-Have

- Spectate link/button if we can support it reliably without taking focus away from analytics.
- Team average champion mastery or champion-specific familiarity if a stable data source is available.
- Pick/ban context if available from live payloads for the relevant queue.

## Skip For Now

- OP Score / LN Score clones. Proprietary scores are not core to WinRift.
- Full rune-page hover overlays in live view. The live endpoint does not give complete selections.
- Direct "do this now" recommendations. Keep language contextual and statistical.

## MVP Order

1. Team average rank and live clock.
2. Role confidence labels.
3. Recent champion/role history panel.
4. Comfort flags.
5. Combined build plus team-strategy analytics.

## Current UI Refinement Pass

The live match screen should feel like one cohesive match dashboard, not separate widgets stacked together. The near-term design pass is:

- Add a compact match header with queue, platform, game clock, searched side, patch context, and team average rank.
- Add fixed lane labels so build cards, champion cards, and matchup analytics visually lock to Top/Jungle/Mid/Bot/Support.
- Make player cards more scannable: champion identity first, then ranked record, champion record, spells, and runes.
- Make build cards read as "best observed item path into this opponent" with clear sample-size language and better no-data states.
- Turn the win-condition match read into a structured card with "play toward," "watch for," timing, evidence, and sample notes.
- Make alternatives feel like lightweight strategy chips instead of another large card competing with the primary read.
- Keep the duration chart visually strong, but keep its language honest: these are game-length outcome buckets, not live minute-by-minute win probability.
- Split the large live component into smaller components once the visual direction settles, so future changes are easier to reason about.
