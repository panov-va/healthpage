import { api } from "@/shared/api";
import type { AuthResult, AuthUser, LoginRequest, RegisterRequest } from "@/shared/api";

// Запросы аутентификации. /auth/* не требуют Bearer (auth: false), refresh-токен
// сервер кладёт в httpOnly-cookie сам.
export function register(body: RegisterRequest): Promise<AuthResult> {
  return api.post<AuthResult>("/auth/register", body, { auth: false });
}

export function login(body: LoginRequest): Promise<AuthResult> {
  return api.post<AuthResult>("/auth/login", body, { auth: false });
}

export function logout(): Promise<void> {
  return api.post<void>("/auth/logout", {}, { auth: false });
}

export function fetchMe(): Promise<AuthUser> {
  return api.get<AuthUser>("/auth/me");
}
