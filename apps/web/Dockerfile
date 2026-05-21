FROM node:24-slim

WORKDIR /app
ENV NPM_CONFIG_AUDIT=false \
    NPM_CONFIG_FUND=false \
    NPM_CONFIG_UPDATE_NOTIFIER=false

COPY apps/web/package.json apps/web/package-lock.json* /app/
RUN npm ci --no-audit --no-fund

COPY apps/web /app

EXPOSE 5173

CMD ["npm", "run", "dev"]
