# Legacy Win Condition Audit

The old win-condition system is worth preserving. It captures a real League idea: champions contribute differently to SplitPush, Pick, Siege, Control, and TeamFight plans, and the shape of the whole composition can influence which timings and macro plans are favorable.

This audit is based on the archived code under `legacy/` and the current Data Dragon roster observed from version `16.10.1`.

External roster reference:

- Data Dragon versions: https://ddragon.leagueoflegends.com/api/versions.json
- Data Dragon champion data for this audit: https://ddragon.leagueoflegends.com/cdn/16.10.1/data/en_US/champion.json

## Core Idea

Each champion has a hand-authored strategic profile:

```text
SplitPush + Pick + Siege + Control + TeamFight = 10
```

Example source: `legacy/data/champion_win_conditions.json`.

Each team profile is calculated by summing the five champion profiles. The team's main win condition is the highest aggregate axis. If multiple axes tie, the legacy Python path tries to break ties by counting how many champions have an `IndividualWinCondition` matching one of the tied axes.

The system then converts raw team-axis scores into letter ratings such as `D`, `B+`, or `S` and aggregates historical winrates for strategic matchups like:

```text
TeamFight B+ vs Pick C+
```

Those matchup aggregates also include game-duration buckets:

- `0-20`
- `20-25`
- `25-30`
- `30-35`
- `35+`

Important nuance: these buckets describe the duration of games that ended in that window. They are not true "win probability at minute 25" state snapshots. They are still useful, but we should label them as game-length outcomes unless we later derive true timeline/state metrics.

## Legacy Files

Champion profiles:

- `legacy/data/champion_win_conditions.json`

Score calculation:

- `legacy/data/modules/calculate_win_condition_scores.py`
- `legacy/server/src/services/calcWinConditionScoresService.js`

Rating scale:

- `legacy/data/modules/ratings.py`
- `legacy/server/src/services/winConditionRatingsService.js`

Match ingestion/storage:

- `legacy/data/main.py`
- `legacy/data/modules/sql_database_functions.py`

Generated aggregate data:

- `legacy/app/winrates_primary.json`
- `legacy/app/winrates_secondary.json`
- `legacy/app/winrates_backup.json`
- mirrored under `legacy/server/`

Legacy frontend display:

- `legacy/frontend/src/components/gameComponents/StatsCards.jsx`
- `legacy/frontend/src/components/gameComponents/statComponents/WinCondition.jsx`
- `legacy/frontend/src/components/gameComponents/statComponents/Alternatives.jsx`
- `legacy/frontend/src/components/gameComponents/statComponents/Enemy.jsx`
- `legacy/frontend/src/components/gameComponents/statComponents/Chart.jsx`

Visual assets:

- `legacy/frontend/public/images/win_condition_icons/`
- `legacy/frontend/public/images/win_condition_ratings/`

## Data Health Findings

The legacy champion profile file has 167 rows but 166 unique champion names.

Known issue:

- `Blitzcrank` appears twice.

Compared against Data Dragon `16.10.1`, the legacy profile file is missing:

- Ambessa
- Aurora
- Mel
- Smolder
- Yunara
- Zaahen

The profile file also contains 20 champions marked `IndividualWinCondition: Flex`:

- Akshan
- Aphelios
- Briar
- Draven
- Ekko
- Ezreal
- Gangplank
- Graves
- Hwei
- Jarvan IV
- K'Sante
- Kayn
- Lee Sin
- Lucian
- Neeko
- Ryze
- Senna
- Taliyah
- Yasuo
- Yone

`Flex` is not one of the five team scoring axes. That is probably fine as an individual label, but it needs a clear definition. In the legacy tie-breaker, `Flex` does not help resolve ties because only the five scoring axes are eligible team win conditions.

The five champion-axis scores all appear to sum to 10, which is a very good invariant to preserve.

Average individual axis weights in the legacy file:

- TeamFight: 2.26
- Control: 2.11
- Pick: 2.04
- SplitPush: 1.95
- Siege: 1.65

This suggests the profile set leans slightly toward TeamFight and away from Siege. That may be correct, but it is worth keeping visible when revising the file so scoring drift is intentional.

## What Was Good

The strategic axes are still valid.

- SplitPush captures side-lane pressure, dueling, structure pressure, and map stretching.
- Pick captures catch tools, burst windows, flank threat, and fog-of-war punishment.
- Siege captures ranged tower pressure, poke, waveclear, and objective setup from ahead.
- Control captures zone denial, terrain control, disengage, setup, and objective space.
- TeamFight captures front-to-back reliability, engage, wombo, scaling 5v5, and grouped execution.

The model is understandable. A user can look at a composition and immediately understand "this team wants to pick" or "this team is much better grouped."

