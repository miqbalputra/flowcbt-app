// Halaman-halaman siswa (S11): login akun, token tamu, daftar ujian,
// tata tertib & mulai. Mobile-first: satu kolom maks 480 px, target sentuh besar.
import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  ApiError, clearTokens, errorOf, getAccessToken, guestAuth, myExams, setTokens,
  startExam, studentLogin, type StartResult, type StudentExam,
} from "../lib/api";

export function Shell({ title, children }: { title?: string; children: React.ReactNode }) {
  return (
    <div className="mx-auto flex min-h-dvh w-full max-w-[480px] flex-col px-4 py-6">
      <header className="mb-4 flex items-center justify-between">
        <span className="font-bold">FlowCBT</span>
        {title && <span className="text-sm text-muted">{title}</span>}
      </header>
      {children}
    </div>
  );
}

// ---- login akun siswa (FR-1.3) ----

export function LoginPage() {
  const navigate = useNavigate();
  const [form, setForm] = useState({ school_code: "", username: "", password: "" });
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (getAccessToken()) navigate("/exams", { replace: true });
  }, [navigate]);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const t = await studentLogin(form.school_code, form.username, form.password);
      setTokens(t.access_token, t.refresh_token);
      navigate("/exams", { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Login gagal. Coba lagi.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Shell title="Masuk">
      <form onSubmit={submit} className="flex flex-col gap-3">
        <h1 className="text-xl font-semibold">Masuk Siswa</h1>
        {error && <p role="alert" className="text-sm" style={{ color: "var(--color-danger)" }}>{error}</p>}
        <label className="flex flex-col gap-1 text-sm">
          <span>Kode sekolah</span>
          <input required value={form.school_code} onChange={(e) => setForm({ ...form, school_code: e.target.value })}
                 className="input" autoCapitalize="none" />
        </label>
        <label className="flex flex-col gap-1 text-sm">
          <span>Username</span>
          <input required value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })}
                 className="input" autoCapitalize="none" />
        </label>
        <label className="flex flex-col gap-1 text-sm">
          <span>Password</span>
          <input required type="password" value={form.password}
                 onChange={(e) => setForm({ ...form, password: e.target.value })} className="input" />
        </label>
        <button type="submit" className="btn-primary touch-target" disabled={busy}>
          {busy ? "Memproses…" : "Masuk"}
        </button>
        <button type="button" className="btn-ghost touch-target" onClick={() => navigate("/guest")}>
          Ikut ujian tanpa akun (token)
        </button>
      </form>
    </Shell>
  );
}

// ---- login tamu token (FR-1.3) ----

