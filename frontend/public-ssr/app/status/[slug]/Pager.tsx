// Пагинация списков (инциденты/работы): ссылки «новее/старее» с сохранением локали (этап 2.10).

import Link from "next/link";

import type { Locale } from "../../../lib/i18n";

export function Pager({
  basePath,
  locale,
  page,
  perPage,
  total,
  prevLabel,
  nextLabel,
}: {
  basePath: string;
  locale: Locale;
  page: number;
  perPage: number;
  total: number;
  prevLabel: string;
  nextLabel: string;
}) {
  const totalPages = Math.max(1, Math.ceil(total / perPage));
  if (totalPages <= 1) {
    return null;
  }
  const href = (p: number): string => {
    const params = new URLSearchParams();
    if (p > 1) params.set("page", String(p));
    if (locale === "en") params.set("lang", "en");
    const qs = params.toString();
    return qs ? `${basePath}?${qs}` : basePath;
  };
  return (
    <nav className="pager">
      {page > 1 ? (
        <Link href={href(page - 1)} className="btn btn-secondary">
          {prevLabel}
        </Link>
      ) : (
        <span />
      )}
      <span className="pager-info">
        {page} / {totalPages}
      </span>
      {page < totalPages ? (
        <Link href={href(page + 1)} className="btn btn-secondary">
          {nextLabel}
        </Link>
      ) : (
        <span />
      )}
    </nav>
  );
}
