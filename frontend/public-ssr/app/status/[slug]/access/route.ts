// Route handler парольного гейта приватной страницы (этап 4.2). Принимает POST с паролем,
// проверяет его через backend (POST /pages/{slug}/access), и при успехе ставит HttpOnly-cookie
// доступа на origin public-ssr (её затем форвардит lib/api как X-Page-Access). Редиректит на
// страницу статуса (303 — POST→GET). Неверный пароль → редирект назад с ?access_error=1.

import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

function apiBaseURL(): string {
  return process.env.HEALTHPAGE_API_URL ?? "http://localhost:8080/api/v1";
}

export async function POST(
  request: Request,
  { params }: { params: { slug: string } },
) {
  const slug = params.slug;
  const form = await request.formData();
  const password = String(form.get("password") ?? "");
  const lang = form.get("lang") === "en" ? "en" : "ru";

  const statusPath = `/status/${encodeURIComponent(slug)}`;
  const withLang = (p: string, q = "") =>
    lang === "en" ? `${p}?${q ? q + "&" : ""}lang=en` : q ? `${p}?${q}` : p;

  let granted: { access_token: string; expires_in: number } | null = null;
  try {
    const res = await fetch(`${apiBaseURL()}/pages/${encodeURIComponent(slug)}/access`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password }),
      cache: "no-store",
    });
    if (res.ok) {
      granted = (await res.json()) as { access_token: string; expires_in: number };
    }
  } catch {
    granted = null;
  }

  if (!granted) {
    return NextResponse.redirect(
      new URL(withLang(statusPath, "access_error=1"), request.url),
      303,
    );
  }

  const res = NextResponse.redirect(new URL(withLang(statusPath), request.url), 303);
  res.cookies.set(`hp_access_${slug}`, granted.access_token, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: granted.expires_in,
  });
  return res;
}
