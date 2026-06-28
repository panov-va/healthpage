import type { ReactNode } from "react";
import { Navigate } from "react-router-dom";

import { useSession } from "@/entities/session";

// Гард приватных маршрутов: пока идёт первичная проверка токена — ничего не
// рендерим (чтобы не мигать логином), затем либо контент, либо редирект на /login.
export function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useSession();
  if (loading) return <div className="hp-center hp-muted">Загрузка…</div>;
  if (!user) return <Navigate to="/login" replace />;
  return <>{children}</>;
}
