.PHONY: up up-worker stop-worker down stop status logs-worker logs-api test

up:
	docker compose up -d --build clickhouse api web

up-worker:
	docker compose --profile worker up -d --build clickhouse api worker

stop-worker:
	docker compose --profile worker stop worker

down:
	docker compose --profile worker down --remove-orphans

stop:
	docker compose --profile worker stop

status:
	docker compose --profile worker ps

logs-worker:
	docker compose logs -f worker

logs-api:
	docker compose logs -f api

test:
	cd services/core && go test ./...
