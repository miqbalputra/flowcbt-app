import { useCallback, useEffect, FormEvent, useState } from "react";
import { ApiError, api, getAccessToken } from "../../lib/api";
import { Field, inputCls } from "../auth/LoginPage";

// FR-3.2/3.4/3.5: daftar siswa (tabel desktop, kartu mobile), tambah siswa
// dengan kredensial otomatis, pindah kelas massal, arsip, search.
interface StudentItem {
  id: string;
  student_code: string;
  name: string;
  gender: "L" | "P";
  class_id?: string;
  class_name?: string;
  username?: string;
  nis?: string;
  status: string;
}

interface ClassItem {
  id: string;
  name: string;
}

interface NewCredentials {
  username: string;
  password: string;
  student_code: string;
}

export default function StudentsPage() {
  const [students, setStudents] = useState<StudentItem[] | null>(null);
  const [classes, setClasses] = useState<ClassItem[]>([]);
  const [filterClass, setFilterClass] = useState("");
  const [search, setSearch] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [newCred, setNewCred] = useState<NewCredentials | null>(null);

  const load = useCallback(async () => {
    try {
      const qs = new URLSearchParams();
      if (filterClass) qs.set("class_id", filterClass);
      if (search) qs.set("q", search);
      setStudents(await api<StudentItem[]>(`/students?${qs.toString()}`));
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Gagal memuat siswa");
    }
  }, [filterClass, search]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    api<ClassItem[]>("/classes").then(setClasses).catch(() => setClasses([]));
  }, []);

  const toggle = (id: string) =>
    setSelected((s) => {
      const next = new Set(s);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  return (
    <div className="mx-auto max-w-3xl">
      <h1 className="text-xl font-semibold">Siswa</h1>

      <div className="mt-4 flex flex-col gap-2 sm:flex-row">
        <input
          placeholder="Cari nama…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className={`${inputCls} sm:max-w-xs`}
        />
        <select value={filterClass} onChange={(e) => setFilterClass(e.target.value)} className={`${inputCls} sm:max-w-xs`}>
          <option value="">Semua kelas</option>
          {classes.map((c) => (
            <option key={c.id} value={c.id}>{c.name}</option>
          ))}
        </select>
      </div>

      {error && <p role="alert" className="mt-3" style={{ color: "var(--color-danger)" }}>{error}</p>}

      <AddStudentForm classes={classes} onCreated={(cred) => { setNewCred(cred); void load(); }} />
      {newCred && (
        <div className="mt-3 rounded-lg border p-4" style={{ borderColor: "var(--color-success)" }}>
          <div className="font-semibold" style={{ color: "var(--color-success)" }}>Siswa dibuat</div>
          <p className="mt-1 text-sm">Kartu login (tampilkan sekali — tab untuk salin):</p>
          <pre className="mt-1 rounded p-2 text-sm" style={{ background: "var(--color-bg-muted)" }}>
            {`Kode: ${newCred.student_code}\nUsername: ${newCred.username}\nPassword: ${newCred.password}`}
          </pre>
        </div>
      )}

      {selected.size > 0 && <MoveClassBar classList={classes} selected={[...selected]} onDone={() => { setSelected(new Set()); void load(); }} />}
      <ImportWizard onDone={load} />

      {/* Desktop: tabel */}
      <div className="mt-4 hidden md:block">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left" style={{ color: "var(--color-text-muted)" }}>
              <th className="p-2"></th>
              <th className="p-2">Nama</th>
              <th className="p-2">Kelas</th>
              <th className="p-2">NIS</th>
              <th className="p-2">JK</th>
              <th className="p-2">Username</th>
              <th className="p-2"></th>
            </tr>
          </thead>
          <tbody>
            {(students ?? []).map((s) => (
              <tr key={s.id} className="border-t" style={{ borderColor: "var(--color-border)" }}>
                <td className="p-2">
                  <input type="checkbox" aria-label={`pilih ${s.name}`} checked={selected.has(s.id)} onChange={() => toggle(s.id)} />
                </td>
                <td className="p-2 font-medium">{s.name}</td>
                <td className="p-2">{s.class_name ?? "—"}</td>
                <td className="p-2">{s.nis ?? "—"}</td>
                <td className="p-2">{s.gender}</td>
                <td className="p-2">{s.username ?? "—"}</td>
                <td className="p-2">
                  <button
                    type="button"
                    className="touch-target rounded-full border px-3 py-1 text-xs"
                    style={{ borderColor: "var(--color-border)" }}
                    onClick={() => archiveStudent(s, setError, load)}
                  >
                    Arsipkan
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Mobile: kartu */}
      <div className="mt-4 flex flex-col gap-3 md:hidden">
        {(students ?? []).map((s) => (
          <div key={s.id} className="rounded-lg border p-4" style={{ borderColor: "var(--color-border)" }}>
            <div className="flex items-center justify-between">
              <label className="flex items-center gap-2 font-medium">
                <input type="checkbox" aria-label={`pilih ${s.name}`} checked={selected.has(s.id)} onChange={() => toggle(s.id)} />
                {s.name}
              </label>
              <span className="text-sm">{s.gender === "L" ? "Laki-laki" : "Perempuan"}</span>
            </div>
            <div className="mt-1 text-sm" style={{ color: "var(--color-text-muted)" }}>
              {s.class_name ?? "Tanpa kelas"} · NIS {s.nis ?? "—"} · {s.student_code}
            </div>
            <button
              type="button"
              className="touch-target mt-3 rounded-full border px-3 py-1 text-sm"
              style={{ borderColor: "var(--color-border)" }}
              onClick={() => archiveStudent(s, setError, load)}
            >
              Arsipkan
            </button>
          </div>
        ))}
      </div>

      {students !== null && students.length === 0 && (
        <p className="mt-4 text-sm" style={{ color: "var(--color-text-muted)" }}>
          Belum ada siswa yang cocok.
        </p>
      )}
    </div>
  );
}

function archiveStudent(s: StudentItem, setError: (m: string | null) => void, reload: () => Promise<void>) {
  if (!window.confirm(`Arsipkan siswa "${s.name}"?`)) return;
  api(`/students/${s.id}`, { method: "DELETE" })
    .then(() => void reload())
    .catch((e: unknown) => setError(e instanceof ApiError ? e.message : "Gagal mengarsipkan siswa"));
}

function AddStudentForm({ classes, onCreated }: { classes: ClassItem[]; onCreated: (c: NewCredentials) => void }) {
  const [form, setForm] = useState({ name: "", gender: "L", class_id: "", nis: "" });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await api<{ student: StudentItem; credentials: NewCredentials }>("/students", {
        method: "POST",
        body: form,
      });
      setForm({ name: "", gender: "L", class_id: "", nis: "" });
      onCreated(res.credentials);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal menambah siswa");
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} className="mt-4 rounded-lg border p-4" style={{ borderColor: "var(--color-border)" }}>
      {!open ? (
        <button type="button" className="btn-primary" onClick={() => setOpen(true)}>
          + Tambah siswa
        </button>
      ) : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Field label="Nama">
            <input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className={inputCls} />
          </Field>
          <Field label="Jenis kelamin">
            <select value={form.gender} onChange={(e) => setForm({ ...form, gender: e.target.value })} className={inputCls}>
              <option value="L">Laki-laki</option>
              <option value="P">Perempuan</option>
            </select>
          </Field>
          <Field label="Kelas">
            <select value={form.class_id} onChange={(e) => setForm({ ...form, class_id: e.target.value })} className={inputCls}>
              <option value="">— tanpa kelas —</option>
              {classes.map((c) => (
                <option key={c.id} value={c.id}>{c.name}</option>
              ))}
            </select>
          </Field>
          <Field label="NIS (opsional)">
            <input value={form.nis} onChange={(e) => setForm({ ...form, nis: e.target.value })} className={inputCls} />
          </Field>
          {error && <p role="alert" className="sm:col-span-2" style={{ color: "var(--color-danger)" }}>{error}</p>}
          <div className="flex gap-2 sm:col-span-2">
            <button type="submit" disabled={busy} className="btn-primary">
              {busy ? "Menyimpan…" : "Simpan (username & password otomatis)"}
            </button>
            <button type="button" className="touch-target rounded-full border px-4 text-sm" style={{ borderColor: "var(--color-border)" }} onClick={() => setOpen(false)}>
              Batal
            </button>
          </div>
        </div>
      )}
    </form>
  );
}

