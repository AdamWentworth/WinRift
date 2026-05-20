# Tier List Ranking

The tier list should not rank champions by raw winrate alone. A one-game 100% champion should not beat a broadly proven 53-54% champion, and a high-winrate niche pick should read differently from a high-winrate, high-presence meta pick.

## Current Formula

The backend returns a `tierScore` from `GET /api/analytics/champion-guides`. The frontend uses that score for default tier-list ordering.

Current score components:

- `winScore`: highest weight. Blends raw winrate with Wilson lower-bound confidence so strong but fragile samples are pulled back.
- `sampleScore`: rewards larger stored samples inside the selected role/patch/rank scope.
- `pickScore`: sample-relative pick presence. This captures how often the champion appears in our stored games for the selected scope.
- `banScore`: sample-relative ban pressure from stored Match-V5 ban data.
- `impactScore`: role-scope-relative player impact from stored participant performance stats. The current backend blends KDA, kill participation, champion damage, economy, vision, objective pressure, utility, and survivability with role-specific weights.

Current weights:

- 58% win score
- 14% sample score
- 12% pick score
- 8% ban score
- 8% impact score

This intentionally keeps winning as the center of gravity while letting popularity, ban respect, stability, and player impact break ties.

## Tiers

The frontend maps WinRift rank into presentation tiers. `S+` is not a fixed number of champions per role, and it is not awarded from raw winrate alone. It is earned from the same composite score that drives the tier-list ordering:

- `tierScore >= 60`
- fallback only when `tierScore` is unavailable: top 5% by role rank

This keeps `S+` attainable when the data clearly supports it, without reserving slots or forcing every role to have the same count. A champion with a flashy raw winrate still needs enough total performance signal to cross the composite threshold. After `S+`, the remaining tiers use broad percentile bands:

- `S`: top 22%
- `A`: top 40%
- `B`: top 65%
- `C`: top 85%
- `D`: remaining rows

The tier label is a presentation bucket. The backend `tierScore` is the actual sortable number.

## Impact Normalization

Match-V5 final participant fields are normalized into `participants` for new ingestion and into `participant_performance` for retained raw-match backfills. Champion guide queries use `participant_performance` when it exists and fall back to participant columns otherwise.

The current impact components are:

- `damageScore`: final champion damage relative to the selected role scope.
- `economyScore`: gold earned plus total CS.
- `visionScore`: final vision score.
- `objectiveScore`: objective damage, structure damage, turret/inhibitor takedowns, dragon/baron kills, and objective steals.
- `utilityScore`: CC time plus healing/shielding utility.
- `survivabilityScore`: lower dead-time share plus self-mitigation relative to damage taken.
- KDA/kill participation blend: rewards involvement without letting cleanup KDA fully dominate.

Role weighting matters. Supports lean more on vision and utility; junglers lean more on objectives; marksmen lean more on damage/economy/survivability; solo lanes use a more balanced mix.

## Current Limitations

These stats are still correlations, not isolated champion power. Better champions attract better players, some champions farm more because of role/function, and losing teams naturally have worse damage/economy/vision. The score is useful for ranking stored performance patterns, but it should stay labeled as WinRift's internal read rather than objective truth.

Future improvement:

- Validate the role weights against larger samples and adjust by patch.
- Consider per-minute normalization for short games versus long games.
- Add lane matchup difficulty and team-composition context.
- Consider separate early/mid/late impact scores from timeline snapshots.
- Consider pick-ban presence by rank and region once the sample is large enough.

Other signals worth considering later:

- matchup-adjusted winrate, so champions are not rewarded only for farming favorable pairings
- damage share and gold share, not just raw damage/gold
- objective participation by team context, especially for junglers and roam supports
- death timing, since one late death can matter more than three low-stakes early deaths
- lane-dominance proxies from 10/15 minute gold, XP, CS, and damage deltas
- player-mastery bias, since niche champions often have higher one-trick concentration

## Product Language

The UI should call this `WinRift Score`, not "truth" or "best champion." It is a broad meta read from our stored ranked Solo/Duo corpus.
