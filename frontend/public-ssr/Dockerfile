# Прод-образ public-ssr (Next.js SSR: лендинг + публичные статус-страницы).
# Многостадийная сборка со standalone-выводом Next — тонкий рантайм без dev-зависимостей.
FROM node:20-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-alpine AS run
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
# standalone-вывод: сервер + минимальные node_modules; отдельно статика и public.
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
COPY --from=build /app/public ./public
EXPOSE 3000
# HEALTHPAGE_API_URL задаётся окружением (внутренний адрес API в сети compose).
CMD ["node", "server.js"]
