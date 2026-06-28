import { Link, Outlet } from "react-router-dom";

import { useSession } from "@/entities/session";
import { LogoutButton } from "@/features/auth";

export function AppLayout() {
  const { user } = useSession();
  return (
    <div className="hp-app">
      <header className="hp-topbar">
        <Link to="/" className="hp-topbar__brand" style={{ color: "inherit" }}>
          HealthPage Admin
        </Link>
        <div className="hp-topbar__user">
          {user && <span>{user.email}</span>}
          <LogoutButton />
        </div>
      </header>
      <Outlet />
    </div>
  );
}
