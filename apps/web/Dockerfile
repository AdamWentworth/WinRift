FROM node:24-slim

WORKDIR /app

COPY apps/web/package.json apps/web/package-lock.json* /app/
RUN npm ci

COPY apps/web /app

EXPOSE 5173

CMD ["npm", "run", "dev"]
