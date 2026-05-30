# Initial ClickHouse Data Migration

Use this once to move the laptop ClickHouse dataset to the server SSD before starting production collection.

## 1. Stop Local Writes

On the laptop:

```bash
cd /home/adam/Projects/WinRift
make down
docker compose --profile worker ps
```

The `ps` output should be empty. The worker must not be writing while the archive is created.

## 2. Create A Cold Archive

```bash
mkdir -p /home/adam/Backups/winrift
docker run --rm \
  -v winrift_clickhouse_data:/var/lib/clickhouse:ro \
  -v /home/adam/Backups/winrift:/backup \
  alpine sh -c 'tar -czf /backup/winrift-clickhouse-$(date +%F-%H%M).tgz -C /var/lib/clickhouse .'
```

Keep this archive until the server restore has been verified and collection is healthy.

## 3. Copy The Archive To The Server

```bash
scp /home/adam/Backups/winrift/winrift-clickhouse-*.tgz adam@SERVER_LAN_IP:/srv/winrift/
```

## 4. Prepare Server Directories

On the server:

```bash
findmnt /mnt/storage
df -hT /mnt/storage
mkdir -p /mnt/storage/clickhouse/{data,logs,backups}
mkdir -p /srv/winrift
```

Do not start the worker yet.

## 5. Restore Into The Data SSD

Only run this when `/mnt/storage/clickhouse/data` is intended to be empty or disposable:

```bash
docker rm -f winrift_clickhouse winrift_api winrift_worker 2>/dev/null || true
rm -rf /mnt/storage/clickhouse/data/*
tar -xzf /srv/winrift/winrift-clickhouse-*.tgz -C /mnt/storage/clickhouse/data
```

## 6. Deploy Without Starting Worker

Run the GitHub deploy workflow with:

```text
start_worker=false
```

That should start ClickHouse and the API only.

## 7. Verify Counts

On the server:

```bash
set -a
source /srv/winrift/.env
set +a

docker exec -it winrift_clickhouse clickhouse-client \
  --user winrift \
  --password "$CLICKHOUSE_PASSWORD" \
  --database winrift \
  --query "SELECT count() FROM raw_matches"

docker exec -it winrift_clickhouse clickhouse-client \
  --user winrift \
  --password "$CLICKHOUSE_PASSWORD" \
  --database winrift \
  --query "SELECT patch, count() FROM raw_matches GROUP BY patch ORDER BY patch"
```

Expected laptop reference before migration:

```text
raw_matches: 41907
16.10: 31630
16.9: 10277
```

If those counts look right, run the deploy workflow again with `start_worker=true`.
