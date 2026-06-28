// Базовый префикс управляющего API (см. openapi.yaml / backend server.go).
// В dev запросы на "/api" проксируются vite на backend (vite.config.ts),
// в проде admin и api живут за одним gateway.
export const API_BASE = "/api/v1";

// Ключ хранения access-токена. Refresh-токен живёт в httpOnly-cookie (hp_refresh)
// и в JS недоступен — его ротацию делает backend на /auth/refresh.
export const ACCESS_TOKEN_KEY = "hp_access_token";
