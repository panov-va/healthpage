import type { ReactNode } from "react";

export const metadata = {
  title: "HealthPage",
  description: "Страницы статуса для вашего продукта / Status pages for your product",
};

// Корневой layout. lang переключится на выбранную локаль на этапе i18n.
export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="ru">
      <body>{children}</body>
    </html>
  );
}
