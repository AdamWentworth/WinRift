# Build Matchup Analytics Discussion

Build matchup analytics are the main product hook: instead of showing only generic champion builds, WinRift tries to answer what has worked for this champion into this opponent.

## What Feels Solid

- Champion plus opponent champion is the right center of gravity.
- Timeline item purchases are better than final inventory alone because they preserve first, second, third item order.
- Per-slot item stats are easier to explain than only full six-item signatures.
- Pooling broadly is correct while data is sparse. Filtering too early by patch, region, rank, and role creates tiny samples and fake certainty.
- Jungle and support should affect item eligibility, but not necessarily hard-filter the matchup data by role.

## Current Behavior

Live item advice currently filters by:

- champion id,
- opponent champion id,
- optional item context.

It does not default-filter by:

- patch,
- region/platform,
- rank bucket,
- exact role.

Item context is used for item pool eligibility:

- Smite players can include jungle starter/jungle item patterns.
- Support slot can include support item patterns.
- Normal contexts avoid jungle/support starter noise.

## Why This Is Better Than Generic Builds

Generic champion builds answer:

```text
What wins most often on this champion overall?
```

WinRift's intended question is:

```text
What has worked for this champion into this opposing champion?
```

That matters because matchup pressure changes build incentives. A defensive first item may be mediocre globally but valuable into a specific lane opponent. A damage rush may look good globally but fail into a matchup where surviving early pressure is the real problem.

## What Is Murky

Item winrate is not pure item impact.

High winrate can mean:

- the item is genuinely strong,
- strong players choose that item,
- the player was already ahead and could afford that item timing,
- the game state allowed the item path,
- the sample is too small.

First-item and second-item stats are useful, but they are not causal proof.

## What We Can Test

- Compare matchup-specific recommendations against generic champion recommendations.
- Track whether high-confidence matchup builds remain stable as samples grow.
- Compare first-item slot stats against completed-build signatures.
- Measure sample-size thresholds where item rankings stop changing wildly.
- Add patch-only views once per-matchup samples are dense enough.
- Evaluate whether rank-filtered advice differs meaningfully from all-rank advice.

## Refinement Ideas

### Fallback Ladder

The long-term query should probably widen scope until it finds enough data:

```text
1. champion + opponent + item context + current patch
2. champion + opponent + item context + current/previous patch
3. champion + opponent across all stored patches
4. champion + opponent class or role archetype
5. champion global item patterns
```

The UI should disclose the fallback scope.

### Confidence Language

Avoid over-selling math terms. "Confidence floor" is accurate but not friendly. Better UI language:

- "Thin"
- "Early"
- "Useful"
- "Strong"

The live build row currently treats the largest populated item-slot sample as the quick-read signal:

- `<5` games: thin
- `<15` games: early
- `<50` games: useful
- `50+` games: strong

This is deliberately simple and player-facing. The deeper Wilson-style confidence math still belongs in diagnostics and backend ranking, but the live card should not make players parse statistical language while loading into a match.

### Live Fallback Ladder

For live matchup cards, the item-slot endpoint now fills missing slots through a labeled fallback ladder:

1. Current patch exact champion matchup.
2. Exact champion matchup across all stored patches.
3. Current patch champion-wide item slot.
4. Champion-wide item slot across all stored patches.

The fallback is slot-by-slot. If slot one has exact matchup data but slot five does not, slot one stays exact while slot five can fall back. The UI labels mixed fallback samples instead of pretending the whole row came from one exact matchup.

### Read Model And Batch Loading

The live page should not ask ClickHouse to replay timeline item history while a player is waiting for a match read.

Current direction:

- The web app sends one batch request for all ten live build cards.
- The API reads `item_slot_analytics` first.
- The expensive timeline scan remains only as a safety fallback when the read model has not been populated yet.
- Refresh the read model after collection runs or patch changes with `POST /api/dev/analytics/item-slots/refresh` locally, or `patchctl -action item-slots -patch <patch> -queue 420` in ops scripts.

This keeps the player-facing response path simple: compact aggregate rows in, formatted build cards out.

### Causal Guardrails

Eventually use timing-aware features:

- first completed item timing,
- gold at purchase,
- opponent item timing,
- lane state proxies from CS/gold/xp at 10 and 15,
- team gold and objective state.

This could separate "this item wins" from "winning players bought this item."

## Current Position

The idea is strong and differentiated. The main risk is sample size and causal interpretation, not the data model. Keep broad pooling for now, then add fallbacks and clearer sample language as the dataset grows.
