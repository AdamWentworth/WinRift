FROM golang:1.26.4 AS build

WORKDIR /src
COPY core/go.mod core/go.sum* ./
RUN go mod download

COPY core ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/winrift-api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/winrift-worker ./cmd/worker \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/winrift-monitor ./cmd/monitor \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/winrift-patchctl ./cmd/patchctl

FROM gcr.io/distroless/static-debian12

WORKDIR /
COPY --from=build /out/winrift-api /winrift-api
COPY --from=build /out/winrift-worker /winrift-worker
COPY --from=build /out/winrift-monitor /winrift-monitor
COPY --from=build /out/winrift-patchctl /winrift-patchctl

EXPOSE 8000
CMD ["/winrift-api"]
