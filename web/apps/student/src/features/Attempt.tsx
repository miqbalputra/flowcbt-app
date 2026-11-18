// Pengerjaan ujian (S12): 5 tipe soal di layar 360 px, timer sticky,
// Sebelumnya/Berikutnya/Ragu-ragu, target sentuh ≥ 44 px, sync 800 ms.
// Offline penuh & auto-submit menyusul S13/S15–S18.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  api, errorOf, type ClientQuestion, type StartResult,
} from "../lib/api";
import { Shell } from "./StudentPages";

type AnswerMap = Record<string, { answer: unknown; flagged: boolean }>;

interface AttemptState extends StartResult {
  status: string;
  answers: Record<string, { answer?: unknown; flagged: boolean }>;
}

function loadState(id: string): AttemptState | null {
  try {
    const fromSession = sessionStorage.getItem("flowcbt:attempt");
    if (fromSession) {
      const s = JSON.parse(fromSession) as StartResult;
      if (s.attempt_id === id) {
        return { ...s, status: "in_progress", answers: {} };
      }
    }
  } catch {
    // abaikan; fallback ke server
  }
  return null;
}

export default function AttemptPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [st, setSt] = useState<AttemptState | null>(() => loadState(id));
  const [answers, setAnswers] = useState<AnswerMap>({});
  const [idx, setIdx] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [timeLeft, setTimeLeft] = useState<number | null>(null);

  // Offset jam klien vs server (deadline otoritas server).
  const clockOffset = useRef(0);
  const seq = useRef(0);
  const queue = useRef<unknown[]>([]);
  const syncTimer = useRef<number | null>(null);

  useEffect(() => {
    void (async () => {
      try {
        const fresh = await api<AttemptState>(`/attempts/${id}`);
        clockOffset.current = Date.parse(fresh.server_time) - Date.now();
        setSt(fresh);
        setAnswers(Object.fromEntries(
          Object.entries(fresh.answers ?? {}).map(([qid, a]) => [qid, { answer: a.answer, flagged: a.flagged }]),
        ));
      } catch (e) {
        // Sesi dari sessionStorage bisa sudah usang — coba lanjut via start ulang.
        setError(errorOf(e));
      }
    })();
  }, [id]);

  const deadline = useMemo(() => {
    if (!st?.deadline_at) return null;
    return Date.parse(st.deadline_at) - clockOffset.current;
  }, [st?.deadline_at]);

  useEffect(() => {
    if (deadline === null) return;
    const t = window.setInterval(() => {
      setTimeLeft(Math.max(0, Math.floor((deadline - Date.now()) / 1000)));
    }, 250);
    return () => window.clearInterval(t);
  }, [deadline]);

  // ---- sync (S12 dasar: online, debounce 800 ms) ----
  const flush = useCallback(async () => {
    if (!st || queue.current.length === 0) return;
    const events = queue.current.splice(0);
    seq.current += 1;
    try {
      await api(`/attempts/${st.attempt_id}/answers:sync`, {
        method: "POST",
        body: { seq: seq.current, events },
      });
    } catch {
      // gagal: kembalikan ke depan antrean (attempt ulang tick berikutnya)
      queue.current = [...events, ...queue.current];
    }
  }, [st]);

  const scheduleSync = useCallback(() => {
    if (syncTimer.current) window.clearTimeout(syncTimer.current);
    syncTimer.current = window.setTimeout(() => void flush(), 800);
  }, [flush]);

  useEffect(() => () => {
    if (syncTimer.current) window.clearTimeout(syncTimer.current);
  }, []);

  const push = useCallback((ev: Record<string, unknown>) => {
    queue.current.push({
      client_event_id: crypto.randomUUID(),
      elapsed_ms: 0,
      ...ev,
    });
    scheduleSync();
  }, [scheduleSync]);

  const setAnswer = (qid: string, answer: unknown) => {
    setAnswers((prev) => ({ ...prev, [qid]: { answer, flagged: prev[qid]?.flagged ?? false } }));
    push({ type: "answer", question_id: qid, answer });
  };

  const toggleFlag = (qid: string) => {
    setAnswers((prev) => {
      const flagged = !(prev[qid]?.flagged ?? false);
      push({ type: "flag", question_id: qid, flagged });
      return { ...prev, [qid]: { answer: prev[qid]?.answer, flagged } };
    });
  };

  if (!st) {
    return (
      <Shell title="Pengerjaan">
        <h1 className="text-xl font-semibold">{error ?? "Memuat sesi…"}</h1>
        {error && (
          <button type="button" className="btn-primary mt-4 touch-target" onClick={() => navigate("/exams")}>
            Kembali ke daftar ujian
          </button>
        )}
      </Shell>
    );
  }

  const qs = st.questions;
  const q = qs[idx];
  const a = answers[q.id]?.answer;
  const flagged = answers[q.id]?.flagged ?? false;
  const answeredCount = qs.filter((x) => answers[x.id]?.answer !== undefined && answers[x.id]?.answer !== null).length;
  const timeCritical = timeLeft !== null && timeLeft <= 300;

  return (
    <div className="mx-auto flex min-h-dvh w-full max-w-[480px] flex-col">
      {/* Header timer sticky (PRD 11.2) */}
      <header className="sticky top-0 z-10 border-b px-4 py-2"
              style={{ borderColor: "var(--color-border)", background: "var(--color-bg)" }}>
        <div className="flex items-center justify-between gap-2">
          <span className="truncate text-sm font-medium">{st.exam_name}</span>
          <span data-role="timer" className="rounded px-2 py-1 text-sm font-bold"
                style={{ background: timeCritical ? "var(--color-danger, #b91c1c)" : "var(--color-bg-muted)",
                         color: timeCritical ? "#fff" : "inherit" }}>
            {timeLeft === null ? "--:--" : fmtTime(timeLeft)}
          </span>
        </div>
        <p className="text-xs text-muted">
          Soal {idx + 1}/{qs.length} · terjawab {answeredCount}
        </p>
      </header>

      <main className="flex-1 px-4 py-4">
        <p className="mb-1 text-xs text-muted">{q.type} · {q.points} poin</p>
        <div className="rich mb-4" dangerouslySetInnerHTML={{ __html: q.body }} />

        {(q.type === "mcq" || q.type === "mcq_complex") && (
          <OptionList q={q} answer={a} onAnswer={(v) => setAnswer(q.id, v)} />
        )}
        {q.type === "matching" && <MatchingAnswer q={q} answer={a} onAnswer={(v) => setAnswer(q.id, v)} />}
        {q.type === "short_answer" && (
          <input className="input w-full" placeholder="Jawaban Anda"
                 value={(a as { text?: string })?.text ?? ""}
                 onChange={(e) => setAnswer(q.id, { text: e.target.value })} />
        )}
        {q.type === "essay" && (
          <textarea className="input min-h-40 w-full" rows={6} placeholder="Tulis jawaban…"
                    value={(a as { text?: string })?.text ?? ""}
                    onChange={(e) => setAnswer(q.id, { text: e.target.value })} />
        )}
      </main>

      {/* Navigasi bawah — semua target ≥ 44 px (touch-target) */}
      <footer className="sticky bottom-0 border-t px-4 py-3"
              style={{ borderColor: "var(--color-border)", background: "var(--color-bg)" }}>
        <div className="flex items-center justify-between gap-2">
          <button type="button" className="btn-ghost touch-target" disabled={idx === 0}
                  onClick={() => { void flush(); setIdx((i) => Math.max(0, i - 1)); }}>
            Sebelumnya
          </button>
          <button type="button" className="btn-ghost touch-target"
                  style={flagged ? { color: "var(--color-warning, #b45309)", fontWeight: 700 } : undefined}
                  onClick={() => toggleFlag(q.id)}>
            {flagged ? "Ragu-ragu" : "Tandai ragu"}
          </button>
          {idx < qs.length - 1 ? (
            <button type="button" className="btn-primary touch-target"
                    onClick={() => { void flush(); setIdx((i) => i + 1); }}>
              Berikutnya
            </button>
          ) : (
            <button type="button" className="btn-primary touch-target" onClick={() => void flush()}>
              Selesai
            </button>
          )}
        </div>
      </footer>
    </div>
  );
}

