import { useCallback, useEffect, FormEvent, useState } from "react";
import { ApiError, api } from "../../lib/api";
import { Field, inputCls } from "../auth/LoginPage";

// FR-3.1: CRUD kelas + arsip.
interface ClassItem {
  id: string;
  name: string;
  academic_year: string;
  student_count: number;
  archived: boolean;
}

export default function ClassesPage() {
  const [classes, setClasses] = useState<ClassItem[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState({ name: "", academic_year: "" });
  const [busy, setBusy] = useState(false);
  const [showArchived, setShowArchived] = useState(false);

  const load = useCallback(async (archived: boolean) => {
    try {
      setClasses(await api<ClassItem[]>(`/classes${archived ? "?archived=1" : ""}`));
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Gagal memuat kelas");
    }
  }, []);

  useEffect(() => {
    void load(showArchived);
  }, [load, showArchived]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api("/classes", { method: "POST", body: form });
      setForm({ name: "", academic_year: "" });
      await load(showArchived);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal membuat kelas");
    } finally {
      setBusy(false);
    }
  };

  const rename = async (c: ClassItem) => {
    const name = window.prompt("Nama baru:", c.name);
    if (!name || name === c.name) return;
    try {
      await api(`/classes/${c.id}`, { method: "PUT", body: { name, academic_year: c.academic_year } });
      await load(showArchived);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal mengubah kelas");
    }
  };

  const archive = async (c: ClassItem) => {
    if (!window.confirm(`Arsipkan kelas "${c.name}"?`)) return;
    try {
      await api(`/classes/${c.id}`, { method: "DELETE" });
      await load(showArchived);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal mengarsipkan kelas");
    }
  };

  return (
    <div className="mx-auto max-w-2xl">
      <h1 className="text-xl font-semibold">Kelas</h1>

      <form onSubmit={submit} className="mt-4 flex flex-col gap-3 rounded-lg border p-4 sm:flex-row sm:items-end"
            style={{ borderColor: "var(--color-border)" }}>
        <div className="flex-1">
          <Field label="Nama kelas (mis. X IPA 1)">
            <input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className={inputCls} />
          </Field>
        </div>
        <div className="w-full sm:w-40">
          <Field label="Tahun ajaran (opsional)">
            <input placeholder="2026/2027" value={form.academic_year} onChange={(e) => setForm({ ...form, academic_year: e.target.value })} className={inputCls} />
          </Field>
        </div>
        <button type="submit" disabled={busy} className="btn-primary">
          {busy ? "Menyimpan…" : "Tambah kelas"}
        </button>
      </form>

      {error && <p role="alert" className="mt-3" style={{ color: "var(--color-danger)" }}>{error}</p>}

      <label className="mt-4 flex items-center gap-2 text-sm">
        <input type="checkbox" checked={showArchived} onChange={(e) => setShowArchived(e.target.checked)} />
        Tampilkan arsip
      </label>

      <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
        {(classes ?? []).map((c) => (
          <div key={c.id} className="rounded-lg border p-4" style={{ borderColor: "var(--color-border)", opacity: c.archived ? 0.6 : 1 }}>
            <div className="flex items-start justify-between">
              <div>
                <div className="font-medium">{c.name}</div>
                <div className="text-sm" style={{ color: "var(--color-text-muted)" }}>
                  {c.academic_year} · {c.student_count} siswa{c.archived ? " · terarsip" : ""}
                </div>
              </div>
            </div>
            <div className="mt-3 flex gap-2">
              <button type="button" onClick={() => rename(c)} disabled={c.archived}
                      className="touch-target rounded-full border px-3 py-1 text-sm" style={{ borderColor: "var(--color-border)" }}>
                Ubah nama
              </button>
              {!c.archived && (
                <button type="button" onClick={() => archive(c)}
                        className="touch-target rounded-full border px-3 py-1 text-sm" style={{ borderColor: "var(--color-border)" }}>
                  Arsipkan
                </button>
              )}
            </div>
          </div>
        ))}
        {classes !== null && classes.length === 0 && (
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            Belum ada kelas. Tambahkan kelas pertama di atas.
          </p>
        )}
      </div>
    </div>
  );
}
