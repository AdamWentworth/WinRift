# Riot API Notes

## Identity

Use Riot ID as the player-facing input:

```text
gameName#tagLine
```

Flow:

1. Account-V1 by Riot ID returns PUUID.
2. Summoner-V4 by PUUID returns platform-specific summoner data when needed.

Do not build new features around summoner names. Riot has marked summoner-name usage as deprecated and not player-facing.

## Match Collection

MVP scope:

- Platform: `NA1`
- Regional route: `AMERICAS`
- Queue: `420`
- Map: `11`
- Game mode: `CLASSIC`

Collector endpoints:

- Match IDs: `/lol/match/v5/matches/by-puuid/{puuid}/ids`
- Match: `/lol/match/v5/matches/{matchId}`
- Timeline: `/lol/match/v5/matches/{matchId}/timeline`

## Rank Enrichment

Match-V5 does not include ranked tier. Rank enrichment is cached separately through League-V4:

- League entries: `/lol/league/v4/entries/by-puuid/{puuid}`

Only `RANKED_SOLO_5x5` entries feed the MVP rank bucket. Rank enrichment has its own request budget and TTL so collection does not query rank for every player in every match.

## Live Context

Live lookup uses Spectator-V5:

```text
/lol/spectator/v5/active-games/by-summoner/{puuid}
```

The public UI only shows aggregate contextual stats. It must not present hidden game-session-specific information as a directive.

## Rate Limits

The core service:

- Sends `X-Riot-Token` from environment.
- Treats 401 and 403 as Riot API key auth failures. The worker stops hard; the API stays up and returns `503 RIOT_API_KEY_UNAVAILABLE` for Riot-dependent endpoints while cached/static/analytics endpoints remain available.
- Treats 404 as ordinary absence, such as an unknown Riot ID or a player not currently being in a live game.
- Honors 429 `Retry-After` once before failing.
- Does not log the API key.
