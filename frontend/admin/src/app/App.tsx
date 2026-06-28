import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { SessionProvider, useSession } from "@/entities/session";
import { LoginPage } from "@/pages/login";
import { PageDetailPage } from "@/pages/page-detail";
import { PagesListPage } from "@/pages/pages-list";

import { AppLayout } from "./AppLayout";
import { RequireAuth } from "./RequireAuth";

// /login: уже залогиненного отправляем на список страниц.
function LoginRoute() {
  const { user, loading } = useSession();
  if (loading) return <div className="hp-center hp-muted">Загрузка…</div>;
  if (user) return <Navigate to="/" replace />;
  return <LoginPage />;
}

export function App() {
  return (
    <SessionProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginRoute />} />
          <Route
            element={
              <RequireAuth>
                <AppLayout />
              </RequireAuth>
            }
          >
            <Route path="/" element={<PagesListPage />} />
            <Route path="/pages/:id" element={<PageDetailPage />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </SessionProvider>
  );
}
