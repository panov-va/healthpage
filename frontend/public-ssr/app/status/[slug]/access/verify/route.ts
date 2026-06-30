// Route handler цели magic-link (этап 4.2.1). Пользователь переходит по ссылке из письма:
// обмениваем token на токен доступа через backend, ставим HttpOnly-cookie hp_access_<slug>
// (как пароль в 4.2) и редиректим на страницу статуса. Невалидный токен → гейт с ?access_error=1.

import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

function apiBaseURL(): string {
  return process.env.HEALTHPAGE_API_URL ?? "http://localhost:8080/api/v1";
}

export async function GET(
  request: Request,
  { params }: { params: { slug: string } },
) {
  const slug = params.slug;
  const url = new URL(request.url);
  const token = url.searchParams.get("token") ?? "";
  const lang = url.searchParams.get("lang") === "en" ? "en" : "ru";
  const statusPath = `/status/${encodeURIComponent(slug)}`;
  const langQ = lang === "en" ? "lang=en" : "";

  let granted: { access_token: string; expires_in: number } | null = null;
  try {
    const res = await fetch(
      `${apiBaseURL()}/pages/${encodeURIComponent(slug)}/access/verify?token=${encodeURIComponent(token)}`,
      { cache: "no-store" },
    );
    if (res.ok) {
      granted = (await res.json()) as { access_token: string; expires_in: number };
    }
  } catch {
    granted = null;
  }

  if (!granted) {
    const q = ["access_error=1", langQ].filter(Boolean).join("&");
    return NextResponse.redirect(new URL(`${statusPath}?${q}`, request.url), 303);
  }

  const dest = langQ ? `${statusPath}?${langQ}` : statusPath;
  const res = NextResponse.redirect(new URL(dest, request.url), 303);
  res.cookies.set(`hp_access_${slug}`, granted.access_token, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: granted.expires_in,
  });
  return res;
}
