import { useState } from "react";
import { useSearchParams } from "react-router-dom";

import { login, register, useSession } from "@/entities/session";
import { HttpError } from "@/shared/api";
import { Button, Field, Input } from "@/shared/ui";

type Mode = "login" | "register";

export function AuthForm({ onSuccess }: { onSuccess: () => void }) {
  const { applyAuth } = useSession();
  const [searchParams] = useSearchParams();
  const [mode, setMode] = useState<Mode>(searchParams.get("mode") === "register" ? "register" : "login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const result =
        mode === "login"
          ? await login({ email, password })
          : await register({ email, password, name: name || undefined });
      applyAuth(result);
      onSuccess();
    } catch (err) {
      if (err instanceof HttpError) {
        setError(
          mode === "login" && err.status === 401
            ? "Неверный email или пароль"
            : err.message,
        );
      } else {
        setError("Не удалось выполнить запрос");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} style={{ width: 320 }}>
      <h1>HealthPage Admin</h1>
      <p className="hp-muted" style={{ marginTop: 0 }}>
        {mode === "login" ? "Вход оператора" : "Регистрация оператора"}
      </p>

      {mode === "register" && (
        <Field label="Имя (необязательно)">
          <Input value={name} onChange={(e) => setName(e.target.value)} autoComplete="name" />
        </Field>
      )}

      <Field label="Email">
        <Input
          type="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          autoComplete="email"
        />
      </Field>

      <Field label="Пароль">
        <Input
          type="password"
          required
          minLength={mode === "register" ? 8 : undefined}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete={mode === "login" ? "current-password" : "new-password"}
        />
      </Field>

      {error && <div className="hp-error">{error}</div>}

      <Button type="submit" disabled={busy} style={{ width: "100%", marginTop: 4 }}>
        {busy ? "…" : mode === "login" ? "Войти" : "Зарегистрироваться"}
      </Button>

      <p className="hp-muted" style={{ fontSize: 13, marginTop: 14, textAlign: "center" }}>
        {mode === "login" ? "Нет аккаунта? " : "Уже есть аккаунт? "}
        <a
          href="#"
          onClick={(e) => {
            e.preventDefault();
            setError(null);
            setMode(mode === "login" ? "register" : "login");
          }}
        >
          {mode === "login" ? "Зарегистрироваться" : "Войти"}
        </a>
      </p>
    </form>
  );
}
