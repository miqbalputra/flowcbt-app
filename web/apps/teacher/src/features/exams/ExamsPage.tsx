// Halaman daftar ujian (S9): cari, status, lihat wizard/detail.
import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { ApiError, deleteExam, errorOf, listExams, type Exam } from "./api";

const STATUS_LABEL: Record<Exam["status"], string> = {
  draft: "Draft",
  published: "Terjadwal",
  paused: "Dijeda",
  closed: "Selesai",
  cancelled: "Dibatalkan",
};

export default function ExamsPage() {
  const [exams, setExams] = useState<Exam[] | null>(null);
  const [q, setQ] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async (search: string) => {
    setBusy(true);
    try {
      setExams(await listExams(search));
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Gagal memuat ujian");
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    void load("");
  }, [load]);

  const onDelete = async (id: string) => {
    if (!window.confirm("Hapus ujian draft ini?")) return;
    setBusy(true);
    try {
      await deleteExam(id);
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
        <h1 className="text-xl font-semibold">Ujian</h1>
        <div className="flex gap-2">
          <input
            placeholder="Cari nama ujian…"
            value={q}
            onChange={(e) => { setQ(e.target.value); void load(e.target.value); }}
            className="input"
          />
          <Link to="/exams/new" className="btn-primary">+ Ujian</Link>
        </div>
      </div>

      {error && <p role="alert" className="mt-3" style={{ color: "var(--color-danger)" }}>{error}</p>}

      <div className="mt-4 flex flex-col gap-3">
        {exams === null ? (
          <p className="text-sm text-muted">Memuat…</p>
        ) : exams.length === 0 ? (
          <p className="text-sm text-muted">
            Belum ada ujian. Klik "+ Ujian" untuk membuat dari paket soal terbit.
          </p>
        ) : (
          exams.map((x) => (
            <div key={x.id} className="rounded-lg border p-4" style={{ borderColor: "var(--color-border)" }}>
              <div className="flex items-start justify-between gap-3">
                <div>
                  <Link to={`/exams/${x.id}`} className="font-medium">{x.name}</Link>
                  <div className="text-sm text-muted">
                    {STATUS_LABEL[x.status]}
                    {x.open_at && ` · buka ${new Date(x.open_at).toLocaleString("id-ID")}`}
                    {x.close_at && ` · tutup ${new Date(x.close_at).toLocaleString("id-ID")}`}
                    {x.duration_minutes ? ` · ${x.duration_minutes} menit` : ""}
                  </div>
                </div>
                {x.status === "draft" && (
                  <button type="button" className="btn-ghost touch-target" onClick={() => void onDelete(x.id)} disabled={busy}>
                    Hapus
                  </button>
                )}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
