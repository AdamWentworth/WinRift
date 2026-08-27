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

FROM caddy:2.11-alpine

COPY apps/web/Caddyfile /etc/caddy/Caddyfile
COPY --from=build /app/dist /srv

EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=4 \
  CMD wget --spider -q http://127.0.0.1:8080/healthz || exit 1