function MoveClassBar({ classList, selected, onDone }: { classList: ClassItem[]; selected: string[]; onDone: () => void }) {
  const [target, setTarget] = useState("");
  const [busy, setBusy] = useState(false);

  const move = async () => {
    if (!target) return;
    setBusy(true);
    try {
      await api("/students/move-class", { method: "POST", body: { student_ids: selected, class_id: target } });
      onDone();
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="sticky bottom-20 md:bottom-4 mt-3 flex items-center gap-2 rounded-lg border p-3"
         style={{ borderColor: "var(--color-primary)", background: "var(--color-surface)" }}>
      <span className="text-sm font-medium">{selected.length} dipilih</span>
      <select value={target} onChange={(e) => setTarget(e.target.value)} className={inputCls}>
        <option value="">Pindah ke kelas…</option>
        {classList.map((c) => (
          <option key={c.id} value={c.id}>{c.name}</option>
        ))}
      </select>
      <button type="button" onClick={move} disabled={busy || !target} className="btn-primary">
        {busy ? "Memindah…" : "Pindahkan"}
      </button>
      <button type="button" onClick={onDone} className="touch-target rounded-full border px-3 text-sm" style={{ borderColor: "var(--color-border)" }}>
        Batal
      </button>
    </div>
  );
}

// ---- S5: Wizard import 3 langkah (template → preview → commit), AC-10 ----

interface ImportErrors {
  row: number;
  name?: string;
  reason: string;
}
interface ImportPreviewRes {
  total: number;
  valid: { row: number; name: string; gender: string; class: string; nis?: string }[];
  errors: ImportErrors[];
  limit: number;
  remaining: number;
}
interface ImportCommitRes {
  imported: number;
  credentials: { username: string; password: string; student_code: string }[];
  classes_created?: string[];
}

export function ImportWizard({ onDone }: { onDone: () => void }) {
  const [open, setOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [autoCreate, setAutoCreate] = useState(true);
  const [preview, setPreview] = useState<ImportPreviewRes | null>(null);
  const [result, setResult] = useState<ImportCommitRes | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const downloadTemplate = async () => {
    const res = await fetch("/api/v1/students/import/template", {
      headers: { Authorization: `Bearer ${getAccessToken() ?? ""}` },
    });
    const blob = await res.blob();
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "template-import-siswa.xlsx";
    a.click();
    URL.revokeObjectURL(a.href);
  };

  const runPreview = async () => {
    if (!file) return;
    setBusy(true);
    setError(null);
    try {
      const fd = new FormData();
      fd.append("file", file);
      setPreview(await api<ImportPreviewRes>("/students/import/preview", { method: "POST", body: fd }));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Gagal memvalidasi file");
    } finally {
      setBusy(false);
    }
  };

  const runCommit = async () => {
    if (!file) return;
    setBusy(true);
    setError(null);
    try {
      const fd = new FormData();
      fd.append("file", file);
      fd.append("auto_create_class", autoCreate ? "1" : "0");
      const res = await api<ImportCommitRes>("/students/import/commit", { method: "POST", body: fd });
      setResult(res);
      setPreview(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Gagal menyimpan import");
    } finally {
      setBusy(false);
    }
  };

  const exportStudents = async (format: "xlsx" | "csv") => {
    const res = await fetch(`/api/v1/students/export?format=${format}`, {
      headers: { Authorization: `Bearer ${getAccessToken() ?? ""}` },
    });
    const blob = await res.blob();
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = `siswa.${format}`;
    a.click();
    URL.revokeObjectURL(a.href);
  };

  return (
    <div className="mt-3 flex flex-wrap items-center gap-2">
      <button type="button" className="btn-primary" onClick={() => setOpen(true)}>
        Import siswa
      </button>
      <button type="button" className="touch-target rounded-full border px-4 py-1.5 text-sm" style={{ borderColor: "var(--color-border)" }} onClick={() => void exportStudents("xlsx")}>
        Export xlsx
      </button>
      <button type="button" className="touch-target rounded-full border px-4 py-1.5 text-sm" style={{ borderColor: "var(--color-border)" }} onClick={() => void exportStudents("csv")}>
        Export csv
      </button>

      {open && (
        <div className="fixed inset-0 z-20 flex items-end justify-center sm:items-center" style={{ background: "rgba(0,0,0,.4)" }} onClick={() => !busy && setOpen(false)}>
          <div className="w-full max-w-xl rounded-t-2xl border p-5 sm:rounded-2xl" style={{ background: "var(--color-surface)", borderColor: "var(--color-border)" }} onClick={(e) => e.stopPropagation()}>
            {!result ? (
              <>
                <div className="mb-3 flex items-center justify-between">
                  <h2 className="font-semibold">Import siswa</h2>
                  <button type="button" onClick={() => void downloadTemplate()} className="touch-target rounded-full border px-3 py-1 text-sm" style={{ borderColor: "var(--color-border)" }}>
                    Unduh template
                  </button>
                </div>
                <input type="file" accept=".xlsx,.csv" onChange={(e) => setFile(e.target.files?.[0] ?? null)} className="mb-3" />
                <label className="mb-3 flex items-center gap-2 text-sm">
                  <input type="checkbox" checked={autoCreate} onChange={(e) => setAutoCreate(e.target.checked)} />
                  Buat kelas otomatis bila belum ada
                </label>

                {error && <p role="alert" className="mb-3" style={{ color: "var(--color-danger)" }}>{error}</p>}

                {preview && (
                  <div className="mb-3 max-h-72 overflow-auto rounded-lg border p-3 text-sm" style={{ borderColor: "var(--color-border)" }}>
                    <p>
                      <b>{preview.valid.length}</b> baris valid dari {preview.total} · sisa kuota {preview.remaining}/{preview.limit}
                      {preview.errors.length > 0 && (
                        <> · <span style={{ color: "var(--color-danger)" }}><b>{preview.errors.length}</b> error</span></>
                      )}
                    </p>
                    <ul className="mt-2 flex flex-col gap-1">
                      {preview.errors.map((e2, i) => (
                        <li key={i} style={{ color: "var(--color-danger)" }}>
                          Baris {e2.row} {e2.name && `(${e2.name})`}: {e2.reason}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}

                <div className="flex gap-2">
                  <button type="button" onClick={runPreview} disabled={busy || !file} className="btn-primary">
                    {busy ? "Memeriksa…" : "1. Periksa file"}
                  </button>
                  {preview && preview.valid.length > 0 && (
                    <button type="button" onClick={runCommit} disabled={busy} className="btn-primary">
                      2. Simpan {preview.valid.length} siswa
                    </button>
                  )}
                  <button type="button" onClick={() => { setOpen(false); setPreview(null); setResult(null); }} className="touch-target rounded-full border px-4 py-1.5 text-sm" style={{ borderColor: "var(--color-border)" }}>
                    Tutup
                  </button>
                </div>
              </>
            ) : (
              <div className="flex flex-col gap-3">
                <p style={{ color: "var(--color-success)" }}>
                  Import selesai: {result.imported} siswa dibuat.
                  {result.classes_created && result.classes_created.length > 0 &&
                    ` Kelas baru: ${result.classes_created.join(", ")}.`}
                </p>
                <details>
                  <summary className="cursor-pointer text-sm font-medium">Kartu login (cetak / simpan)</summary>
                  <pre className="mt-2 max-h-64 overflow-auto rounded p-2 text-xs" style={{ background: "var(--color-bg-muted)" }}>
                    {result.credentials.map((c) => `${c.student_code}\t${c.username}\t${c.password}`).join("\n")}
                  </pre>
                </details>
                <button type="button" onClick={() => { setOpen(false); setResult(null); onDone(); }} className="btn-primary self-start">
                  Selesai
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
