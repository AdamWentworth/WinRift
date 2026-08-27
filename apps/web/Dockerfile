FROM node:25-slim AS dependencies

WORKDIR /app
ENV NPM_CONFIG_AUDIT=false \
    NPM_CONFIG_FUND=false \
    NPM_CONFIG_UPDATE_NOTIFIER=false

COPY apps/web/package.json apps/web/package-lock.json* /app/
RUN npm ci --no-audit --no-fund

FROM dependencies AS development

COPY apps/web /app

EXPOSE 5173
CMD ["npm", "run", "dev"]

FROM dependencies AS build

COPY apps/web /app

ARG VITE_API_URL=""
ENV VITE_API_URL=${VITE_API_URL}
RUN npm run build

FROM golang:1.26.7 AS caddy-build

WORKDIR /src
COPY apps/web/caddy/go.mod /src/go.mod
COPY apps/web/caddy/go.sum /src/go.sum
COPY apps/web/caddy/healthcheck.go /src/healthcheck.go
RUN go mod download all \
    && CGO_ENABLED=0 GOOS=linux go build -mod=readonly -trimpath -ldflags="-s -w" \
        -o /out/caddy github.com/caddyserver/caddy/v2/cmd/caddy \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
        -o /out/healthcheck /src/healthcheck.go

FROM gcr.io/distroless/static-debian12:nonroot

ENV XDG_CONFIG_HOME=/tmp/caddy-config \
    XDG_DATA_HOME=/tmp/caddy-data

COPY apps/web/Caddyfile /etc/caddy/Caddyfile
COPY --from=build /app/dist /srv
COPY --from=caddy-build /out/caddy /usr/bin/caddy
COPY --from=caddy-build /out/healthcheck /healthcheck

EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=4 \
  CMD ["/healthcheck"]

ENTRYPOINT ["/usr/bin/caddy"]
CMD ["run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
