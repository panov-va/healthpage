import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { getAccessToken, setAccessToken } from "@/shared/api";
import type { AuthResult, AuthUser } from "@/shared/api";

import { fetchMe } from "./api";

interface SessionContextValue {
  user: AuthUser | null;
  // null пока идёт первичная проверка токена (чтобы не мигать логином).
  loading: boolean;
  // Применить результат логина/регистрации: сохранить токен + пользователя.
  applyAuth: (result: AuthResult) => void;
  clear: () => void;
}

const SessionContext = createContext<SessionContextValue | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);

  // На старте: если есть access-токен — подтянуть профиль. Иначе сразу гость.
  useEffect(() => {
    let cancelled = false;
    if (!getAccessToken()) {
      setLoading(false);
      return;
    }
    fetchMe()
      .then((u) => {
        if (!cancelled) setUser(u);
      })
      .catch(() => {
        if (!cancelled) {
          setAccessToken(null);
          setUser(null);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const applyAuth = useCallback((result: AuthResult) => {
    setAccessToken(result.access_token);
    setUser(result.user);
  }, []);

  const clear = useCallback(() => {
    setAccessToken(null);
    setUser(null);
  }, []);

  const value = useMemo<SessionContextValue>(
    () => ({ user, loading, applyAuth, clear }),
    [user, loading, applyAuth, clear],
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionContextValue {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSession должен использоваться внутри SessionProvider");
  return ctx;
}
