import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { errorOf, gradingDetail, setAnswerScore, regrade, releaseScores, type GradingDetail } from "./api";

/** Koreksi per attempt: rubrik, input nilai, komentar, override nilai otomatis. */
export default function GradingDetailPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [detail, setDetail] = useState<GradingDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [scores, setScores] = useState<Record<string, string>>({});
  const [comments, setComments] = useState<Record<string, string>>({});

  const load = useCallback(() => {
    let alive = true;
    gradingDetail(id)
      .then((d) => { if (alive) setDetail(d); })
      .catch((e) => { if (alive) setError(errorOf(e)); });
    return () => { alive = false; };
  }, [id]);

  useEffect(() => load(), [load]);

  const submitOne = async (qid: string) => {
    const score = Number(scores[qid] ?? "");
    if (!Number.isFinite(score) || Number.isNaN(score)) {
      setError("Nilai harus berupa angka.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const d = await setAnswerScore(id, qid, score, comments[qid]);
      setDetail(d);
    } catch (e) {
      setError(errorOf(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="flex flex-col gap-4">
      <header className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Koreksi Jawaban</h1>
        <button type="button" className="btn-ghost" onClick={() => navigate(-1)}>Kembali</button>
      </header>
      {error && <p role="alert" className="text-sm" style={{ color: "var(--color-danger)" }}>{error}</p>}
      {detail === null ? (
        <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>Memuat…</p>
      ) : (
        <article className="rounded-lg border p-4" style={{ borderColor: "var(--color-border)" }}>
          <h2 className="font-medium">{detail.student_name}</h2>
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            Status: {detail.status === "graded" ? "Selesai dinilai" : detail.status} ·
            {" "}Nilai — status {detail.provisional ? "Sementara" : "Akhir"}: {detail.final_score} / {detail.max_score}
          </p>
          <ul className="mt-3 flex flex-col gap-4">
            {detail.questions.map((q) => (
              <li key={q.question_id} className="rounded-md border p-3" style={{ borderColor: "var(--color-border)" }}>
                <div className="flex items-baseline justify-between gap-2">
                  <p className="text-sm font-medium">Soal {q.order_no} · {q.type} · {q.points} poin</p>
                  {q.auto_score != null && (
                    <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
                      Otomatis: {q.auto_score} {q.override_reason ? `· di-override (${q.override_reason})` : ""}
                    </p>
                  )}
                </div>
                <p className="mt-1 text-sm">{q.prompt}</p>
                {q.rubric && (
                  <p className="mt-1 text-xs" style={{ color: "var(--color-text-muted)" }}>
                    Rubrik: {q.rubric}
                  </p>
                )}
                <div className="mt-2 flex items-center gap-2">
                  <label htmlFor={`score-${q.question_id}`} className="text-sm">Nilai:</label>
                  <input
                    id={`score-${q.question_id}`}
                    type="number"
                    min={0}
                    max={q.points}
                    step="0.5"
                    value={scores[q.question_id] ?? (q.score != null ? String(q.score) : "")}
                    onChange={(e) => setScores((s) => ({ ...s, [q.question_id]: e.target.value }))}
                    className="w-24 rounded-md border px-2 py-1 text-sm"
                    style={{ borderColor: "var(--color-border)" }}
                    disabled={busy}
                  />
                  <button
                    type="button"
                    className="btn-primary touch-target"
                    disabled={busy || scores[q.question_id] === undefined && q.score != null}
                    onClick={() => void submitOne(q.question_id)}
                  >
                    Simpan Nilai
                  </button>
                </div>
                <input
                  type="text"
                  placeholder="Komentar (opsional)"
                  value={comments[q.question_id] ?? ""}
                  onChange={(e) => setComments((c) => ({ ...c, [q.question_id]: e.target.value }))}
                  className="mt-2 w-full rounded-md border px-2 py-1 text-sm"
                  style={{ borderColor: "var(--color-border)" }}
                  aria-label={`Komentar soal ${q.order_no}`}
                  disabled={busy}
                />
              </li>
            ))}
          </ul>
          <div className="mt-4 flex flex-wrap gap-2">
            <button
              type="button"
              className="btn-ghost touch-target"
          
              onClick={() => {
                setBusy(true);
                regrade(id)
                  .then((r) => window.alert(`Nilai ulang pada ${r.regraded} soal.`))
                  .then(load)
                  .catch((e) => setError(errorOf(e)))
                  .finally(() => setBusy(false));
              }}
            >
              Nilai Ulang Soal Objektif
            </button>
            {detail.status !== "graded" && (
              <button
                type="button"
                className="btn-primary touch-target"
                disabled={busy}
                onClick={() => {
                  setBusy(true);
                  releaseScores(detail.exam_id)
                    .then(() => window.alert("Nilai dirilis — siswa dapat melihat hasilnya."))
                    .then(load)
                    .catch((e) => setError(errorOf(e)))
                    .finally(() => setBusy(false));
                }}
              >
                Rilis Nilai
              </button>
            )}
          </div>
        </article>
      )}
    </section>
  );
}
