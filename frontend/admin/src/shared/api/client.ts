import { ACCESS_TOKEN_KEY, API_BASE } from "@/shared/config";

// Ошибка запроса с кодом/сообщением из контрактного {error:{code,message}}.
export class HttpError extends Error {
  readonly status: number;
  readonly code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "HttpError";
    this.status = status;
    this.code = code;
  }
}

// ── Хранилище access-токена ──────────────────────────────────────────────
// Access-токен дублируем в localStorage (переживает перезагрузку) и держим в
// памяти. Refresh-токен — в httpOnly-cookie, JS его не трогает.
let accessToken: string | null = readStoredToken();

function readStoredToken(): string | null {
  try {
    return localStorage.getItem(ACCESS_TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setAccessToken(token: string | null): void {
  accessToken = token;
  try {
    if (token) localStorage.setItem(ACCESS_TOKEN_KEY, token);
    else localStorage.removeItem(ACCESS_TOKEN_KEY);
  } catch {
    /* приватный режим/недоступный storage — игнорируем */
  }
}

export function getAccessToken(): string | null {
  return accessToken;
}

// ── Базовый запрос ────────────────────────────────────────────────────────
interface RequestOptions {
  method?: string;
  body?: unknown;
  // Не пытаться обновить токен и не слать Authorization (для /auth/*).
  auth?: boolean;
}

// При 401 пробуем один раз обновить access-токен через refresh-cookie.
// Защищаемся от параллельных рефрешей одним общим промисом.
let refreshing: Promise<boolean> | null = null;

async function refreshAccessToken(): Promise<boolean> {
  if (!refreshing) {
    refreshing = (async () => {
      const res = await fetch(`${API_BASE}/auth/refresh`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: "{}",
      });
      if (!res.ok) {
        setAccessToken(null);
        return false;
      }
      const data = (await res.json()) as { access_token?: string };
      if (!data.access_token) return false;
      setAccessToken(data.access_token);
      return true;
    })().finally(() => {
      refreshing = null;
    });
  }
  return refreshing;
}

async function rawFetch(path: string, opts: RequestOptions): Promise<Response> {
  const headers: Record<string, string> = {};
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  if (opts.auth !== false && accessToken) {
    headers["Authorization"] = `Bearer ${accessToken}`;
  }
  return fetch(`${API_BASE}${path}`, {
    method: opts.method ?? "GET",
    credentials: "include",
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  });
}

async function parseError(res: Response): Promise<HttpError> {
  let code = "error";
  let message = res.statusText || "Ошибка запроса";
  try {
    const data = (await res.json()) as { error?: { code?: string; message?: string } };
    if (data.error) {
      code = data.error.code ?? code;
      message = data.error.message ?? message;
    }
  } catch {
    /* тело не JSON — оставляем дефолт */
  }
  return new HttpError(res.status, code, message);
}

// request — основной метод. Возвращает распарсенный JSON (или undefined для 204).
export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  let res = await rawFetch(path, opts);

  // Прозрачное обновление access-токена при 401 (один раз, только для
  // авторизованных запросов).
  if (res.status === 401 && opts.auth !== false && accessToken) {
    const ok = await refreshAccessToken();
    if (ok) res = await rawFetch(path, opts);
  }

  if (!res.ok) throw await parseError(res);
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown, opts?: RequestOptions) =>
    request<T>(path, { ...opts, method: "POST", body }),
  patch: <T>(path: string, body?: unknown) => request<T>(path, { method: "PATCH", body }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};
