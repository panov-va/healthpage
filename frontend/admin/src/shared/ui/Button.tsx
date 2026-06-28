import type { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "danger";

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: "sm" | "md";
}

const variantClass: Record<Variant, string> = {
  primary: "",
  secondary: "hp-btn--secondary",
  danger: "hp-btn--danger",
};

export function Button({ variant = "primary", size = "md", className, ...rest }: Props) {
  const classes = [
    "hp-btn",
    variantClass[variant],
    size === "sm" ? "hp-btn--sm" : "",
    className ?? "",
  ]
    .filter(Boolean)
    .join(" ");
  return <button className={classes} {...rest} />;
}
