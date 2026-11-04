import { useState, type FormEvent } from "react";
import { Link } from "react-router";
import { useAuth } from "../auth";

export function AuthPage({ mode }: { mode: "login" | "register" }) {
  const { login, register } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const isLogin = mode === "login";

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      if (isLogin) await login(email, password);
      else await register(email, password, displayName);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Что-то пошло не так");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="auth">
      <form className="auth__form" onSubmit={onSubmit}>
        <h1 className="auth__brand">Wormest</h1>
        <p className="auth__lead">
          {isLogin ? "Войдите, чтобы открыть свою библиотеку." : "Аккаунт нужен, чтобы заметки были доступны с любого устройства."}
        </p>

        {!isLogin && (
          <label className="field">
            <span>Имя</span>
            <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} autoComplete="name" />
          </label>
        )}
        <label className="field">
          <span>Email</span>
          <input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" />
        </label>
        <label className="field">
          <span>Пароль</span>
          <input
            type="password"
            required
            minLength={isLogin ? undefined : 8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete={isLogin ? "current-password" : "new-password"}
          />
          {!isLogin && <small>Не короче 8 символов.</small>}
        </label>

        {error && <p className="notice notice--error" role="alert">{error}</p>}

        <button className="btn btn--primary btn--wide" disabled={busy}>
          {isLogin ? "Войти" : "Создать аккаунт"}
        </button>

        <p className="auth__switch">
          {isLogin ? (
            <>Нет аккаунта? <Link to="/register">Создать</Link></>
          ) : (
            <>Уже есть аккаунт? <Link to="/login">Войти</Link></>
          )}
        </p>
      </form>
    </div>
  );
}
