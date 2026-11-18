// Halaman daftar paket soal: cari, buat, duplikat, arsip, buka editor.
import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { ApiError, archiveSet, createSet, duplicateSet, listSets, publishSet, unpublishSet } from "./api";
import type { QuestionSet } from "./api";
import { errorOf } from "./api";

export default function QuestionSetsPage() {
  const [sets, setSets] = useState<QuestionSet[] | null>(null);
  const [q, setQ] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({ subject_name: "", title: "", description: "" });

  const load = useCallback(async (search: string) => {
    setBusy(true);
    try {
      setSets(await listSets(search));
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Gagal memuat paket soal");
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    void load("");
  }, [load]);

  const submitCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      // Kode ujian digenerate di server saat paket dibuat (FR-4.1).
      await createSet(form);
      setError(null);
      setCreating(false);
      setForm({ subject_name: "", title: "", description: "" });
      await load(q);
    } catch (err) {
      setError(errorOf(err));
    } finally {
      setBusy(false);
    }
  };

  const onDuplicate = async (id: string) => {
    setBusy(true);
    try {
      await duplicateSet(id);
      await load(q);
    } catch (err) {
      setError(errorOf(err));
    } finally {
      setBusy(false);
    }
  };

  const onPublish = async (id: string) => {
    if (!window.confirm("Publikasikan paket ini? Setelah terbit, soal dibekukan dan perubahan berikutnya membuat versi baru.")) return;
    setBusy(true);
    try {
      await publishSet(id);
      await load(q);
    } catch (err) {
      setError(errorOf(err));
    } finally {
      setBusy(false);
    }
  };

  const onUnpublish = async (id: string) => {
    if (!window.confirm("Tarik publikasi? Paket kembali ke draft. Gagal bila ujian masih memakai versi terbit.")) return;
    setBusy(true);
    try {
      await unpublishSet(id);
      await load(q);
    } catch (err) {
      setError(errorOf(err));
    } finally {
      setBusy(false);
    }
  };

  const onArchive = async (id: string) => {
    if (!window.confirm("Arsipkan paket ini?")) return;
    setBusy(true);
    try {
      await archiveSet(id);
      await load(q);
    } catch (err) {
      setError(errorOf(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mx-auto max-w-3xl">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Bank Soal</h1>
        <div className="flex gap-2">
          <input
            placeholder="Cari judul / mapel…"
            value={q}
            onChange={(e) => { setQ(e.target.value); void load(e.target.value); }}
            className="input"
          />
          <button type="button" className="btn-primary" onClick={() => setCreating(true)} disabled={busy}>
            + Paket
          </button>
        </div>
      </div>

      {error && <p role="alert" className="mt-3" style={{ color: "var(--color-danger)" }}>{error}</p>}

      {creating && (
        <form onSubmit={submitCreate} className="mt-4 rounded-lg border p-4"
              style={{ borderColor: "var(--color-border)" }}>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <label className="flex flex-col gap-1 text-sm">
              <span>Mata pelajaran</span>
              <input required value={form.subject_name} onChange={(e) => setForm({ ...form, subject_name: e.target.value })} className="input" />
            </label>
            <label className="flex flex-col gap-1 text-sm">
              <span>Judul paket</span>
              <input required value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} className="input" />
            </label>
            <label className="flex flex-col gap-1 text-sm sm:col-span-2">
              <span>Deskripsi (opsional)</span>
              <input value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} className="input" />
            </label>
          </div>
          <div className="mt-3 flex gap-2">
            <button type="submit" className="btn-primary" disabled={busy}>
              {busy ? "Menyimpan…" : "Buat paket (draft)"}
            </button>
            <button type="button" className="btn-ghost" onClick={() => setCreating(false)}>
              Batal
            </button>
          </div>
        </form>
      )}

      <div className="mt-4 flex flex-col gap-3">
        {sets === null ? (
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>Memuat…</p>
        ) : sets.length === 0 ? (
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            Belum ada paket. Klik "+ Paket" untuk membuat yang pertama.
          </p>
        ) : (
          sets.map((s) => (
            <div key={s.id} className="rounded-lg border p-4" style={{ borderColor: "var(--color-border)", opacity: s.status === "archived" ? 0.6 : 1 }}>
              <div className="flex items-start justify-between gap-3">
                <div>
                  <Link to={`/question-sets/${s.id}`} className="font-medium">
                    {s.title}
                  </Link>
                  <div className="text-sm" style={{ color: "var(--color-text-muted)" }}>
                    {s.subject_name} · {s.question_count ?? 0} soal · maks {s.max_points ?? 0} poin · {s.exam_code}
                  </div>
                </div>
                <div className="flex shrink-0 gap-2">
                  {s.status === "draft" && (
                    <button type="button" className="btn-ghost touch-target" onClick={() => void onPublish(s.id)} disabled={busy}>
                      Publish
                    </button>
                  )}
                  {s.status === "published" && (
                    <button type="button" className="btn-ghost touch-target" onClick={() => void onUnpublish(s.id)} disabled={busy}>
                      Tarik publikasi
                    </button>
                  )}
                  <button type="button" className="btn-ghost touch-target" onClick={() => void onDuplicate(s.id)} disabled={busy}>
                    Duplikat
                  </button>
                  <button type="button" className="btn-ghost touch-target" onClick={() => void onArchive(s.id)} disabled={busy || s.status !== "draft"}>
                    Arsipkan
                  </button>
                </div>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
