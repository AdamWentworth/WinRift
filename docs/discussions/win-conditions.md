# Win Conditions Discussion

The win-condition system is one of WinRift's strongest ideas, but it should be treated as a strategic model that can be tested and refined, not as a solved truth.

The legacy model is documented in [Legacy Win Condition Audit](../legacy-win-condition-audit.md). This note focuses on how to reason about the model going forward.

## Building Block 1: The Five Axes

The current axes are:

- SplitPush
- Pick
- Siege
- Control
- TeamFight

### What Is Sound

These axes map well to durable League concepts. They describe how teams create pressure and convert advantages. They are also understandable to players.

SplitPush, Pick, Siege, Control, and TeamFight are broad enough to cover most composition identities without becoming a huge taxonomy.

### What Requires Research

- Whether "Control" is too broad and overlaps too much with Siege and TeamFight.
- Whether "Skirmish" deserves its own axis or is already captured by Pick/TeamFight.
- Whether "Scaling" should be represented directly or inferred from time-bucket outcomes.

### Leap Of Faith

The model assumes five axes are enough. That is a design judgment, not a fact from Riot data.

### What We Can Test

- Whether each axis produces distinct empirical matchup behavior.
- Whether high-rated teams in one axis actually cluster around different game lengths, objective patterns, or winrates.
- Whether removing or splitting an axis improves predictive performance.

### Improvement Path

Keep the five axes for now. They are coherent and preserve the legacy product identity. Use empirical analysis later to decide whether an axis should split, merge, or be redefined.

## Building Block 2: Champion Scores Sum To 10

Each champion receives five scores that sum to 10.

### What Is Sound

The sum-to-10 rule is very strong as a modeling constraint. It forces relative identity instead of letting every champion be good at everything. It also makes profiles easy to review.

For example, giving a champion more TeamFight weight must take weight away from SplitPush, Pick, Siege, or Control.

### What Requires Research

- Whether every champion should have exactly the same total strategic "budget."
- Whether some champions are genuinely broader or narrower than others.
- Whether role-specific profiles should use the same 10-point budget.

### Leap Of Faith

The model assumes champion identity is primarily relative, not absolute. That is probably healthy for UX, but it may understate champions who are strong in many strategic modes or weak in many strategic modes.

### What We Can Test

- Compare hand-authored scores against observed role, build, and game-length outcomes.
- Check whether champions with flatter distributions behave like flexible picks in match data.
- Check whether champions with spiky distributions produce clearer primary-condition outcomes.

### Improvement Path

Keep sum-to-10. Add metadata around confidence and review status for each champion profile. Later, test whether some champions need role-specific overrides rather than changing the global scoring rule.

## Building Block 3: Individual Champion Archetype

Each champion has an `individualWinCondition`, including `Flex` for champions that do not cleanly belong to one axis.

### What Is Sound

This is useful for tie-breaking and human explanation. It lets the system say not only "this team has 16 Pick points," but also "this composition contains multiple champions whose primary identity is Pick."

### What Requires Research

- The exact definition of `Flex`.
- Whether `Flex` should influence tie-breaking differently.
- Whether individual archetype should be role-specific.

### Leap Of Faith

Using one label per champion can flatten nuance. Hwei, Jayce, Varus, Karma, Pantheon, Poppy, and others can change strategic identity depending on role, build, and team context.

### What We Can Test

- How often team primary scores tie.
- Whether tie-breaks based on individual archetype produce more stable empirical outcomes than deterministic axis ordering.
- Whether Flex-heavy teams behave differently from teams with a clear primary identity.

### Improvement Path

Define `Flex` clearly. Consider role-specific archetype overrides before replacing the system.

## Building Block 4: Adding Champion Scores Into Team Scores

The team score is currently the sum of five champion profiles.

### What Is Sound

Addition is simple, explainable, deterministic, and cheap. In extreme cases, it captures reality well. Five champions with engage, AoE, and front-to-back tools should rate highly for TeamFight. A comp with several side-lane duelists and weak grouping should rate highly for SplitPush.

### What Requires Research

- Synergy effects.
- Anti-synergy effects.
- Role weighting.
- Whether a team needs at least one enabling component for an axis to be real.

### Leap Of Faith

Addition assumes champion contributions are independent and linear. League is not fully linear. Orianna plus Jarvan can be more than two separate TeamFight scores. A poke siege comp without disengage may be less functional than its Siege score suggests. A splitpush comp without waveclear or vision control may not actually execute.

