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

- Role confidence is used for ordering, but is not shown on the default card UI.
- Unique Smite still heavily locks jungle assignment.
- Champion role-rate evidence still informs lane ordering.
- Manual drag still overrides automatic ordering.
- If this is shown later, it should be in a tooltip/debug affordance rather than always-visible chips.

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
- Raw match and participant rows are not tainted with hand-authored scores. Team strategy rows are derived into `match_team_win_conditions` from `core/internal/winconditions/champion_profiles.json`.
- Historical stats are served from precompiled `patch_win_condition_metrics` rows. The worker refreshes the current patch on a short interval so the live page does not recompute winrates while a player is waiting.
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
2. Role-aware card ordering using smite and collected champion role rates.
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

Recent refinements:

- Champion cards use splash art backdrops, with champion stats paired against ranked record in separate columns.
- Ranked tier icon lives above the ranked record area so it mirrors the champion portrait/champion-stats relationship.
- Role confidence is used behind the scenes for ordering but is hidden from the default card UI.
- Build cards now show prioritized item-slot reads, clear sample-size language, quieter no-data states, and less tiny explanatory text.
- Build cards keep exact matchup rows and champion-wide baseline rows separate. Sparse exact matchups can widen to all stored patches, but they do not silently borrow champion-wide slots.
- Live lookup loading and miss states are presented as status plates instead of loose text.
- Win-condition match reads surface the selected `Your condition vs Enemy condition` pairing, evidence strength, game count, and likely winrate range near the headline.

## Static Art And Deployment Notes

The global background slideshow uses Riot/Data Dragon art through URL references, not checked-in image files. The canonical implementation note is `docs/product/global-background-system.md`.

UI policy:

- Every primary page should render above the shared animated art background.
- General pages use the full champion/skin splash manifest.
- Specific champion guide pages use only that champion's base splash, skins, and related Data Dragon splash art.
- Page panels can add dark overlays and local hero images, but they should not replace the shared background policy with unrelated decoration.

Current flow:

- The API serves `GET /api/static/champion-splashes`.
- That endpoint reads Data Dragon champion metadata, expands champions into base-skin plus skin splash URLs, filters chroma/variant rows that do not have standalone splash files, and caches the resulting manifest in API memory.
- The web app randomizes across that manifest and the browser loads the selected image directly from Data Dragon's CDN. When a champion id is provided, the slideshow first filters the manifest to that champion's skins.
- The repo stores only code and URL-generating logic. It does not store Riot artwork.

This is the right default for development and early self-hosted deployment because it avoids thousands of large binary files in git and avoids local disk growth.

Deployment options later:

- Keep CDN mode as default. This is simplest and should be good enough while the app is online-only.
- Add an optional external asset cache outside git if CDN latency or availability becomes a problem, for example `/srv/winrift/assets/ddragon/splash/Aatrox_7.jpg` on the server or NAS-backed storage.
- If self-hosting assets, keep them in a mounted volume, not in the repository. A future asset sync command could read `/api/static/champion-splashes`, download missing images, and rewrite served URLs to local static paths.
- Do not commit Riot splash art to git. It bloats the repo and creates unnecessary licensing/distribution risk.

## Live Match Mode Rail

A side mode rail is a strong direction for the live match page. The screen is starting to contain several valuable but competing ideas: matchup item paths, team win-condition analysis, player rank/champion comfort, and eventually recent history. Showing everything at once will keep making the live page taller, denser, and harder to read.

Recommended model:

- Keep the match header visible in all modes.
- Add a narrow vertical mode rail on the left side of the live board on desktop.
- On mobile, turn the same modes into a compact segmented control above the lane tabs.
- Swap the analytics layer based on mode instead of stacking all analytics at once.

Current modes:

- Match: default scout view with lane labels and the two teams' champion cards. No heavier analytics are loaded.
- Builds: focused item-path view. It defaults to the searched player and lane opponent, but both the build target and opponent can be changed from the live match. It shows exact matchup slot reads beside a separate champion-wide baseline, so a player can compare "what works into this enemy" against "what generally works on this champion" without mixing those scopes into one card.
- Win Conditions: show champion rows with the win-condition analysis band between teams.
- Player Form later: deeper recent games, off-role/specialist flags, and live backfill status if the card stats need more context.

Why this is good:

- It reduces clutter without deleting any of the work.
- It lets each analytic surface breathe visually.
- It gives the app a clearer mental model: same live match, different analytical lenses.
- It gives us room to add future modes without turning the page into one giant spreadsheet.

Implementation notes:

- `liveMode` state in `LiveMatchups` is `'match' | 'builds' | 'winConditions'`.
- A small mode context banner now sits below the match header. It names the current lens, makes the lazy-loading behavior explicit, and gives users a short reminder of what that mode is for.
- Gate the build-advice query so it only runs in Builds mode. The unified `GET /api/analytics/build-advice` response returns matchup slots, champion baseline slots, runes, spells, paths, sample quality, and fallback notes in one payload.
- Build rows use minimum sample thresholds before an item can appear: 5+ games for exact matchup slots and 10+ games for champion baseline slots. Results are still sorted by Wilson lower-bound confidence after the threshold, so a 1-game 100% item does not outrank a more established pattern.
- Matchup cards can fall back from current-patch exact matchup to all-patch exact matchup, but they do not borrow champion-wide rows. The champion-wide baseline is shown separately.
- Build cards should prioritize context without tiny-text overload: weighted shown-item winrate, shown sample volume, matchup delta/reference chip, one sample-strength chip, one caveat max, and the item slot row.
- Displayed slot rows should avoid impossible reads such as two boots in one build. If duplicate boots would appear, keep the stronger boot row and look for the next non-boot candidate in the other slot.
- Gate win-condition query so it only runs in Win Conditions mode, unless we later want to prefetch after idle.
- Keep the current champion card drag/drop shared across all modes because role correction affects every mode.
- Use icons plus short labels in the rail. Good icon choices from lucide: sword/build, network/strategy, user/player form.
- Default mode is Match because the first live lookup should answer "who is in my game?" before opening an analytic lens.

Open UX questions:

- Whether the rail belongs on the left or right. Left is discoverable; right may stay out of the way of the WinRift logo/search header.
- Whether the mode context banner remains permanently or eventually collapses once the UI is obvious.
- Whether Win Conditions should become the default when build sample sizes are very thin. Probably not yet; better to keep defaults predictable.
