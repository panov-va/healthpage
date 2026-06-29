# Сборка api-сервиса HealthPage.
# Многостадийная: компилируем статический бинарь, кладём в минимальный образ.
FROM golang:1.25 AS build
WORKDIR /src

# Кэшируем зависимости отдельно от исходников.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 go build -o /out/queue-setup ./cmd/queue-setup
RUN CGO_ENABLED=0 go build -o /out/worker-email ./cmd/worker-email
RUN CGO_ENABLED=0 go build -o /out/worker-telegram ./cmd/worker-telegram

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /out/migrate /app/migrate
COPY --from=build /out/queue-setup /app/queue-setup
COPY --from=build /out/worker-email /app/worker-email
COPY --from=build /out/worker-telegram /app/worker-telegram
COPY --from=build /src/migrations /app/migrations
EXPOSE 8080
ENTRYPOINT ["/app/api"]
