# Collector Runbook

## Start Infrastructure

```bash
docker compose up --build clickhouse api
```

## Seed From API

Call the dev-only endpoint while `ENVIRONMENT=development`:

```bash
curl -X POST http://localhost:8000/api/dev/collector/seed \
  -H "Content-Type: application/json" \
  -d '{
    "riotIds": [{"gameName": "Example", "tagLine": "NA1", "platform": "NA1"}],
    "matchCount": 20
  }'
```

## Seed Worker

Set one or both in `.env`:

```text
COLLECTOR_SEED_RIOT_IDS=Example#NA1
COLLECTOR_SEED_PUUIDS=
```

Then run:

```bash
docker compose --profile worker up --build worker
```

The worker sleeps between passes using `COLLECTOR_INTERVAL_SECONDS`.

## Safety

Keep match counts low on a development key. The client honors Riot 429 `Retry-After`, but large crawl seeds can still consume a daily development-key window quickly.
