import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { SessionProvider, useSession } from "@/entities/session";
import { IncidentDetailPage, IncidentsListPage } from "@/pages/incidents";
import { LoginPage } from "@/pages/login";
import { MaintenanceDetailPage, MaintenancesListPage } from "@/pages/maintenances";
import { PageDetailPage } from "@/pages/page-detail";
import { PagesListPage } from "@/pages/pages-list";
import { SubscribersPage } from "@/pages/subscribers";
import { TemplatesPage } from "@/pages/templates";

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
            <Route path="/pages/:id/incidents" element={<IncidentsListPage />} />
            <Route
              path="/pages/:id/incidents/:incidentId"
              element={<IncidentDetailPage />}
            />
            <Route path="/pages/:id/maintenances" element={<MaintenancesListPage />} />
            <Route
              path="/pages/:id/maintenances/:maintenanceId"
              element={<MaintenanceDetailPage />}
            />
            <Route path="/pages/:id/templates" element={<TemplatesPage />} />
            <Route path="/pages/:id/subscribers" element={<SubscribersPage />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </SessionProvider>
  );
}
