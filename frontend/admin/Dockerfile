# Прод-образ admin (React+Vite SPA): статическая сборка, отдаётся nginx.
# ВАЖНО: контекст сборки — КОРЕНЬ РЕПО (не frontend/admin), т.к. vite-alias @api-types указывает
# на ../../shared/api-types/ts (типы, сгенерированные из openapi.yaml). CI/compose собирают с
# context=. и file=frontend/admin/Dockerfile. Локально: `docker build -f frontend/admin/Dockerfile .`
# API-запросы админки идут на относительный /api/v1 — их проксирует ingress (Caddy) на api.
FROM node:20-alpine AS build
WORKDIR /app
# shared кладём на два уровня выше /app/src (alias ../../shared → /shared) — т.е. в /shared.
COPY shared/ /shared/
COPY frontend/admin/package.json frontend/admin/package-lock.json ./
RUN npm ci
COPY frontend/admin/ ./
RUN npm run build

FROM nginx:1.27-alpine AS run
COPY --from=build /app/dist /usr/share/nginx/html
COPY frontend/admin/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
