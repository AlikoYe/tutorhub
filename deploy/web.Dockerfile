# syntax=docker/dockerfile:1
# Production web image: the built Vue app served by Caddy, which also proxies /api.
# Build context is the repo root; web.Dockerfile.dockerignore keeps .env (secrets and
# VITE_DEV_TELEGRAM_ID) out of it, so the production bundle never contains dev settings.

FROM node:22-alpine AS build
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM caddy:2-alpine
COPY deploy/Caddyfile /etc/caddy/Caddyfile
COPY --from=build /app/frontend/dist /srv