function OptionList({ q, answer, onAnswer }: {
  q: ClientQuestion; answer: unknown; onAnswer: (v: unknown) => void;
}) {
  const opts = q.options ?? [];
  const selectedOne = (answer as { selected?: string })?.selected;
  const selectedMany = (answer as { selected?: string[] })?.selected ?? [];
  return (
    <div className="flex flex-col gap-2">
      {opts.map((o, i) => {
        const checked = q.type === "mcq" ? selectedOne === o.id : selectedMany.includes(o.id);
        const toggle = () => {
          if (q.type === "mcq") {
            onAnswer({ selected: o.id });
          } else {
            onAnswer({ selected: checked ? selectedMany.filter((x) => x !== o.id) : [...selectedMany, o.id] });
          }
        };
        return (
          <button key={o.id} type="button" onClick={toggle} aria-pressed={checked}
                  className="touch-target flex items-center gap-3 rounded-lg border p-3 text-left"
                  style={{ borderColor: checked ? "var(--color-primary, #2563eb)" : "var(--color-border)",
                           borderWidth: checked ? 2 : 1 }}>
            <span className="flex size-6 shrink-0 items-center justify-center rounded-full border text-xs"
                  style={{ borderColor: "var(--color-border)" }}>
              {String.fromCharCode(65 + i)}
            </span>
            <span className="rich" dangerouslySetInnerHTML={{ __html: o.html }} />
          </button>
        );
      })}
    </div>
  );
}

