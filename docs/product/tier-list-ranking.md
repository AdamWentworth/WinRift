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

## Follow-Up Notes

The current tier-list pass is good enough for MVP, but it should be revisited after the stored corpus is much larger.

Things that feel solid:

- strict role buckets for champion rankings, while still letting flex champions count in the role they actually played
- S+ as a score threshold instead of a fixed quota
- sample-shrunk winrate plus Wilson confidence, so tiny hot samples do not dominate
- lower ban-rate weight, because ban rate can reflect frustration or popularity as much as champion strength
- top-lane durability pressure as part of impact, especially damage taken/mitigated while not dying

Things to validate later:

- Whether `tierScore >= 59` remains the right S+ display threshold once each role has far more games.
- Whether the current role weights produce sensible leaderboards by role across multiple patches.
- Whether durable top-lane pressure is over- or under-valued compared with splitpush pressure, lane leads, and objective pressure.
- Whether impact should use per-minute fields everywhere instead of final totals.
- Whether low-pick one-trick champions need a mastery-bias caveat or a separate niche-strength label.
- Whether ban rate should be capped, log-scaled, or split into "respect ban" versus "annoyance ban" once we have more context.
- Whether matchup-adjusted winrate should become the main rank signal instead of global role winrate.

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
