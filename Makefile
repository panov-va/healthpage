# HealthPage — корневой Makefile. Цели сгруппированы: backend, frontend, infra, codegen.
.DEFAULT_GOAL := help
.PHONY: help up down build test lint fmt migrate-up migrate-down migrate-status \
        front-build gen gen-go gen-ts

BACKEND := backend

help: ## Показать список целей
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

## --- Infra (docker-compose) ---
up: ## Поднять dev-стек (postgres, redis, rabbitmq, api)
	docker compose up -d --build

down: ## Остановить dev-стек
	docker compose down

## --- Backend ---
build: ## Собрать все Go-пакеты
	cd $(BACKEND) && go build ./...

test: ## Прогнать Go-тесты
	cd $(BACKEND) && go test ./...

lint: ## Запустить golangci-lint
	cd $(BACKEND) && golangci-lint run ./...

fmt: ## Отформатировать Go-код
	cd $(BACKEND) && gofmt -w . && goimports -w -local github.com/healthpage/backend .

## --- Миграции (goose) ---
migrate-up: ## Применить все миграции (нужен DATABASE_URL)
	cd $(BACKEND) && go run ./cmd/migrate up

migrate-down: ## Откатить одну миграцию
	cd $(BACKEND) && go run ./cmd/migrate down

migrate-status: ## Показать статус миграций
	cd $(BACKEND) && go run ./cmd/migrate status

## --- Frontend ---
front-build: ## Собрать оба фронта
	cd frontend/admin && npm ci && npm run build
	cd frontend/public-ssr && npm ci && npm run build

## --- Кодогенерация типов из openapi.yaml ---
gen: gen-go gen-ts gen-sqlc ## Сгенерировать типы (openapi) и store-код (sqlc)

gen-go: ## Go-типы из openapi.yaml -> shared/api-types/go
	bash scripts/gen-go-types.sh

gen-ts: ## TS-типы из openapi.yaml -> shared/api-types/ts
	bash scripts/gen-ts-types.sh

gen-sqlc: ## Store-код из SQL-запросов -> backend/internal/store/db
	bash scripts/gen-sqlc.sh
