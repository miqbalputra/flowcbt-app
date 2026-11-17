import { FormEvent, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { errorOf, useAuth } from "./AuthContext";
import { AuthShell, Field, inputCls } from "./LoginPage";

// FR-1.1: nama, email, HP (opsional), password, nama sekolah, jenjang.
// Ditambah kode sekolah (keputusan PRD 18.6 #1: dibutuhkan login siswa dengan akun).
const LEVELS = [
  { value: "paud_tk", label: "PAUD / TK" },
  { value: "sd_mi", label: "SD / MI" },
  { value: "smp_mts", label: "SMP / MTs" },
  { value: "sma_smk_ma", label: "SMA / SMK / MA" },
  { value: "perguruan_tinggi", label: "Perguruan Tinggi" },
  { value: "kursus_bimbel", label: "Kursus / Bimbel" },
  { value: "lainnya", label: "Lainnya" },
];

export default function RegisterPage() {
  const { register, verifyEmail } = useAuth();
  const navigate = useNavigate();
  const [form, setForm] = useState({
    name: "",
    email: "",
    phone: "",
    password: "",
    school_name: "",
    school_code: "",
    level: "sd_mi",
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [pendingToken, setPendingToken] = useState<string | null>(null);

  const set = (key: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) =>
    setForm((f) => ({ ...f, [key]: e.target.value }));

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await register({
        name: form.name,
        email: form.email,
        phone: form.phone || undefined,
        password: form.password,
        school_name: form.school_name,
        school_code: form.school_code,
        level: form.level,
      });
      if (res.verify_token) {
        // Development (belum ada mailer): langsung tawarkan verifikasi.
        setPendingToken(res.verify_token);
        setNotice("Registrasi berhasil. Verifikasi email untuk mengaktifkan akun.");
      } else {
        setNotice("Registrasi berhasil. Buka email Anda dan klik tautan verifikasi.");
      }
    } catch (err) {
      setError(errorOf(err));
    } finally {
      setBusy(false);
    }
  };

  const onVerify = async () => {
    if (!pendingToken) return;
    setBusy(true);
    setError(null);
    try {
      await verifyEmail(pendingToken);
      navigate("/login", { replace: true });
    } catch (err) {
      setError(errorOf(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthShell title="Daftar Sekolah">
      {notice ? (
        <div className="flex flex-col gap-4">
          <p style={{ color: "var(--color-success)" }}>{notice}</p>
          {pendingToken && (
            <button onClick={onVerify} disabled={busy} className="btn-primary">
              {busy ? "Memproses…" : "Verifikasi email sekarang (dev)"}
            </button>
          )}
          <Link to="/login" className="text-sm" style={{ color: "var(--color-primary)" }}>
            Ke halaman masuk
          </Link>
        </div>
      ) : (
        <form onSubmit={onSubmit} className="flex flex-col gap-4">
          <Field label="Nama lengkap">
            <input required value={form.name} onChange={set("name")} className={inputCls} />
          </Field>
          <Field label="Email">
            <input type="email" required value={form.email} onChange={set("email")} className={inputCls} autoComplete="email" />
          </Field>
          <Field label="Nomor HP (opsional)">
            <input type="tel" value={form.phone} onChange={set("phone")} className={inputCls} autoComplete="tel" />
          </Field>
          <Field label="Nama sekolah / lembaga">
            <input required value={form.school_name} onChange={set("school_name")} className={inputCls} />
          </Field>
          <Field label="Kode sekolah (huruf kecil/angka, unik — dipakai siswa saat login)">
            <input
              required
              pattern="[a-z0-9][a-z0-9-]{2,39}"
              title="3-40 karakter: huruf kecil, angka, tanda hubung"
              value={form.school_code}
              onChange={set("school_code")}
              className={inputCls}
            />
          </Field>
          <Field label="Jenjang">
            <select required value={form.level} onChange={set("level")} className={inputCls}>
              {LEVELS.map((l) => (
                <option key={l.value} value={l.value}>
                  {l.label}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Password (min. 8 karakter, huruf + angka)">
            <input
              type="password"
              required
              minLength={8}
              value={form.password}
              onChange={set("password")}
              className={inputCls}
              autoComplete="new-password"
            />
          </Field>
          {error && <p role="alert" style={{ color: "var(--color-danger)" }}>{error}</p>}
          <button type="submit" disabled={busy} className="btn-primary">
            {busy ? "Memproses…" : "Daftar"}
          </button>
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            Mendaftar otomatis membuat paket Gratis/Trial (10 siswa, 1 ujian, 14 hari).
          </p>
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            Sudah punya akun?{" "}
            <Link to="/login" style={{ color: "var(--color-primary)" }}>
              Masuk
            </Link>
          </p>
        </form>
      )}
    </AuthShell>
  );
}
