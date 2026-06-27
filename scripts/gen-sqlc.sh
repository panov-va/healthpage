#!/usr/bin/env bash
# Генерация store-кода из SQL-запросов через sqlc (backend/sqlc.yaml).
# Схему sqlc берёт из goose-миграций. Сгенерированный код — в internal/store/db (не править руками).
#
# Локально на свежем macOS SDK сборка sqlc через `go run` падает на cgo (pg_query_go).
# Поэтому предпочитаем бинарь sqlc на PATH; на CI (linux) `go run` работает.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/backend"

if command -v sqlc >/dev/null 2>&1; then
  sqlc generate
else
  echo "sqlc не найден на PATH — пробую 'go run' (может не собраться на macOS)."
  echo "Установка бинаря: https://docs.sqlc.dev/en/latest/overview/install.html"
  go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate
fi
echo "sqlc: done."
