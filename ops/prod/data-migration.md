# Initial ClickHouse Data Migration

Use this once to move the laptop ClickHouse dataset to the server SSD before starting production collection.

Prefer the direct LAN stream below. It avoids creating a huge temporary archive on either machine and preserves ClickHouse file ownership by extracting inside a Docker container on the server.

## 1. Stop Local Writes

On the laptop:

```bash
cd /home/adam/Projects/WinRift
make down
docker compose --profile worker ps
```

The `ps` output should be empty. The worker must not be writing while the archive is created.

## 2. Confirm SSH Access

```bash
ssh adam@192.168.1.77 'hostname && findmnt /mnt/storage && df -hT /mnt/storage'
```

If SSH key auth is not enabled, add the laptop's public key to `/home/adam/.ssh/authorized_keys` on the server.

## 3. Prepare Server Directories

```bash
ssh adam@192.168.1.77 '
  set -e
  findmnt /mnt/storage
  mkdir -p /mnt/storage/clickhouse/{data,logs,backups}
  mkdir -p /srv/winrift
  docker rm -f winrift_clickhouse winrift_api winrift_worker 2>/dev/null || true
  docker run --rm -v /mnt/storage/clickhouse/data:/restore busybox \
    find /restore -mindepth 1 -maxdepth 1 -exec rm -rf {} +
'
```

Do not start the worker yet.

## 4. Stream The Volume Over LAN

```bash
docker run --rm -v winrift_clickhouse_data:/data:ro busybox \
  tar -cf - -C /data . \
  | ssh adam@192.168.1.77 \
      'docker run --rm -i -v /mnt/storage/clickhouse/data:/restore busybox tar -xf - -C /restore'
```

This will take a while. If it is interrupted, repeat steps 3 and 4 so the server restore starts from a clean target directory.

## 5. Deploy Without Starting Worker

Run the GitHub deploy workflow with:

```text
start_worker=false
```

That should start ClickHouse and the API only.

## 6. Verify Counts

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

## Archive Fallback

If direct streaming is not practical, use an external drive or NAS for the archive. Avoid writing the archive to the laptop root disk unless there is clearly enough free space.

```bash
mkdir -p /path/to/large-disk/winrift
docker run --rm -v winrift_clickhouse_data:/data:ro busybox tar -cf - -C /data . \
  | pigz -1 > /path/to/large-disk/winrift/winrift-clickhouse.tgz
sha256sum /path/to/large-disk/winrift/winrift-clickhouse.tgz > /path/to/large-disk/winrift/winrift-clickhouse.tgz.sha256
```

Then copy the archive to the server or NAS and extract it into `/mnt/storage/clickhouse/data` before deploying.
