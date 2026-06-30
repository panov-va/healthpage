// Route handler запроса magic-link доступа (этап 4.2.1). Принимает POST с email, дёргает backend
// (всегда 202 — не раскрывает, разрешён ли адрес) и редиректит обратно на гейт с ?link_sent=1.

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
  const email = String(form.get("email") ?? "");
  const lang = form.get("lang") === "en" ? "en" : "ru";

  try {
    await fetch(`${apiBaseURL()}/pages/${encodeURIComponent(slug)}/access/request-link`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
      cache: "no-store",
    });
  } catch {
    // Игнорируем — пользователю всё равно показываем нейтральное сообщение.
  }

  const q = lang === "en" ? "link_sent=1&lang=en" : "link_sent=1";
  return NextResponse.redirect(
    new URL(`/status/${encodeURIComponent(slug)}?${q}`, request.url),
    303,
  );
}
