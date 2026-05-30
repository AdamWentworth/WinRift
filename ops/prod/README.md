# WinRift Production Ops

This folder contains the private server deployment shape for WinRift core services.

## What Runs

- `winrift_clickhouse`: persistent ClickHouse analytics store.
- `winrift_api`: private-LAN API bound to `0.0.0.0:8000` by default.
- `winrift_worker`: collector worker. It is started explicitly and uses `restart: "no"` so expired Riot keys do not loop forever.

The frontend is intentionally not deployed here yet. For now, develop it locally and point it at the server API over the private LAN.

## Server Bootstrap

On the server:

```bash
sudo mkdir -p /srv/winrift
sudo chown -R $USER:$USER /srv/winrift
findmnt /mnt/storage
sudo mkdir -p /mnt/storage/clickhouse/{data,logs,backups}
sudo chown -R $USER:$USER /mnt/storage/clickhouse
cp ops/prod/winrift.env.example /srv/winrift/.env
editor /srv/winrift/.env
```

Set the real Riot key, ClickHouse password, current patch, and platform list.
Leave ClickHouse paths pointed at `/mnt/storage/clickhouse/...` so database files land on the second SSD, not the OS disk.

For the one-time laptop database copy, follow [data-migration.md](data-migration.md).

## Laptop Development Against Server API

```bash
cd apps/web
VITE_API_URL=http://SERVER_LAN_IP:8000 npm run dev
```

See [Deployment And Operations](../../docs/ops-deployment.md) for the full runbook.

## Archive An Old Patch

After a Riot patch rolls over and the collector window moves on, archive the old patch before pruning:

```bash
cd /srv/winrift
docker compose --env-file /srv/winrift/.env run --rm api \
  /winrift-patchctl -action archive -patch 16.9 -platform ALL -queue 420 -retain-days 0
```

This keeps app summaries and the small normalized lookup index, while deleting bulky raw/timeline payloads.
