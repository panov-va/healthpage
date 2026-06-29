// Клиентская страница отписки (этап 3.10, минимальная). Открывается по ссылке из письма
// (`/unsubscribe?token=…`): выполняет отписку через backend и показывает дружелюбное
// подтверждение RU/EN. Семантика «один клик» совпадает с GET /unsubscribe (идемпотентна).
import { unsubscribeByToken } from "../../lib/api";
import { resolveLocale, type Locale } from "../../lib/i18n";

export const dynamic = "force-dynamic";

interface Texts {
  title: string;
  okTitle: string;
  okBody: string;
  failTitle: string;
  failBody: string;
  missingTitle: string;
  missingBody: string;
}

const TEXTS: Record<Locale, Texts> = {
  ru: {
    title: "Управление подпиской",
    okTitle: "Вы отписаны",
    okBody: "Больше не будем присылать вам уведомления об этой странице статуса.",
    failTitle: "Ссылка недействительна",
    failBody: "Ссылка отписки устарела или некорректна. Попробуйте ссылку из последнего письма.",
    missingTitle: "Нет токена",
    missingBody: "В ссылке отсутствует токен отписки.",
  },
  en: {
    title: "Manage subscription",
    okTitle: "You're unsubscribed",
    okBody: "We will no longer send you notifications for this status page.",
    failTitle: "Invalid link",
    failBody: "The unsubscribe link is expired or invalid. Try the link from the latest email.",
    missingTitle: "Missing token",
    missingBody: "The unsubscribe link has no token.",
  },
};

export default async function UnsubscribePage({
  searchParams,
}: {
  searchParams: { token?: string | string[]; lang?: string | string[] };
}) {
  const locale = resolveLocale(searchParams.lang);
  const t = TEXTS[locale];
  const token = Array.isArray(searchParams.token) ? searchParams.token[0] : searchParams.token;

  let title = t.missingTitle;
  let body = t.missingBody;
  if (token) {
    const ok = await unsubscribeByToken(token);
    title = ok ? t.okTitle : t.failTitle;
    body = ok ? t.okBody : t.failBody;
  }

  return (
    <main className="page">
      <section className="group">
        <div style={{ padding: "32px 24px", textAlign: "center" }}>
          <h1 style={{ marginTop: 0 }}>{t.title}</h1>
          <h2 style={{ fontSize: "1.2rem" }}>{title}</h2>
          <p>{body}</p>
        </div>
      </section>
    </main>
  );
}