// Menjodohkan: tap kiri → tap kanan = pasangan (PRD 11 mobile).
function MatchingAnswer({ q, answer, onAnswer }: {
  q: ClientQuestion; answer: unknown; onAnswer: (v: unknown) => void;
}) {
  const left = q.matching?.left ?? [];
  const right = q.matching?.right ?? [];
  const pairs = (answer as { pairs?: { left: string; right: string }[] })?.pairs ?? [];
  const [pickLeft, setPickLeft] = useState<string | null>(null);

  const bind = (rightId: string) => {
    if (!pickLeft) return;
    const next = [...pairs.filter((p) => p.left !== pickLeft && p.right !== rightId), { left: pickLeft, right: rightId }];
    onAnswer({ pairs: next });
    setPickLeft(null);
  };
  const unbind = (leftId: string) => {
    onAnswer({ pairs: pairs.filter((p) => p.left !== leftId) });
  };

  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-2 gap-2">
        <div className="flex flex-col gap-2">
          <span className="text-xs text-muted">Pilih kiri</span>
          {left.map((l) => {
            const active = pickLeft === l.id;
            const bound = pairs.find((p) => p.left === l.id);
            return (
              <button key={l.id} type="button"
                      className="touch-target rounded-lg border p-2 text-left text-sm"
                      style={{ borderColor: active ? "var(--color-primary, #2563eb)" : "var(--color-border)",
                               borderWidth: active ? 2 : 1 }}
                      onClick={() => setPickLeft(active ? null : l.id)}>
                {l.html}
                {bound && <span className="ml-1 text-xs text-muted">→ {right.find((r) => r.id === bound.right)?.html}</span>}
              </button>
            );
          })}
        </div>
        <div className="flex flex-col gap-2">
          <span className="text-xs text-muted">{pickLeft ? "Pilih pasangannya" : "Kanan"}</span>
          {right.map((r) => {
            const taken = pairs.some((p) => p.right === r.id);
            return (
              <button key={r.id} type="button" disabled={!pickLeft || taken}
                      className="touch-target rounded-lg border p-2 text-left text-sm disabled:opacity-40"
                      style={{ borderColor: taken ? "var(--color-border)" : "var(--color-primary, #2563eb)" }}
                      onClick={() => bind(r.id)}>
                {r.html}
              </button>
            );
          })}
        </div>
      </div>
      {pairs.length > 0 && (
        <div className="flex flex-col gap-1 text-sm">
          {pairs.map((p) => (
            <div key={p.left} className="flex items-center gap-2">
              <span>{left.find((l) => l.id === p.left)?.html} → {right.find((r) => r.id === p.right)?.html}</span>
              <button type="button" className="btn-ghost" onClick={() => unbind(p.left)}>Lepas</button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function fmtTime(sec: number): string {
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}
