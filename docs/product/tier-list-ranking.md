# Tier List Ranking

The tier list should not rank champions by raw winrate alone. A one-game 100% champion should not beat a broadly proven 53-54% champion, and a high-winrate niche pick should read differently from a high-winrate, high-presence meta pick.

## Current Formula

The backend returns a `tierScore` from `GET /api/analytics/champion-guides`. The frontend uses that score for default tier-list ordering.

Current score components:

- `winScore`: highest weight. Blends sample-shrunk raw winrate with Wilson lower-bound confidence so strong but fragile samples are pulled back.
- `sampleScore`: rewards larger stored samples inside the selected role/patch/rank scope.
- `pickScore`: sample-relative pick presence. This captures how often the champion appears in our stored games for the selected scope.
- `banScore`: sample-relative ban pressure from stored Match-V5 ban data.
- `impactScore`: role-scope-relative player impact from stored participant performance stats. The current backend blends KDA, kill participation, champion damage, economy, vision, objective pressure, utility, and survivability with role-specific weights.

Current weights:

- 70% win score
- 17% impact score
- 8% sample score
- 5% presence score, where presence is mostly pick pressure with a smaller ban-pressure component

Raw winrate is shrunk toward 50% until a champion reaches roughly 250 games in the selected scope, and Wilson confidence now carries most of the win score. This intentionally keeps winning as the center of gravity while making tiny hot samples less likely to outrank broad, stable performers. Popularity, stability, ban respect, and player impact then break ties. Ban rate is deliberately a small signal because it can represent frustration, fame, or matchup avoidance rather than actual winning strength.

The score also applies a losing-performance guardrail. If a champion is below 50% winrate, especially below 49%, the score is pulled down so pick/ban pressure and broad sample size cannot crown a losing pick as an elite performer by themselves. This keeps "popular and feared" separate from "actually winning enough to deserve the top tier."

## Tiers

The frontend maps WinRift rank into presentation tiers. `S+` is not a fixed number of champions per role, and it is not awarded from raw winrate alone. It is earned from the same composite score that drives the tier-list ordering:

- `tierScore >= 59`
- winrate at least 50%
- fallback only when `tierScore` is unavailable: top 5% by role rank

This keeps `S+` attainable when the data clearly supports it, without reserving slots or forcing every role to have the same count. A champion with a flashy raw winrate still needs enough total performance signal to cross the composite threshold. After `S+`, the remaining tiers use broad percentile bands:

- `S`: top 22%
- `A`: top 40%
- `B`: top 65%
- `C`: top 85%
- `D`: remaining rows

The tier label is a presentation bucket. The backend `tierScore` is the actual sortable number.

When a backend score is available, the frontend maps tiers directly from `tierScore` before falling back to rank percentile. It also caps losing records: a sub-49% champion cannot display as `S` or `S+` from presence alone. A champion can still rank highly enough to be watched, but the visual tier should not imply elite performance when the stored games say it is losing.

Champion guide pages use a stable 50-game floor for the role-rank comparison field. The guide can still show thinner rune, item, matchup, and skill signals where that is useful, but `Rank X / Y` should not compare a 2,000-game champion against 5-game off-role noise.

## Impact Normalization

Match-V5 final participant fields are normalized into `participants` for new ingestion and into `participant_performance` for retained raw-match backfills. Champion guide queries use `participant_performance` when it exists and fall back to participant columns otherwise.

The current impact components are:

- `damageScore`: final champion damage relative to the selected role scope.
- `economyScore`: gold earned plus total CS.
- `visionScore`: final vision score.
- `objectiveScore`: objective damage, structure damage, turret/inhibitor takedowns, dragon/baron kills, and objective steals.
- `utilityScore`: CC time plus healing/shielding utility.
- `survivabilityScore`: lower dead-time share, self-mitigation relative to damage taken, and damage absorbed per minute while still staying alive.
- KDA/kill participation blend: rewards involvement without letting cleanup KDA fully dominate.

Role weighting matters. Supports lean more on vision and utility; junglers lean more on objectives; marksmen lean more on damage/economy/survivability. Top lane now weights durable pressure more heavily than mid, while mid weights damage and fight participation more heavily.

Champion guide and tier-list rankings use strict role buckets. This does not exclude flex champions from off-roles; Yasuo top still counts as top-lane Yasuo when he is actually played top. It only prevents Yasuo mid, Katarina mid, Ahri mid, Zed mid, Zoe mid, and similar games from leaking into top-lane performance math. Build matchup advice can still merge top and mid when that makes strategic sense.

## Current Limitations

These stats are still correlations, not isolated champion power. Better champions attract better players, some champions farm more because of role/function, and losing teams naturally have worse damage/economy/vision. The score is useful for ranking stored performance patterns, but it should stay labeled as WinRift's internal read rather than objective truth.

## Validation Boundaries

The score is reviewed against larger samples and across patches rather than treated as a permanent formula. The highest-value validation questions are:

- whether the `S+` threshold remains calibrated as role samples grow;
- whether role weights produce stable, credible ordering across patches;
- whether final totals should be replaced by per-minute or share-of-team signals;
- whether matchup strength, lane state, composition, and one-trick concentration materially change the ordering;
- whether ban pressure needs a cap or transformation to separate competitive respect from frustration.

Any future signal must improve held-out or cross-patch behavior before it receives user-facing weight. More inputs are not automatically a better ranking.

## Product Language

The UI should call this `WinRift Score`, not "truth" or "best champion." It is a broad meta read from our stored ranked Solo/Duo corpus.
