// Публичная оферта (этап 6.8, DESIGN §4.2) — условия подключения и отключения автоплатежа.
// Требование ЮKassa для рекуррентных списаний. RU/EN, SSR. Локаль — через ?lang=en.
//
// [ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ БИЛЛИНГА]: финализировать юридический текст и реквизиты
// самозанятого (ИНН, контакты) с юристом; актуальная цена Premium — на странице тарифов.

import type { Metadata } from "next";

import { resolveLocale, type Locale } from "../../lib/i18n";

interface OfferProps {
  searchParams: { lang?: string | string[] };
}

export function generateMetadata({ searchParams }: OfferProps): Metadata {
  const t = offer(resolveLocale(searchParams.lang));
  return { title: t.title, description: t.intro };
}

export default function OfferPage({ searchParams }: OfferProps) {
  const t = offer(resolveLocale(searchParams.lang));
  return (
    <main style={{ maxWidth: 760, margin: "0 auto", padding: "2rem 1.25rem", lineHeight: 1.6 }}>
      <h1>{t.title}</h1>
      <p>
        <em>{t.draftNote}</em>
      </p>
      <p>{t.intro}</p>
      {t.sections.map((s) => (
        <section key={s.h}>
          <h2 style={{ marginTop: "1.5rem" }}>{s.h}</h2>
          {s.p.map((para, i) => (
            <p key={i}>{para}</p>
          ))}
        </section>
      ))}
    </main>
  );
}

interface OfferDict {
  title: string;
  draftNote: string;
  intro: string;
  sections: { h: string; p: string[] }[];
}

function offer(locale: Locale): OfferDict {
  return locale === "en" ? offerEn : offerRu;
}

const offerRu: OfferDict = {
  title: "Публичная оферта на оказание услуг HealthPage",
  draftNote:
    "Черновик. Текст и реквизиты будут финализированы до запуска платного тарифа.",
  intro:
    "Настоящая оферта описывает условия подключения платного тарифа Premium сервиса HealthPage, " +
    "порядок автоматических списаний (автоплатежа) и его отключения.",
  sections: [
    {
      h: "1. Предмет",
      p: [
        "Исполнитель предоставляет доступ к расширенным возможностям сервиса HealthPage (тариф Premium): " +
          "собственный домен с TLS, приватные страницы, собственный SMTP, скрытие брендинга и приоритетная поддержка.",
      ],
    },
    {
      h: "2. Стоимость и периодичность",
      p: [
        "Актуальная стоимость тарифа Premium и доступные периоды (помесячно / ежегодно) указаны на странице тарифов сервиса.",
        "Оплата производится банковской картой физического лица через платёжного провайдера (ЮKassa).",
      ],
    },
    {
      h: "3. Автоплатёж (рекуррентные списания)",
      p: [
        "Подключая Premium, вы соглашаетесь на автоматическое списание стоимости следующего периода с привязанной карты " +
          "в дату окончания текущего оплаченного периода.",
        "Данные карты на стороне сервиса не хранятся — используется платёжный токен провайдера.",
      ],
    },
    {
      h: "4. Отключение автоплатежа",
      p: [
        "Отключить автопродление можно в любой момент в личном кабинете (раздел «Тариф») — кнопка отмены подписки. " +
          "Доступ к Premium сохраняется до конца уже оплаченного периода, после чего аккаунт переходит на бесплатный тариф.",
      ],
    },
    {
      h: "5. Возвраты",
      p: [
        "Возврат за неиспользованную часть оплаченного периода осуществляется по обращению в поддержку в соответствии с законодательством РФ.",
      ],
    },
    {
      h: "6. Фискализация",
      p: [
        "Исполнитель применяет налоговый режим «Налог на профессиональный доход» (самозанятость). " +
          "По каждому платежу формируется чек через сервис «Мой налог»; провайдер передаёт сведения в ФНС.",
      ],
    },
  ],
};

const offerEn: OfferDict = {
  title: "HealthPage Public Offer",
  draftNote: "Draft. Wording and legal details will be finalized before the paid plan launch.",
  intro:
    "This offer describes the terms of subscribing to the paid Premium plan of HealthPage, " +
    "the recurring auto-payment process, and how to disable it.",
  sections: [
    {
      h: "1. Subject",
      p: [
        "The provider grants access to advanced HealthPage features (Premium): custom domain with TLS, " +
          "private pages, custom SMTP, white-label, and priority support.",
      ],
    },
    {
      h: "2. Price and billing period",
      p: [
        "The current Premium price and available periods (monthly / yearly) are shown on the pricing page.",
        "Payment is made with an individual's bank card via the payment provider (YooKassa).",
      ],
    },
    {
      h: "3. Auto-payment (recurring charges)",
      p: [
        "By subscribing to Premium you agree to automatic charging of the next period to your saved card " +
          "on the expiry date of the current paid period.",
        "Card data is not stored on our side — a provider payment token is used.",
      ],
    },
    {
      h: "4. Disabling auto-payment",
      p: [
        "You can disable auto-renewal at any time in your account (the “Billing” section). " +
          "Premium access remains until the end of the already paid period, after which the account reverts to the free plan.",
      ],
    },
    {
      h: "5. Refunds",
      p: ["Refunds for an unused part of a paid period are handled via support per applicable law."],
    },
    {
      h: "6. Fiscalization",
      p: [
        "The provider operates under the self-employment tax regime; a receipt is issued for each payment " +
          "via the tax service, which reports to the authorities.",
      ],
    },
  ],
};
