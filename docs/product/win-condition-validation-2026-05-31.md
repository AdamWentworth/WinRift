# Win Condition Validation Snapshot - 2026-05-31

Scope:

- Patch: `16.10`
- Platform: `ALL`
- Rank bucket: `ALL`
- Queue: ranked Solo/Duo, `420`
- Minimum games for findings: `100`
- Corpus size: `31,630` matches, `63,260` team rows

Command:

```bash
curl 'http://192.168.1.77:8000/api/analytics/win-conditions/validation?patch=16.10&platform=ALL&rankBucket=ALL&minGames=100&limit=5' | jq
```

## First Read

The first validation pass is useful, but it is not flattering to the simplest version of the model.

Same-axis score advantages are weak in this snapshot:

| Axis | Positive Score Delta vs Negative Score Delta |
| --- | ---: |
| SplitPush | `-0.35` pts |
| Pick | `+0.34` pts |
| Siege | `-0.45` pts |
| Control | `+0.58` pts |
| TeamFight | `+1.64` pts |

This does not mean the five-axis model is useless. It does mean raw additive score advantage by itself is not yet a strong predictor of winning.

Likely explanations:

- Champion strength and player skill may dominate composition identity in broad samples.
- The model may need role weighting or synergy terms.
- Some axes may matter more into specific enemy strategies than in global same-axis comparisons.
- Static grade thresholds may be too coarse.
- Teams may not actually play through every strategy they are rated for.

## Rating Outcomes

Selected rating outcomes:

| Axis | Rating | Games | Win Rate | Confidence | Avg Score |
| --- | --- | ---: | ---: | ---: | ---: |
| SplitPush | A | 449 | 53.67% | 49.05 | 19.35 |
| SplitPush | B | 5,569 | 48.66% | 47.35 | 13.40 |
| SplitPush | C | 13,410 | 50.43% | 49.58 | 7.47 |
| Pick | A | 137 | 50.36% | 42.10 | 19.16 |
| Pick | B | 10,250 | 51.11% | 50.14 | 13.41 |
| Pick | C | 10,593 | 49.49% | 48.54 | 7.59 |
| Siege | A | 884 | 49.66% | 46.37 | 19.42 |
| Siege | B | 6,018 | 48.47% | 47.21 | 13.44 |
| Siege | C | 11,426 | 50.53% | 49.62 | 7.48 |
| Control | A | 177 | 51.98% | 44.65 | 19.31 |
| Control | B | 9,241 | 50.20% | 49.18 | 13.41 |
| Control | C | 11,742 | 49.49% | 48.58 | 7.58 |
| TeamFight | A | 431 | 52.90% | 48.18 | 19.26 |
| TeamFight | B | 12,111 | 50.54% | 49.65 | 13.44 |
| TeamFight | C | 8,389 | 49.39% | 48.32 | 7.60 |

The clearest broad signal in this snapshot is TeamFight, but even that is modest. Pick B is also mildly positive with enough games to be worth watching.

## Primary Margin

Primary margin by itself is almost flat:

| Margin Bucket | Games | Win Rate | Confidence |
| --- | ---: | ---: | ---: |
| TIE | 8,935 | 50.39% | 49.35 |
| 1 | 15,533 | 50.11% | 49.33 |
| 2-3 | 20,834 | 49.81% | 49.13 |
| 4-6 | 13,022 | 50.00% | 49.14 |
| 7+ | 4,936 | 49.76% | 48.36 |

This suggests "sharp identity" should be treated as descriptive, not automatically better.

## Low-Rated High-Winrate Warnings

The endpoint flagged rows where a weak strategy label still showed high winrate:

| Strategy | Rating | Opponent | Opp Rating | Mode | Games | Win Rate |
| --- | --- | --- | --- | ---: | ---: | ---: |
| SplitPush | D+ | Pick | C- | 22 | 384 | 58.33% |
| Siege | C | Pick | A- | 22 | 191 | 59.69% |
| Siege | D | TeamFight | C | 22 | 171 | 59.65% |
| Siege | C | Pick | A- | 21 | 189 | 59.26% |
| SplitPush | D+ | Control | B+ | 22 | 352 | 57.10% |

These are exactly the kind of rows that should be explained as correlation-first. A `SplitPush D+` team probably did not win because it splitpushed well. It likely won through some other strength while still matching that low-rated strategy row.

## Takeaways

Keep the model, but do not oversell raw score advantage.

Near-term refinement should focus on:

- validating primary strategy pairings rather than only same-axis score deltas,
- adding role-specific profile overrides,
- adding synergy and anti-synergy residual checks,
- testing patch-relative grade thresholds,
- using timeline evidence to confirm whether teams actually played through a strategy.

The important positive result is that we now have a way to falsify and tune the model. That is more valuable than pretending the first pass is already right.

## Follow-Up: Primary Matchup Signal

After this snapshot, the validation endpoint was tightened so `primaryMatchups` are ranked by Wilson-backed directional signal instead of mostly by sample size.

That matters because the broad additive checks above are intentionally blunt. They ask whether a higher score on one axis beats a lower enemy score on the same axis. The more important legacy question is narrower:

```text
Does this team's primary strategy reliably perform into that team's primary strategy?
```

For that reason, the next win-condition tuning pass should start with high-signal primary matchup rows, not global score-delta rows. A primary matchup with a Wilson interval clearing 50% is better evidence than a same-axis score bucket hovering around even.

Practical read:

- Use score deltas to audit whether the five additive scores have broad predictive value.
- Use primary matchup signal to discover strategy-pair edges worth explaining in the live UI.
- Do not change champion scores from one row alone; use high-signal rows as leads for role, synergy, and timeline checks.
