.PHONY: up up-worker up-monitor stop-worker stop-monitor down stop status logs-worker logs-api logs-monitor test

up:
	docker compose up -d --build clickhouse api web

up-worker:
	docker compose --profile worker up -d --build clickhouse api worker

up-monitor:
	docker compose --profile monitor up -d --build clickhouse api monitor

stop-worker:
	docker compose --profile worker stop worker

stop-monitor:
	docker compose --profile monitor stop monitor

down:
	docker compose --profile worker --profile monitor down --remove-orphans

stop:
	docker compose --profile worker --profile monitor stop

status:
	docker compose --profile worker --profile monitor ps

logs-worker:
	docker compose logs -f worker

logs-api:
	docker compose logs -f api

logs-monitor:
	docker compose logs -f monitor

test:
	cd core && go test ./...
