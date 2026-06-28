import type { ReactNode } from "react";

export function Card({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={["hp-card", className ?? ""].join(" ")}>{children}</div>;
}