### What We Can Test

- Compare additive team scores against winrates and game-length outcomes.
- Look for champion-pair residuals: pairs that consistently outperform or underperform what additive scores predict.
- Test role-weighted sums, such as support/control having more objective setup influence or top/splitpush carrying more side-lane influence.
- Test whether primary condition margin matters. A team with TeamFight 17 and Pick 16 may be less "TeamFight" than a team with TeamFight 22 and next-best 11.

### Improvement Path

Keep additive scores as version one. Add derived features:

- primary margin,
- top two axes,
- composition sharpness,
- synergy bonuses,
- role-specific weighting.

Do not jump straight to opaque ML until the simple model's strengths and failures are measured.

## Building Block 5: Letter Grades

Raw team-axis scores are converted into grades like `C+`, `B`, `A-`, or `S`.

### What Is Sound

Grades are good UI. They are compact and familiar. They let a player quickly see "this team is excellent at TeamFight but mediocre at Siege."

### What Requires Research

The current grading thresholds are the murkiest part of the model.

Static thresholds assume score meaning is stable across patches and across the champion roster. That might be fine enough, but it may also distort the middle of the distribution.

### Leap Of Faith

An `A` grade currently means "the additive score crossed this hand-authored threshold." It does not necessarily mean "top-tier among real teams this patch."

### What We Can Test

- Distribution of real team scores per patch.
- Percentile rank of each grade.
- Whether grades are balanced or most teams cluster around a few letters.
- Whether grade deltas correlate with winrate deltas.

### Improvement Path

Consider moving from static thresholds to patch distribution grades:

```text
S  = top 2% of observed team scores for that axis
A  = next 10%
B  = above-average cluster
C  = average cluster
D  = below-average cluster
```

We can still display familiar letters, but define them by observed patch distribution. This would make `A TeamFight` mean "this is actually high TeamFight for this patch's real compositions."

The simplest near-term improvement is to keep raw score plus letter grade visible in debugging docs and add distribution checks.

### Current Diagnostic Hook

The API exposes a diagnostics report so distributions can be checked against collected data:

```bash
curl 'http://localhost:8000/api/analytics/win-conditions/diagnostics?queueId=420&patch=16.10' | jq
```

The report includes:

- axis/rating distributions,
- primary condition distributions,
- primary margin buckets,
- total teams and matches included.

The API also exposes a validation report for direct outcome checks:

```bash
curl 'http://localhost:8000/api/analytics/win-conditions/validation?queueId=420&patch=16.10&minGames=50' | jq
```

That report compares ratings, score deltas, primary strategy matchups, primary-margin buckets, and suspicious low-rated/high-winrate rows against real win/loss outcomes. See [Win Condition Validation](../product/win-condition-validation.md).

Diagnostics should be the first place to look before changing the grading scale. Validation should be the first place to look before claiming the model has predictive signal.

## Building Block 6: Primary And Alternative Conditions

The primary condition is the highest-rated axis. Alternative conditions are the other axes.

### What Is Sound

This is very useful. Teams often have a main identity and fallback patterns. The primary-vs-alternative distinction also explains why `Pick B- vs Pick B-` is not automatically 50/50: one row can mean primary Pick into alternative Pick.

### What Requires Research

- How close an alternative score must be before it is strategically meaningful.
- Whether alternatives below a threshold should be hidden.
- Whether the primary condition should require a minimum margin over second place.

### Leap Of Faith

The current system treats every axis as queryable, even if the team is barely built for it. That may make the middle panel feel busy or murky.

### What We Can Test

- Primary margin distribution.
- Winrate stability for primary-primary rows versus alternative rows.
- Whether low-rated alternatives produce meaningful stats or mostly noise.

### Improvement Path

Add labels like:

- Primary
- Co-primary
- Strong secondary
- Secondary
- Weak angle

This keeps the old system but makes the UI more honest.

### Current UI Pass

The analyzer now annotates each axis with plan context:

- `Primary`: the selected highest team identity after tie-breaks.
- `Co-primary`: tied with the primary score, but not selected by the tie-breaker.
- `Strong secondary`: within two points of the primary score.
- `Secondary`: a plausible fallback based on score strength or closeness.
- `Weak angle`: present in the profile, but not a plan the comp is strongly built around.

Team profiles also expose primary margin and identity sharpness:

- `Tied identity`
- `Contested identity`
- `Flexible identity`
- `Clear identity`
- `Sharp identity`

