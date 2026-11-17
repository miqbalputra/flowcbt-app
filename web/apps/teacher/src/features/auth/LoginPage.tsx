import { FormEvent, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { errorOf, useAuth } from "./AuthContext";

export default function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await login(email, password);
      navigate("/dashboard", { replace: true });
    } catch (err) {
      setError(errorOf(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthShell title="Masuk">
      <form onSubmit={onSubmit} className="flex flex-col gap-4">
        <Field label="Email">
          <input
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className={inputCls}
            autoComplete="email"
          />
        </Field>
        <Field label="Password">
          <input
            type="password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className={inputCls}
            autoComplete="current-password"
          />
        </Field>
        {error && <p role="alert" style={{ color: "var(--color-danger)" }}>{error}</p>}
        <button type="submit" disabled={busy} className="btn-primary">
          {busy ? "Memproses…" : "Masuk"}
        </button>
        <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
          Belum punya akun?{" "}
          <Link to="/register" style={{ color: "var(--color-primary)" }}>
            Daftar sekolah
          </Link>
        </p>
      </form>
    </AuthShell>
  );
}

export const inputCls =
  "w-full rounded px-3 py-2.5 text-base outline-none touch-target";

// Shell bersama untuk halaman auth (tanpa sidebar).
export function AuthShell({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="flex min-h-dvh items-center justify-center px-4" style={{ background: "var(--color-bg-muted)" }}>
      <div
        className="w-full max-w-sm rounded-xl border p-6 shadow-sm"
        style={{ background: "var(--color-surface)", borderColor: "var(--color-border)" }}
      >
        <div className="mb-6 text-center">
          <div className="text-lg font-bold">FlowCBT</div>
          <h1 className="mt-1 font-semibold">{title}</h1>
        </div>
        {children}
      </div>
    </div>
  );
}

export function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-sm" style={{ color: "var(--color-text-muted)" }}>
        {label}
      </span>
      {children}
    </label>
  );
}