The model is also complementary to build analytics. Build data answers "what items tend to win in this champion matchup." Win-condition data answers "what kind of game is this composition trying to create."

The legacy UI already had the right product instinct:

- Show your team identity.
- Show enemy identity.
- Show alternate viable identities.
- Show winrate by game-length bucket.

That shape is still strong.

## What Should Change

### Version The Profiles

Champion profiles should be patch-versioned. Champion kits, items, runes, and meta roles change.

Suggested table:

```text
champion_win_condition_profiles
- patch
- champion_id
- champion_name
- role nullable
- splitpush
- pick
- siege
- control
- teamfight
- individual_archetype
- source
- notes
- updated_at
```

Start with role-null profiles. Add role-specific overrides later for champions like Poppy, Gragas, Neeko, Pantheon, Karma, or others whose strategic identity can change by lane.

### Separate Human Profiles From Empirical Aggregates

The hand-authored profile is input data. Matchup winrates are derived data.

Suggested derived table:

```text
match_team_win_conditions
- match_id
- platform
- patch
- queue_id
- team_id
- rank_bucket
- splitpush_score
- pick_score
- siege_score
- control_score
- teamfight_score
- primary_condition
- primary_rating
- secondary_condition
- secondary_rating
```

Suggested aggregate table:

```text
win_condition_matchup_metrics
- patch
- queue_id
- rank_bucket
- team_condition
- team_rating
- opponent_condition
- opponent_rating
- game_length_bucket
- wins
- games
- win_rate
- confidence
```

This should be compiled per patch the same way build metrics are compiled.

### Normalize Ratings

The legacy rating scale assumes a max of 25:

- 0 = `D-`
- 1-2 = `D`
- 3-4 = `D+`
- ...
- 23 = `S-`
- 24 = `S`
- 25 = `S+`

However, individual champions can have more than 5 points in one axis, for example high Siege scores. A five-champion team could theoretically exceed 25 in an axis, which would become `N/A` in the old rating function.

Better options:

- Keep the letter scale but clamp at `S+`.
- Or convert scores to a normalized 0-100 percentile per patch.
- Or rate by empirical distribution so `A` means "top X percent of real teams this patch."

The third option is best once enough data exists.

### Keep Duration Buckets Honest

The old chart title was "Winrate Over Time." The data is actually "winrate for games ending in this duration bucket."

For the revived UI, label this as:

- `Winrate By Game Length`
- `Short Games`
- `20-25`
- `25-30`
- `35+`

Later, true power-spike analytics can use timeline frames to answer stateful questions like gold/XP/objective position at 10, 15, and 20 minutes.

### Add Confidence

The old frontend filtered aggregate results below 5 matches, but it did not show uncertainty.

The revived system should show:

- games
- wins
- raw winrate
- confidence or sample warning
- fallback tier when samples are thin

Use the same confidence approach already used for build analytics.

### Integrate With Roles And Rank Buckets

The old model was team-only and average-elo based. The new model should filter or group by:

- patch
- queue
- rank bucket
- platform/region where useful
- role-specific champion profile when available

## Suggested Revival Plan

1. Import the legacy champion profiles into a structured Go fixture or ClickHouse seed file.
2. Add validation tests:
   - every current champion has exactly one profile
   - no duplicate champion ids or names
   - five axis scores sum to 10
   - axis values are non-negative
   - `individual_archetype` is either one of the five axes or `Flex`
   - rating logic handles scores above legacy bounds
3. Add profiles for Ambessa, Aurora, Mel, Smolder, Yunara, and Zaahen.
4. Review changed/reworked champions and patch-sensitive outliers.
5. Add team-profile calculation to the Go analytics package.
6. Store match/team win-condition rows during ingestion.
7. Add patch-compiled win-condition matchup aggregates.
8. Add a live-match UI panel with team profile bars and matchup/game-length context.
9. Combine win-condition filters with build analytics after the base panel is useful.

## Product Direction

In the MVP, this should be contextual:

- "Blue is strongest at Pick and TeamFight."
- "Red has stronger Siege and Control."
- "Historically, Pick B+ into TeamFight A- has been stronger in shorter games."
- "Small sample: using all-rank 16.10 data."

Avoid direct calls like "force Baron now" or "split bot." WinRift should inform the player about composition shape and historical patterns, not command live gameplay.

## Bottom Line

The win-condition idea should not be thrown away. It should come back as a patch-versioned, validated, composition-level analytics layer that sits beside matchup build stats.

The right long-term framing is:

- Build analytics: "What tends to work for this champion into this opponent?"
- Win-condition analytics: "What kind of game does this team composition want?"
- Timeline analytics: "When does that plan tend to become strong or weak?"
