import { useCallback, useEffect, FormEvent, useState } from "react";
import { ThemeToggle } from "flowcbt-ui";
import { ApiError, api } from "../../lib/api";
import { Field, inputCls } from "../auth/LoginPage";

// M3/S3: /settings — profil, manajemen guru (FR-1.5), preferensi tema (FR-11.1).
interface Teacher {
  id: string;
  name: string;
  email: string;
  role: string;
  status: string;
}

export default function SettingsPage() {
  return (
    <div className="mx-auto max-w-2xl">
      <h1 className="text-xl font-semibold">Pengaturan</h1>

      <Section title="Profil">
        <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
          Profil pengguna diisi pada sprint berikutnya (endpoint <code>/users/me</code>).
        </p>
      </Section>

      <TeacherSection />

      <Section title="Tema tampilan">
        <div className="flex items-center gap-3">
          <ThemeToggle />
          <span className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            Pilihan tersimpan di perangkat ini dan mengikuti pengaturan sistem bila dipilih.
          </span>
        </div>
      </Section>
    </div>
  );
}

function TeacherSection() {
  const [teachers, setTeachers] = useState<Teacher[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [invite, setInvite] = useState({ name: "", email: "", phone: "" });
  const [inviteMsg, setInviteMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      setTeachers(await api<Teacher[]>("/teachers"));
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Gagal memuat daftar guru");
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const submitInvite = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setInviteMsg(null);
    try {
      await api<{ message?: string }>("/teachers/invite", { method: "POST", body: invite });
      setInviteMsg(`Undangan terkirim ke ${invite.email}.`);
      setInvite({ name: "", email: "", phone: "" });
      await load();
    } catch (err) {
      setInviteMsg(err instanceof ApiError ? err.message : "Gagal mengirim undangan");
    } finally {
      setBusy(false);
    }
  };

  const deactivate = async (id: string, name: string) => {
    if (!window.confirm(`Nonaktifkan guru "${name}"?`)) return;
    try {
      await api(`/teachers/${id}`, { method: "DELETE" });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal menonaktifkan guru");
    }
  };

  return (
    <Section title="Guru">
      {error && <p role="alert" style={{ color: "var(--color-danger)" }}>{error}</p>}

      {teachers === null ? (
        <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>Memuat…</p>
      ) : (
        <ul className="flex flex-col divide-y" style={{ borderColor: "var(--color-border)" }}>
          {teachers.map((t) => (
            <li key={t.id} className="flex items-center justify-between gap-3 py-2.5">
              <div>
                <div className="font-medium">{t.name}</div>
                <div className="text-sm" style={{ color: "var(--color-text-muted)" }}>{t.email}</div>
              </div>
              <div className="flex items-center gap-2">
                <span
                  className="rounded-full px-2 py-0.5 text-xs"
                  style={{
                    background: t.status === "active" ? "var(--color-success)" : "var(--color-warning)",
                    color: "var(--color-on-primary)",
                  }}
                >
                  {t.status === "active" ? "Aktif" : t.status === "invited" ? "Diundang" : "Nonaktif"}
                </span>
                {t.role === "teacher" && t.status !== "disabled" && (
                  <button
                    type="button"
                    onClick={() => deactivate(t.id, t.name)}
                    className="touch-target rounded-full border px-3 py-1 text-xs"
                    style={{ borderColor: "var(--color-border)" }}
                  >
                    Nonaktifkan
                  </button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}

      <form onSubmit={submitInvite} className="mt-4 flex flex-col gap-3 rounded-lg border p-4"
            style={{ borderColor: "var(--color-border)" }}>
        <div className="font-medium">Undang guru (owner saja)</div>
        <Field label="Nama guru">
          <input required value={invite.name} onChange={(e) => setInvite({ ...invite, name: e.target.value })} className={inputCls} />
        </Field>
        <Field label="Email guru">
          <input type="email" required value={invite.email} onChange={(e) => setInvite({ ...invite, email: e.target.value })} className={inputCls} />
        </Field>
        <Field label="Nomor HP (opsional)">
          <input type="tel" value={invite.phone} onChange={(e) => setInvite({ ...invite, phone: e.target.value })} className={inputCls} />
        </Field>
        {inviteMsg && <p style={{ color: "var(--color-success)" }}>{inviteMsg}</p>}
        <button type="submit" disabled={busy} className="btn-primary self-start">
          {busy ? "Mengirim…" : "Kirim undangan"}
        </button>
      </form>
    </Section>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mt-6">
      <h2 className="mb-2 font-semibold">{title}</h2>
      {children}
    </section>
  );
}
