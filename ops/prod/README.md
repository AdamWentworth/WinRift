# WinRift Production Ops

This folder contains the private server deployment shape for WinRift core services.

## What Runs

- `winrift_clickhouse`: persistent ClickHouse analytics store.
- `winrift_api`: private API bound to `127.0.0.1:8000` by default.
- `winrift_worker`: collector worker. It is started explicitly and uses `restart: "no"` so expired Riot keys do not loop forever.

The frontend is intentionally not deployed here yet. For now, develop it locally and point it at the server API through an SSH tunnel.

## Server Bootstrap

On the server:

```bash
sudo mkdir -p /srv/winrift
sudo chown -R $USER:$USER /srv/winrift
cp ops/prod/winrift.env.example /srv/winrift/.env
editor /srv/winrift/.env
```

Set the real Riot key, ClickHouse password, current patch, and platform list.

## Laptop Development Against Server API

```bash
ssh -N -L 8000:127.0.0.1:8000 your-server
cd apps/web
VITE_API_URL=http://127.0.0.1:8000 npm run dev
```

See [Deployment And Operations](../../docs/ops-deployment.md) for the full runbook.
