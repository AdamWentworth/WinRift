# Win Condition Validation

The win-condition model should be treated as a testable strategic model, not a finished truth. This note describes the first validation pass now available from the backend.

## Endpoint

```bash
curl 'http://localhost:8000/api/analytics/win-conditions/validation?patch=16.17&platform=ALL&rankBucket=ALL&minGames=50' | jq
```

Query parameters:

- `patch`: patch to validate. If omitted, the latest patch in `match_team_win_conditions` is used.
- `platform`: platform to scope to, or `ALL`.
- `rankBucket`: rank bucket to scope to, or `ALL`.
- `queueId`: defaults to ranked Solo/Duo, `420`.
- `minGames`: minimum sample threshold for primary matchup rows and generated findings. Defaults to `50`.
- `weakSignalWinRate`: winrate threshold for warning about low-rated high-winrate rows. Defaults to `55`.
- `limit`: maximum primary matchup rows and weak-signal warnings. Defaults to `25`.
- `synergyMinGames`: minimum games for champion-pair residual rows. Defaults to `25`.
- `synergyLimit`: maximum champion-pair residual rows. Defaults to `25`.
- `synergyParentLimit`: number of high-signal primary matchup rows to inspect for residuals. Defaults to `6`.
- `synergyMinParentSignal`: minimum primary matchup signal required before checking pair residuals. Defaults to `1`.

## What It Tests

The report reads `match_team_win_conditions`, which is one derived row per team per match. That means the report tests the hand-authored champion scores after they have been summed into actual team compositions.

It returns:

- `ratingOutcomes`: winrate by strategy axis and letter rating.
- `scoreDeltaOutcomes`: winrate when a team has more or fewer points than the enemy on the same axis.
- `primaryMatchups`: winrate for primary strategy into enemy primary strategy.
- `primaryMarginOutcomes`: winrate by how sharply a team's top strategy beats its second-best strategy.
- `synergyResiduals`: champion-pair rows that overperform or underperform the parent primary matchup bucket.
- `weakSignalWarnings`: low-rated strategy rows that still show high winrate.
- `findings`: generated plain-English notes from the report.

`primaryMatchups` are sorted by directional Wilson signal, then raw edge and sample size. Each row includes:

- `edge`: raw winrate minus 50.
- `wilsonLow` / `wilsonHigh`: 95% Wilson interval.
- `direction`: `favorable`, `unfavorable`, or `mixed`.
- `signal`: how far the Wilson interval clears 50%. A `2.4` signal means the confidence interval is at least 2.4 percentage points away from even.

This keeps the report from overvaluing one noisy 60% row or one giant 50% row. If a primary matchup has no signal, it may still be worth watching, but it should not drive scoring changes yet.

## How To Read It

### Rating Outcomes

This is the first sanity check for the grading curve.

If `A` and `S` ratings on an axis do not perform differently from `C` or `D` ratings, that does not automatically mean the axis is wrong. It may mean:

- the rating thresholds are off,
- the data is still thin,
- the axis only matters into certain opponents,
- champion strength is overpowering composition identity,
- the model needs role or synergy terms.

### Score Delta Outcomes

This is the cleanest first test of the additive model.

For each axis, the report compares team score against enemy score:

```text
<=-8, -7..-4, -3..-1, 0, 1..3, 4..7, 8+
```

If the model has signal, positive buckets should generally outperform negative buckets over large samples. This does not need to be perfectly monotonic. League is noisy, and the axis may only be valuable when paired with the right enemy strategy or game length.

### Primary Matchups

This keeps the legacy idea intact:

```text
Your primary strategy vs enemy primary strategy
```

It is useful for asking questions like:

- Does `Pick B+` reliably beat `Siege B`?
- Does `Control A` struggle into `SplitPush B+`?
- Are some pairings mostly sample noise?

This is currently the most promising validation layer because it tests the actual legacy premise: a composition identity into an opposing composition identity. Same-axis score advantage is a rough additive-model test; primary matchups are the real strategy-pair test.

Rows should be interpreted by `signal`, not only raw winrate:

```text
58% WR, 40 games, signal 0     = interesting but noisy
53% WR, 700 games, signal 1.5  = modest but more trustworthy
47% WR, 700 games, signal 1.5  = modest unfavorable signal
50% WR, 2,000 games, signal 0  = stable even matchup
```

### Champion-Pair Residuals

This is the first pass at the synergy question.

For the strongest primary-vs-primary matchup rows, the backend inspects:

- teammate champion pairs on the team being evaluated,
- opponent champion pairs on the enemy team.

Each row compares the pair's winrate against the parent strategy matchup winrate, not against global 50%.

Example:

```text
Parent: TeamFight A- into TeamFight B+ wins 63%
Pair: champion 111 + 157 wins 75% in that same parent bucket
Residual: +12 pts
```

That does not immediately prove champion synergy. It says this pair is carrying, amplifying, or distorting the parent strategy signal enough to deserve inspection.

Fields:

- `pairType`: `teammate` or `opponent`.
- `championId1` / `championId2`: sorted champion ids for the pair.
- `parentWinRate`: winrate of the parent primary matchup row.
- `residual`: pair winrate minus parent winrate.
- `direction`: `overperforming`, `underperforming`, or `mixed`.
- `signal`: how far the pair's Wilson interval clears the parent winrate.

For `opponent` pair rows, winrate and direction are still from the selected team's perspective. An overperforming opponent-pair row means the selected strategy won more often when that enemy pair appeared; it does not mean the enemy pair itself performed well.

Use this report to decide whether a win-condition edge is broad or cluster-driven:

```text
Many small mixed residuals        = strategy signal is probably broad
One huge teammate pair residual   = possible synergy or champion-strength artifact
One huge opponent pair residual   = enemy pair may counter or inflate the read
```

### Primary Margin

Primary margin is the distance between a team's highest axis and second-highest axis.

This should not be read as "sharp teams always win more." A sharp identity can be strong or brittle. The useful question is whether sharp, flexible, and tied identities have different outcome patterns.

### Weak Signal Warnings

These are rows like:

```text
SplitPush C with high winrate into Control B
```

The warning does not say the row is useless. It says the causal story is suspicious. A team with a low splitpush rating probably did not win because it executed splitpush well. More likely, those teams won through their stronger plan while also matching that low-rated row.

These rows are good candidates for future timeline validation:

- lane assignment,
- side-lane pressure,
- objective trades,
- gold/xp by minute,
- tower damage and structure timing.

## Current Limits

This is still correlation analysis.

It does not prove:

- a team actually played through the selected strategy,
- an item or rune choice caused the outcome,
- a duration bucket is a true power-spike graph,
- a high winrate row should be recommended to players.

It does help decide where the model deserves trust, where the thresholds may be wrong, and where we need deeper timeline features.

## Next Improvements

Good follow-ups:

- Add role-specific validation to see whether the same champion profile behaves differently by role.
- Add champion-pair residuals to detect synergy and anti-synergy.
- Compare static letter ratings against patch-relative score percentiles.
- Add timeline validation for strategies, especially splitpush and siege.
- Store generated validation snapshots per patch so model revisions can be compared over time.

## Snapshots

- [2026-05-31 Patch 16.10 Snapshot](./win-condition-validation-2026-05-31.md)
