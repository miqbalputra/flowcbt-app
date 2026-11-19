import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { errorOf, gradingQueue, type GradingQueueItem } from "./api";

/** Daftar attempt yang perlu dikoreksi guru untuk satu ujian (S14). */
export default function GradingQueuePage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [items, setItems] = useState<GradingQueueItem[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    gradingQueue(id)
      .then((r) => alive && setItems(r))
      .catch((e) => alive && setError(errorOf(e)));
    return () => { alive = false; };
  }, [id]);

  return (
    <section className="flex flex-col gap-3">
      <header className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Antrean Koreksi</h1>
        <button type="button" className="btn-ghost" onClick={() => navigate(-1)}>Kembali</button>
      </header>
      {error && <p role="alert" className="text-sm" style={{ color: "var(--color-danger)" }}>{error}</p>}
      {items === null ? (
        <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>Memuat…</p>
      ) : items.length === 0 ? (
        <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
          Belum ada jawaban yang menunggu koreksi.
        </p>
      ) : (
        <ul className="flex flex-col gap-2" aria-label="Antrean koreksi">
          {items.map((it) => (
            <li key={it.attempt_id} className="rounded-lg border p-4" style={{ borderColor: "var(--color-border)" }}>
              <div className="flex items-center justify-between gap-2">
                <button
                  type="button"
                  className="font-medium underline-offset-2 hover:underline"
                  onClick={() => navigate(`/grading/${it.attempt_id}`)}
                >
                  {it.student_name} · percobaan {it.attempt_no}
                </button>
                <span className="text-sm" style={{ color: "var(--color-text-muted)" }}>
                  {it.status === "awaiting_manual_grading"
                    ? `${it.needs_grade} soal menunggu nilai`
                    : it.status === "graded" ? "Selesai dinilai" : "Menunggu koreksi"}
                </span>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
