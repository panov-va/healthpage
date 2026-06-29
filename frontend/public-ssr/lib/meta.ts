// Метаданные публичной страницы статуса (этап 4.1): заголовок вкладки = название страницы,
// favicon = favicon_url страницы (если задан). Используется в generateMetadata каждой вкладки.
// Приватная/несуществующая страница → дефолтные метаданные (контент всё равно отдаст 404).

import type { Metadata } from "next";

import { fetchPageMeta, PageNotFoundError } from "./api";

export async function buildStatusMetadata(slug: string): Promise<Metadata> {
  try {
    const page = await fetchPageMeta(slug);
    const meta: Metadata = { title: page.name };
    if (page.favicon_url) {
      meta.icons = { icon: page.favicon_url };
    }
    return meta;
  } catch (err) {
    if (err instanceof PageNotFoundError) {
      return {};
    }
    throw err;
  }
}
