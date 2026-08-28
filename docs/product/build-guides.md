# Build Guides

WinRift build guides are patch-, role-, rank-, and matchup-aware views over the stored ranked corpus. They emphasize sample scope and fallback behavior instead of presenting one unexplained build as universally correct.

## Current Surface

A champion page can show:

- champion, role, patch, rank, and optional opponent filters;
- stored games, win rate, confidence, sample-relative pick/ban presence, and role rank;
- WinRift tier score and its win, sample, presence, and impact components;
- rune, summoner-spell, and timeline-derived skill-order aggregates;
- legal opening-purchase bundles;
- completed-item slot recommendations;
- favorable and difficult opponent rows;
- alternative build families derived from observed early core items.

The primary item panels distinguish opening purchases from completed items. Starting loadouts come from the first legal shop burst under the normal starting-gold cap; core and late slots use completed-item purchase order. Display logic removes duplicate boots and avoids repeating a core item in a late slot when a supported alternative exists.

## API Contracts

- `GET /api/analytics/champion-page` is the preferred champion-page contract. It bundles the guide, build advice, role hints, and guide index so the browser does not create a request waterfall.
- `GET /api/analytics/build-advice` supplies champion-wide and optional exact-matchup item, loadout, rune, spell, and build-path data. Live matchup panels use this focused contract.
- `GET /api/analytics/champion-guides` supplies the guide index and tier-list inputs for one role/patch/rank scope.
- `GET /api/analytics/champion-guide` supplies one focused champion and role guide.
- `GET /api/analytics/item-slots` supplies the lower-level item-slot read model for specialty views and diagnostics.

Development refresh routes can rebuild guide and item-slot models from retained data. Normal production requests never perform that work.

## Read Models and Cache

Guide responses are assembled from compact ClickHouse tables:

- `champion_role_analytics` resolves role distributions and defaults;
- `champion_guide_summary_analytics` stores champion performance and impact totals;
- `champion_guide_scope_analytics` stores the comparison population for a role/rank scope;
- `champion_matchup_analytics` stores opponent results;
- `champion_signature_analytics` stores rune, spell, skill, and build signatures;
- `champion_build_variant_analytics` stores alternative build families;
- `item_slot_analytics` and `starting_loadout_analytics` store item panels;
- `build_signature_analytics` supports build paths and archived fallback;
- `champion_page_bundle_cache` persists the complete frontend response across API restarts.

Worker startup prewarms every canonical champion for the current patch and every selectable archived patch. Scheduled refreshes rebuild changing retained-patch bundles; immutable archived bundles remain reusable. Common retained-patch opponent bundles are also warmed within a hard cap because exact matchup selection is the most noticeable cold path.

## Build Families

Alternative tabs come from WinRift's stored corpus, not a runtime scrape of another guide site. The classifier ignores boots, starter items, jungle pets, support quest items, consumables, and common components when identifying a family.

Labels are deliberately broad—such as AP, On Hit, Tank, Crit, Lethality, AD Bruiser, Enchanter, and Support Tank. Rows that resolve to the same player-facing identity are combined. The `Recommended` tab remains the highest-support broad setup; alternative tabs intentionally narrow to one observed family.

Recommended item choices use a support-aware score so tiny hot samples do not outrank stable, high-volume choices. Build-specific skill orders require a minimum retained sample; below that floor, the page uses the champion-level skill order.

## Scope and Limitations

- Pick and ban rates describe WinRift's stored sample, not Riot-wide global popularity.
- Tier score is a correlation-heavy internal ranking signal; it is not isolated champion power.
- First-three-item signatures use timeline purchase order where available. Final inventory signatures reflect the Riot payload's inventory order and are not guaranteed purchase order.
- Exact-matchup panels do not mix champion-wide fallback rows into the matchup card. Champion-wide baseline remains separately labeled.
- Thin matchup samples may widen across stored patches; the response reports the resolved scope and fallback notes.
- Slot panels remain useful when a complete build-path sample is too thin, but they should not be described as causal recommendations.

Champion pages are reference surfaces for exploring historical patterns. Live match views reuse the same evidence while keeping opponent context and uncertainty visible.
