# Tier List Ranking

The tier list should not rank champions by raw winrate alone. A one-game 100% champion should not beat a broadly proven 53-54% champion, and a high-winrate niche pick should read differently from a high-winrate, high-presence meta pick.

## Current Formula

The backend returns a `tierScore` from `GET /api/analytics/champion-guides`. The frontend uses that score for default tier-list ordering.

Current score components:

- `winScore`: highest weight. Blends raw winrate with Wilson lower-bound confidence so strong but fragile samples are pulled back.
- `sampleScore`: rewards larger stored samples inside the selected role/patch/rank scope.
- `pickScore`: sample-relative pick presence. This captures how often the champion appears in our stored games for the selected scope.
- `banScore`: sample-relative ban pressure from stored Match-V5 ban data.
- `impactScore`: role-scope-relative KDA impact from stored participant K/D/A.

Current weights:

- 58% win score
- 14% sample score
- 12% pick score
- 8% ban score
- 8% impact score

This intentionally keeps winning as the center of gravity while letting popularity, ban respect, stability, and player impact break ties.

## Tiers

The frontend maps rank percentile to:

- `S+`: top 3%
- `S`: top 10%
- `A`: top 25%
- `B`: top 55%
- `C`: top 78%
- `D`: remaining rows

The tier label is a presentation bucket. The backend `tierScore` is the actual sortable number.

## Current Limitations

`impactScore` currently uses KDA because K/D/A is normalized into the participant table. It does not yet include final damage, CS, gold, vision, objective damage, or kill participation.

We already store timeline power snapshots for gold, CS, jungle CS, champion damage, and damage taken. That data is useful, but it is currently oriented around 10/15/20 minute power curves rather than final-game champion tiering.

Future improvement:

- Normalize final participant stats from Match-V5 into `participants` or a companion `participant_scores` table.
- Include role-relative damage, CS/gold, vision, objective contribution, and kill participation.
- Keep each signal role-relative. A support's useful impact shape is not the same as a marksman's or jungler's.
- Validate the formula against future sample growth before presenting the score as anything more than WinRift's internal ranking.

## Product Language

The UI should call this `WinRift Score`, not "truth" or "best champion." It is a broad meta read from our stored ranked Solo/Duo corpus.