export function GuestPage() {
  const navigate = useNavigate();
  const [token, setToken] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const t = await guestAuth(token.trim().toUpperCase());
      setTokens(t.access_token, t.refresh_token);
      navigate("/exams", { replace: true });
    } catch (err) {
      setError(errorOf(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Shell title="Token">
      <form onSubmit={submit} className="flex flex-col gap-3">
        <h1 className="text-xl font-semibold">Ikut Ujian Tanpa Akun</h1>
        <p className="text-sm text-muted">Masukkan token dari pengawas. Huruf besar/kecil diabaikan.</p>
        {error && <p role="alert" className="text-sm" style={{ color: "var(--color-danger)" }}>{error}</p>}
        <label className="flex flex-col gap-1 text-sm">
          <span>Token ujian</span>
          <input required value={token} onChange={(e) => setToken(e.target.value)}
                 className="input text-center font-mono text-lg tracking-widest" />
        </label>
        <button type="submit" className="btn-primary touch-target" disabled={busy}>
          {busy ? "Memeriksa…" : "Lanjut"}
        </button>
        <button type="button" className="btn-ghost touch-target" onClick={() => navigate("/login")}>
          Punya akun? Masuk
        </button>
      </form>
    </Shell>
  );
}

// ---- dashboard ujian siswa ----

export function ExamsPage() {
  const navigate = useNavigate();
  const [exams, setExams] = useState<StudentExam[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setExams(await myExams());
      setError(null);
    } catch (e) {
      setError(errorOf(e));
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const fmtRange = (x: StudentExam) =>
    [x.open_at && `buka ${new Date(x.open_at).toLocaleString("id-ID")}`,
     x.close_at && `tutup ${new Date(x.close_at).toLocaleString("id-ID")}`]
      .filter(Boolean).join(" · ") || "tanpa jadwal";

  return (
    <Shell title="Ujian Saya">
      <h1 className="mb-3 text-xl font-semibold">Ujian Saya</h1>
      {error && <p role="alert" className="mb-3 text-sm" style={{ color: "var(--color-danger)" }}>{error}</p>}
      <div className="flex flex-col gap-3">
        {exams === null ? (
          <p className="text-sm text-muted">Memuat…</p>
        ) : exams.length === 0 ? (
          <>
            <p className="text-sm text-muted">Belum ada ujian untuk Anda.</p>
            <button type="button" className="btn-ghost touch-target" onClick={() => { clearTokens(); navigate("/login"); }}>
              Keluar
            </button>
          </>
        ) : (
          exams.map((x) => (
            <div key={x.id} className="rounded-lg border p-4" style={{ borderColor: "var(--color-border)" }}>
              <p className="font-medium">{x.name}</p>
              <p className="text-sm text-muted">{fmtRange(x)}</p>
              <p className="text-sm text-muted">
                {x.attempts_done}/{x.max_attempts ?? "∞"} percobaan · {x.duration_minutes ?? "?"} menit
                {x.status === "paused" && " · DIJEDA GURU"}
              </p>
              <div className="mt-3">
                {x.active_attempt_id ? (
                  <button type="button" className="btn-primary touch-target" onClick={() => navigate(`/attempt/${x.active_attempt_id}`)}>
                    Lanjutkan pengerjaan
                  </button>
                ) : (
                  <button type="button" className="btn-primary touch-target"
                          onClick={() => navigate(`/exams/${x.id}/start`)}
                          disabled={x.status !== "published"}>
                    {x.status === "published" ? "Mulai" : "Menunggu guru"}
                  </button>
                )}
              </div>
            </div>
          ))
        )}
      </div>
    </Shell>
  );
}

// ---- tata tertib & mulai ujian ----

const RULES = [
  "Kerjakan mandiri tanpa bantuan orang lain atau alat yang tidak diizinkan.",
  "Jangan keluar aplikasi/berpindah tab selama ujian — terhitung pelanggaran.",
  "Waktu berakhir otomatis saat timer habis; jawaban tersimpan per perubahan.",
  "Kamera (bila diaktifkan) mengambil snapshot berkala saat ujian berjalan.",
  "Pelanggaran berulang dapat mengurangi nilai atau dilaporkan curang.",
  "Dengan memulai, Anda menyetujui tata tertib di atas.",
];

export function StartPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [agree, setAgree] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const begin = async () => {
    setBusy(true);
    try {
      const res: StartResult = await startExam(id);
      // Soal & deadline dipindah ke halaman pengerjaan via sessionStorage (S12 mengganti dengan store).
      sessionStorage.setItem("flowcbt:attempt", JSON.stringify(res));
      navigate(`/attempt/${res.attempt_id}`);
    } catch (e) {
      setError(errorOf(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Shell title="Tata tertib">
      <h1 className="mb-3 text-xl font-semibold">Tata Tertib Ujian</h1>
      <ul className="mb-4 flex list-disc flex-col gap-2 pl-5 text-sm">
        {RULES.map((r, i) => <li key={i}>{r}</li>)}
      </ul>
      {error && <p role="alert" className="mb-3 text-sm" style={{ color: "var(--color-danger)" }}>{error}</p>}
      <label className="mb-3 flex items-start gap-2 text-sm">
        <input type="checkbox" checked={agree} onChange={(e) => setAgree(e.target.checked)}
               className="mt-0.5 size-5" />
        <span>Saya membaca dan menyetujui tata tertib.</span>
      </label>
      <button type="button" className="btn-primary touch-target w-full" disabled={!agree || busy} onClick={() => void begin()}>
        {busy ? "Menyiapkan…" : "Mulai Ujian"}
      </button>
    </Shell>
  );
}
