# Summoner Profiles

Summoner profiles are the non-live player surface. A Riot ID search should first check whether the player is currently in a live game; if yes, WinRift opens the live match room. If not, the app falls back to the stored profile view.

## Current Surface

The profile page currently shows:

- saved Riot ID alias and platform
- live-match redirect when Spectator-V5 finds an active game
- cached ranked Solo/Duo snapshot when rank enrichment has seen the player
- stored ranked Solo/Duo form from collected matches
- stored data window showing first and last retained game in the profile sample
- explicit freshness read so old stored samples are not mistaken for live form
- champion comfort rows with client-side champion-name filtering and role filters
- summoner-owned build paths from stored games
- recent stored matches with result, role, patch, KDA, and duration
- profile-specific background art using splash art for recently played champions

The page structure takes product-level inspiration from established profile tools: a high-signal profile summary first, then separate sections for overview, champion stats, and match history. WinRift should keep its own visual language and emphasize stored-sample transparency rather than copying another site's layout one-to-one.

## Backend Shape

The frontend should not aggregate a summoner's whole stored history on every request. The profile endpoint now prefers compact read models:

- `summoner_profile_summary`: one row per platform, queue, and PUUID
- `summoner_champion_summary`: one row per platform, queue, PUUID, and champion
- `summoner_champion_role_summary`: one row per platform, queue, PUUID, champion, and role

The profile endpoint also returns `topBuilds`, grouped directly from stored participant rows by champion, role, item signatures, rune signature, and spell signature. This is usage history for that summoner, not generalized build advice. It belongs on profile pages as "what this player tends to use"; matchup advice stays in live match and champion-guide surfaces.

The worker refreshes these summaries on a schedule. The API falls back to direct participant aggregation if the read model has not been populated yet, which keeps local development forgiving.

## Follow-Ups

- Add patch filters once the profile page needs historical comparison.
- Consider a small "collector queued" state when a live lookup nudges a player into the frontier.
- Add profile-level trends over time once we keep enough retained history.
- Consider exposing background-art scope as a reusable page policy: global random art by default, champion-specific art on champion pages, and recent champion art on summoner profiles.
