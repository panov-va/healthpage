import { NavLink } from "react-router-dom";

// Подвигация разделов страницы статуса (компоненты / инциденты / работы / шаблоны).
const tabs = [
  { to: "", label: "Компоненты", end: true },
  { to: "incidents", label: "Инциденты", end: false },
  { to: "maintenances", label: "Работы", end: false },
  { to: "templates", label: "Шаблоны", end: false },
];

export function PageNav({ pageId }: { pageId: string }) {
  return (
    <nav className="hp-tabs">
      {tabs.map((t) => (
        <NavLink
          key={t.to}
          to={t.to ? `/pages/${pageId}/${t.to}` : `/pages/${pageId}`}
          end={t.end}
          className={({ isActive }) =>
            ["hp-tab", isActive ? "hp-tab--active" : ""].filter(Boolean).join(" ")
          }
        >
          {t.label}
        </NavLink>
      ))}
    </nav>
  );
}
