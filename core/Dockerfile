FROM golang:1.26.7 AS build

WORKDIR /src
COPY core/go.mod core/go.sum* ./
RUN go mod download

COPY core ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/winrift-api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/winrift-worker ./cmd/worker \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/winrift-monitor ./cmd/monitor \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/winrift-patchctl ./cmd/patchctl

FROM gcr.io/distroless/static-debian12

ARG WINRIFT_BUILD_SHA=development

WORKDIR /
LABEL org.opencontainers.image.source="https://github.com/AdamWentworth/WinRift" \
      org.opencontainers.image.revision="${WINRIFT_BUILD_SHA}"
COPY --from=build /out/winrift-api /winrift-api
COPY --from=build /out/winrift-worker /winrift-worker
COPY --from=build /out/winrift-monitor /winrift-monitor
COPY --from=build /out/winrift-patchctl /winrift-patchctl
COPY core/internal/clickhouse/schema.sql /usr/share/winrift/deployment/schema.sql
COPY ops/prod/champion-page-perf-audit.py /usr/share/winrift/deployment/champion-page-perf-audit.py
COPY ops/perf-smoke.sh /usr/share/winrift/deployment/perf-smoke.sh

EXPOSE 8000
CMD ["/winrift-api"]
