// Маршрутизация кастомных доменов клиентов (этап 4.3.3): раньше это делал отдельный edge-прокси
// (backend/internal/edge, не используется в проде — см. DEPLOY.md §8), теперь Traefik/Dokploy шлёт
// трафик кастомного домена прямо в public-ssr, и без этого middleware на корне открывался бы
// лендинг healthpage.ru вместо страницы статуса клиента. Резолвим Host → slug через
// GET /pages/by-domain (публичный, только для верифицированных доменов) и молча rewrite'им
// (без редиректа — адрес в браузере не меняется).
import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

function apiBaseURL(): string {
  return process.env.HEALTHPAGE_API_URL ?? "http://localhost:8080/api/v1";
}

// ownHosts — хосты, на которых сам public-ssr отдаёт лендинг (не кастомные домены клиентов).
function ownHosts(): Set<string> {
  const primary = new URL(process.env.PUBLIC_URL ?? "https://healthpage.ru").hostname;
  return new Set([primary, `www.${primary}`, "localhost"]);
}

export async function middleware(request: NextRequest) {
  const host = request.headers.get("host")?.split(":")[0]?.toLowerCase();
  if (!host || ownHosts().has(host)) {
    return NextResponse.next();
  }

  try {
    const res = await fetch(`${apiBaseURL()}/pages/by-domain?domain=${encodeURIComponent(host)}`, {
      cache: "no-store",
    });
    if (!res.ok) {
      return NextResponse.next();
    }
    const { slug } = (await res.json()) as { slug: string };
    const url = request.nextUrl.clone();
    url.pathname = `/status/${slug}`;
    return NextResponse.rewrite(url);
  } catch {
    // API недоступен — отдаём лендинг как есть, не блокируем запрос.
    return NextResponse.next();
  }
}

// Резолв домена нужен только на корне — внутренние ссылки страницы статуса уже ведут
// на /status/<slug>/... (как раньше делал edge-прокси, см. комментарий выше).
export const config = {
  matcher: "/",
};