This does not change the underlying scores or the legacy-style primary-vs-alternative metric rows. It changes how we explain them, which is the safer first step.

If a weak or low-rated angle shows a high historical winrate, the match read now warns that this is likely correlation rather than causation. For example, `SplitPush C-` with a high winrate into `Control B` should not imply those teams won by splitpushing well. More likely, those games were won through the composition's stronger plans while also matching the low SplitPush rating row.

The live UI now filters alternatives by plan fit. It still asks the backend for the full matrix, but the player-facing condition buttons hide weak angles by default. The default display shows:

- `Primary`
- `Co-primary`
- `Strong secondary`
- `Secondary` only when the plan is close enough to the primary score or has at least a `B-` rating

This keeps the middle panel focused on strategies the composition can plausibly execute. Weak-angle rows remain available in the API response for diagnostics, future advanced views, and model validation.

## Building Block 7: Winrates By Game Length

The legacy system uses duration buckets:

- `0-20`
- `20-25`
- `25-30`
- `30-35`
- `35+`

### What Is Sound

Game-length outcomes are useful. They can show whether a strategic pairing wins more in short games or long games.

### What Requires Research

Game duration is not the same as live win probability at that minute. A win in the `25-30` bucket means the game ended then, not that the team was favored at minute 25.

### Leap Of Faith

The chart can tempt users to read duration buckets as power spikes. That is close to the original dream, but not yet technically precise.

### What We Can Test

- Whether certain condition pairings consistently skew short or long.
- Whether timeline state at 10/15/20 minutes explains the duration bucket.
- Whether adding gold/xp/objective state changes the story.

### Improvement Path

Keep the current chart labeled as "Winrate By Game Length." Later add true timeline analytics:

- gold difference by minute,
- objective ownership,
- item completion timing,
- level curves,
- win probability from game state snapshots.

The live UI chart uses a focused 35-65% y-axis centered on 50%. This makes practical edges easier to see while still labeling the panel as duration-bucket results, not true minute-by-minute power-spike prediction.

## Building Block 8: ML And Statistical Refinement

ML could help, but only after the simple model is well-instrumented.

### Good ML Uses

- Learn role-specific champion profile adjustments.
- Detect champion-pair synergy residuals.
- Suggest profile score changes after a patch.
- Learn better grade thresholds from observed distributions.
- Predict game-length bucket from composition scores and matchup context.

### Risky ML Uses

- Replacing the five-axis model with opaque embeddings too early.
- Producing recommendations that cannot be explained.
- Training on sparse or biased collection data and treating it as universal truth.

### Best Path

Use ML as a critic and assistant for the hand-authored model, not as a replacement at first.

For example:

```text
Human model says: Team A has TeamFight A and Pick B.
Data says: similar teams overperform when they include Orianna + Jarvan.
System response: add a synergy feature or flag profile review, not discard the model.
```

## Current Position

The concept is sound. The murky parts are not the five axes or sum-to-10 champion profiles; those are actually strong constraints. The murky parts are:

- static grading thresholds,
- linear additive team scores,
- primary/alternative display rules,
- lack of synergy and role-specific overrides,
- duration buckets being adjacent to, but not yet equal to, power-spike modeling.

The next best work is not to tear it down. It is to instrument it, document the assumptions, and test each layer.

## Evidence Score

Winrate alone is not persuasive enough.

A `53%` result over thousands of games is often more meaningful than a `68%` result over 19 games. The UI now separates:

- raw winrate,
- games,
- Wilson confidence interval,
- evidence level,
- evidence direction.

Evidence direction can be:

- `favorable`,
- `unfavorable`,
- `neutral`,
- `mixed`,
- `unknown`.

Evidence level can be:

- `No sample`,
- `Thin`,
- `Early`,
- `Moderate`,
- `Strong`,
- `Very strong`.

This is intentionally not the same as advantage. A stable `47%` can be strong evidence too; it is just strong unfavorable evidence. A tiny `70%` can still be thin evidence.

The score is currently based on sample size plus Wilson interval stability. If the interval still overlaps even outcomes, the score is capped so the UI cannot over-sell noisy data.

The live match read now shows the evidence level, a 0-100 evidence score, games, and likely winrate range next to the headline. This keeps the prose grounded: a good-sounding read with thin samples should feel tentative, while a modest edge with thousands of games can feel more credible.

This is the right direction, but it should be revisited after we inspect real diagnostics output.
