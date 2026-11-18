// Editor paket soal: daftar soal, editor per tipe (FR-4.3), autosave 800 ms
// (FR-4.4), pratinjau siswa. Kunci jawaban TIDAK pernah dikirim ke siswa.
import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "react-router-dom";
import {
  ApiError, commitQuestionImport, createQuestion, deleteQuestion, downloadQuestionTemplate,
  errorOf, exportQuestions, getSet, getSetQuestions, previewQuestionImport, publishSet,
  reorderQuestions, updateQuestion,
  type Option, type Question, type QuestionImportPreview,
} from "./api";
import { MatchingView, OptionsView, QuestionBodyView, ScoringBadge } from "./views";

const ALL_TYPES = [
  { value: "mcq", label: "Pilihan Ganda" },
  { value: "mcq_complex", label: "PG Kompleks" },
  { value: "matching", label: "Menjodohkan" },
  { value: "short_answer", label: "Isian Singkat" },
  { value: "essay", label: "Uraian" },
] as const;

// stripHTML untuk pratinjau ringkas (render penuh tetap lewat React).
const stripHTML = (s: string) => s.replace(/<[^>]*>/g, " ").replace(/\s+/g, " ").trim();

export default function EditorPage() {
  const { id = "" } = useParams();
  const [set, setSet] = useState<Awaited<ReturnType<typeof getSet>> | null>(null);
  const [questions, setQuestions] = useState<Question[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [newType, setNewType] = useState<Question["type"]>("mcq");
  const [importPreview, setImportPreview] = useState<QuestionImportPreview | null>(null);
  const [importFile, setImportFile] = useState<File | null>(null);

  const load = useCallback(async () => {
    try {
      const [s, qs] = await Promise.all([getSet(id), getSetQuestions(id)]);
      setSet(s);
      setQuestions(qs);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Gagal memuat paket");
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!set) return <p className="p-6 text-sm text-muted">Memuat…</p>;

  const locked = set.status !== "draft"; // terbit: soal dibekukan (FR-4.8)

  return (
    <div className="mx-auto max-w-3xl p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">{set.title}</h1>
          <p className="text-sm text-muted">
            {set.subject_name} · {set.exam_code} · {questions.length} soal
            {set.status !== "draft" && " · TERBIT"}
          </p>
        </div>
        <div className="flex shrink-0 gap-2">
          <button type="button" className="btn-ghost touch-target" onClick={() => setPreviewing((v) => !v)}>
            {previewing ? "Editor" : "Pratinjau siswa"}
          </button>
          {!locked && (
            <button type="button" className="btn-primary touch-target" onClick={() => void onPublish()} disabled={saving}>
              {saving ? "Memproses…" : "Publish"}
            </button>
          )}
        </div>
      </div>

      {error && <p role="alert" className="mt-3" style={{ color: "var(--color-danger)" }}>{error}</p>}

      {/* Toolbar import/export template (S8). Import hanya paket draft. */}
      <div className="mt-3 flex flex-wrap gap-2">
        <button type="button" className="btn-ghost touch-target" onClick={() => void downloadQuestionTemplate()}>
          Template
        </button>
        <label className="btn-ghost touch-target" style={{ cursor: locked ? "not-allowed" : "pointer", opacity: locked ? 0.5 : 1 }}>
          Impor
          <input type="file" accept=".xlsx,.csv" className="hidden" disabled={locked}
                 onChange={(e) => void onImportPick(e.target.files?.[0] ?? null)} />
        </label>
        <button type="button" className="btn-ghost touch-target" onClick={() => void exportQuestions(id)}>
          Ekspor XLSX
        </button>
      </div>

      {importPreview && (
        <div className="mt-3 rounded-lg border p-4" style={{ borderColor: "var(--color-border)" }}>
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">
              Pratinjau impor: {importPreview.valid.length} sah / {importPreview.total} baris
            </span>
            <div className="flex gap-2">
              <button type="button" className="btn-primary touch-target"
                      disabled={saving || importPreview.valid.length === 0}
                      onClick={() => void onImportCommit()}>
                {saving ? "Menyimpan…" : `Simpan ${importPreview.valid.length} soal`}
              </button>
              <button type="button" className="btn-ghost touch-target" onClick={() => setImportPreview(null)}>
                Batal
              </button>
            </div>
          </div>
          {importPreview.errors.length > 0 && (
            <ul className="mt-2 text-sm" style={{ color: "var(--color-danger)" }}>
              {importPreview.errors.map((e) => (
                <li key={e.row}>Baris {e.row}: {e.reason}</li>
              ))}
            </ul>
          )}
          <ul className="mt-2 text-sm">
            {importPreview.valid.slice(0, 10).map((q, i) => (
              <li key={i}>
                {q.type} · {q.points} poin · {stripHTML(q.body).slice(0, 80)}
              </li>
            ))}
            {importPreview.valid.length > 10 && (
              <li className="text-muted">…dan {importPreview.valid.length - 10} soal lainnya</li>
            )}
          </ul>
        </div>
      )}

      <div className="mt-4 flex flex-col gap-4">
        <QuestionList
          questions={questions}
          locked={locked || previewing}
          onDelete={(qid) => void removeQuestion(qid)}
          onReorder={(order) => void reorder(order)}
        />
      </div>

      {!locked && (
        <div className="mt-4 flex items-end gap-2">
          <label className="flex flex-col gap-1 text-sm">
            <span>Tipe soal baru</span>
            <select value={newType} onChange={(e) => setNewType(e.target.value as Question["type"])} className="input">
              {ALL_TYPES.map((t) => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </select>
          </label>
          <button type="button" className="btn-ghost touch-target" onClick={() => void addQuestion(newType)} disabled={saving}>
            + Soal
          </button>
        </div>
      )}

      <p className="mt-6 text-xs text-muted">
        Gunakan tombol Naik/Turun untuk mengubah urutan soal. Semua perubahan tersimpan otomatis.
      </p>
    </div>
  );

  async function onImportPick(file: File | null) {
    setImportFile(file);
    setImportPreview(null);
    if (!file) return;
    try {
      setSaving(true);
      setImportPreview(await previewQuestionImport(id, file));
      setError(null);
    } catch (e) {
      setError(errorOf(e));
    } finally {
      setSaving(false);
    }
  }

  async function onImportCommit() {
    if (!importFile) return;
    try {
      setSaving(true);
      const res = await commitQuestionImport(id, importFile);
      setError(res.errors.length > 0 ? `${res.inserted} soal disimpan, ${res.errors.length} baris gagal.` : null);
      setImportPreview(null);
      setImportFile(null);
      await load();
    } catch (e) {
      setError(errorOf(e));
    } finally {
      setSaving(false);
    }
  }

  async function onPublish() {
    try {
      setSaving(true);
      await publishSet(id);
      await load();
    } catch (e) {
      setError(errorOf(e));
    } finally {
      setSaving(false);
    }
  }

  async function addQuestion(type: Question["type"]) {
    try {
      setSaving(true);
      // body_min: placeholder paragraph — body_html NOT NULL di DB.
      await createQuestion(id, { type, body: "<p></p>", points: 1 });
      await load();
    } catch (e) {
      setError(errorOf(e));
    } finally {
      setSaving(false);
    }
  }

  async function removeQuestion(qid: string) {
    try {
      setSaving(true);
      await deleteQuestion(qid);
      await load();
    } catch (e) {
      setError(errorOf(e));
    } finally {
      setSaving(false);
    }
  }

  async function reorder(order: string[]) {
    try {
      await reorderQuestions(id, order);
      await load();
    } catch (e) {
      setError(errorOf(e));
    }
  }
}

function QuestionList({
  questions, locked, onDelete, onReorder,
}: {
  questions: Question[];
  locked: boolean;
  onDelete: (qid: string) => void;
  onReorder: (order: string[]) => void;
}) {
  const move = (from: number, to: number) => {
    if (to < 0 || to >= questions.length) return;
    const ids = questions.map((x) => x.id);
    const [m] = ids.splice(from, 1);
    ids.splice(to, 0, m);
    onReorder(ids);
  };
  return (
    <div className="flex flex-col gap-3">
      {questions.map((q, i) => (
        <QuestionCard key={q.id} q={q} no={i + 1} locked={locked}
          onUp={() => move(i, i - 1)} onDown={() => move(i, i + 1)}
          onDelete={() => onDelete(q.id)} />
      ))}
    </div>
  );
}

function QuestionCard({
  q, no, locked, onUp, onDown, onDelete,
}: {
  q: Question; no: number; locked: boolean;
  onUp: () => void;
  onDown: () => void;
  onDelete: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<Question>(q);
  const timer = useRef<number | null>(null);
  const dirty = useRef(false);

  useEffect(() => setDraft(q), [q]);
  useEffect(() => () => {
    if (timer.current) window.clearTimeout(timer.current);
  }, []);

  // Autosave (FR-4.4): PUT soal 800 ms setelah perubahan terakhir.
  const patch = useCallback((p: Partial<Question>) => {
    setDraft((d) => ({ ...d, ...p }));
    dirty.current = true;
    if (timer.current) window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      void (async () => {
        try {
          await updateQuestion(q.id, {
            type: draft.type, body: draft.body, options: draft.options,
            matching_options: draft.matching_options ?? null,
            answer_key: draft.answer_key, rubric: draft.rubric, points: draft.points,
            scoring_mode: draft.scoring_mode, explanation_html: draft.explanation_html || "",
          });
        } catch {
          // gagal senyap: perubahan tetap lokal, dicoba lagi di_ubah berikutnya
        }
      })();
    }, 800);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q.id, draft.type, draft.body, draft.options, draft.answer_key, draft.rubric,
      draft.points, draft.scoring_mode, draft.explanation_html]);

  const label = ALL_TYPES.find((t) => t.value === q.type)?.label ?? q.type;

  return (
    <div className="rounded-lg border p-4" style={{ borderColor: "var(--color-border)" }}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="mb-1 flex items-center gap-2 text-sm text-muted">
            <span className="font-medium">Soal {no}</span>
            <span>{label}</span>
            <ScoringBadge q={q} />
          </div>
          {editing && !locked ? (
            <QuestionEditor draft={draft} onPatch={patch} />
          ) : (
            <QuestionPreview q={draft} />
          )}
        </div>
        <div className="flex shrink-0 flex-col items-end gap-1">
          {!locked && (
            <>
              <button type="button" className="btn-ghost touch-target" onClick={() => setEditing((v) => !v)}>
                {editing ? "Lihat" : "Edit"}
              </button>
              <button type="button" className="btn-ghost touch-target" onClick={onUp} aria-label="naik">Naik</button>
              <button type="button" className="btn-ghost touch-target" onClick={onDown} aria-label="turun">Turun</button>
              <button type="button" className="btn-ghost touch-target" onClick={onDelete}>Hapus</button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function QuestionPreview({ q }: { q: Question }) {
  return (
    <div className="flex flex-col gap-2">
      <QuestionBodyView q={q} />
      {(q.type === "mcq" || q.type === "mcq_complex") && <OptionsView q={q} shuffle />}
      {q.type === "matching" && <MatchingView q={q} />}
      {q.type === "short_answer" && (
        <p className="text-sm text-muted">Kunci diterima: {(answerKeysOf(q) || ["—"]).join(", ")}</p>
      )}
      {q.type === "essay" && (
        <p className="text-sm text-muted">Uraian · rubrik maks {rubricOf(q)} poin</p>
      )}
      {q.explanation_html && (
        <details className="text-sm">
          <summary className="cursor-pointer text-muted">Penjelasan</summary>
          <div className="rich" dangerouslySetInnerHTML={{ __html: q.explanation_html }} />
        </details>
      )}
    </div>
  );
}

// ---- editor per tipe ----

function QuestionEditor({ draft, onPatch }: { draft: Question; onPatch: (p: Partial<Question>) => void }) {
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-end gap-3">
        <label className="flex flex-col gap-1 text-sm">
          <span>Tipe</span>
          <select value={draft.type} onChange={(e) => onPatch({ type: e.target.value as Question["type"] })} className="input">
            {ALL_TYPES.map((t) => (
              <option key={t.value} value={t.value}>{t.label}</option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-sm">
          <span>Poin</span>
          <input type="number" min={0} step={0.5} value={draft.points}
                 onChange={(e) => onPatch({ points: Number(e.target.value) || 0 })} className="input w-24" />
        </label>
        <label className="flex flex-col gap-1 text-sm">
          <span>Penilaian</span>
          <select value={draft.scoring_mode}
                  onChange={(e) => onPatch({ scoring_mode: e.target.value as Question["scoring_mode"] })}
                  className="input">
            <option value="all_or_nothing">semua-atau-tidak</option>
            <option value="partial">parsial</option>
          </select>
        </label>
      </div>

      <RichInput html={draft.body} onChange={(h) => onPatch({ body: h })} />

      {(draft.type === "mcq" || draft.type === "mcq_complex") && (
        <OptionsEditor q={draft} onPatch={onPatch} />
      )}
      {draft.type === "matching" && <MatchingEditor q={draft} onPatch={onPatch} />}
      {draft.type === "short_answer" && <ShortAnswerEditor q={draft} onPatch={onPatch} />}
      {draft.type === "essay" && <EssayEditor q={draft} onPatch={onPatch} />}

      <label className="flex flex-col gap-1 text-sm">
        <span>Penjelasan (opsional)</span>
        <textarea value={draft.explanation_html ?? ""} rows={2}
                  onChange={(e) => onPatch({ explanation_html: e.target.value })} className="input" />
      </label>
    </div>
  );
}

// MCQ / PG kompleks: opsi tersimpan di options_json [{id, html, correct}].
function OptionsEditor({ q, onPatch }: { q: Question; onPatch: (p: Partial<Question>) => void }) {
  const opts = q.options ?? [];
  const setOpt = (i: number, p: Partial<Option>) => {
    onPatch({ options: opts.map((o, j) => (j === i ? { ...o, ...p } : o)) });
  };
  const addOpt = () => {
    const id = (typeof crypto !== "undefined" && "randomUUID" in crypto)
      ? crypto.randomUUID() : `opt-${Date.now()}-${opts.length}`;
    onPatch({ options: [...opts, { id, html: "", correct: false }] });
  };
  return (
    <div className="flex flex-col gap-2">
      <span className="text-sm text-muted">
        {q.type === "mcq" ? "Tandai satu jawaban benar." : "Tandai semua jawaban benar (PG kompleks)."}
      </span>
      {opts.map((o, i) => (
        <div key={o.id} className="flex items-center gap-2">
          <input type="checkbox" checked={o.correct}
                 onChange={(e) => setOpt(i, { correct: e.target.checked })}
                 aria-label={`opsi ${i + 1} benar`} />
          <input value={o.html} placeholder={`Teks opsi ${String.fromCharCode(65 + i)}`}
                 onChange={(e) => setOpt(i, { html: e.target.value })} className="input flex-1" />
          <button type="button" className="btn-ghost touch-target"
                  onClick={() => onPatch({ options: opts.filter((_, j) => j !== i) })}
                  aria-label={`hapus opsi ${i + 1}`}>Hapus</button>
        </div>
      ))}
      <button type="button" className="btn-ghost self-start touch-target" onClick={addOpt}>+ Opsi</button>
    </div>
  );
}

// Menjodohkan: pilihan tersimpan di matching_options {left,right},
// pasangan benar di answer_key.pairs (id ↔ id) — S12. Edit per baris teks.
function MatchingEditor({ q, onPatch }: { q: Question; onPatch: (p: Partial<Question>) => void }) {
  const mo = q.matching_options ?? { left: [], right: [] };
  const key = (q.answer_key ?? {}) as { pairs?: { left: string; right: string }[] };
  const pairs = Array.isArray(key.pairs) ? key.pairs : [];

  const write = (nextPairs: { left: string; right: string }[],
                  left: { id: string; text: string }[], right: { id: string; text: string }[]) => {
    onPatch({ matching_options: { left, right }, answer_key: { pairs: nextPairs } });
  };

  const textOf = (list: {id:string;text:string}[], id: string) =>
    list.find((x) => x.id === id)?.text ?? "";

  const editText = (i: number, side: "left" | "right", text: string) => {
    const p = pairs[i];
    if (!p) return;
    const list = (side === "left" ? mo.left : mo.right).map((x) => (x.id === p[side] ? { ...x, text } : x));
    write(pairs, side === "left" ? list : mo.left, side === "right" ? list : mo.right);
  };

  const addPair = () => {
    const lid = `L${Date.now().toString(36)}${pairs.length}L`;
    const rid = `R${Date.now().toString(36)}${pairs.length}R`;
    write([...pairs, { left: lid, right: rid }],
      [...mo.left, { id: lid, text: "" }], [...mo.right, { id: rid, text: "" }]);
  };

  const removePair = (i: number) => {
    const p = pairs[i];
    if (!p) return;
    const usedLeft = pairs.some((x, j) => j !== i && x.left === p.left);
    const usedRight = pairs.some((x, j) => j !== i && x.right === p.right);
    write(pairs.filter((_, j) => j !== i),
      usedLeft ? mo.left : mo.left.filter((x) => x.id !== p.left),
      usedRight ? mo.right : mo.right.filter((x) => x.id !== p.right));
  };

  return (
    <div className="flex flex-col gap-2">
      <span className="text-sm text-muted">Pasangan kiri → kanan yang benar.</span>
      {pairs.map((p, i) => (
        <div key={p.left} className="flex items-center gap-2">
          <input value={textOf(mo.left, p.left)} placeholder={`Kiri ${i + 1}`}
                 onChange={(e) => editText(i, "left", e.target.value)} className="input flex-1" />
          <span aria-hidden>→</span>
          <input value={textOf(mo.right, p.right)} placeholder={`Kanan ${i + 1}`}
                 onChange={(e) => editText(i, "right", e.target.value)} className="input flex-1" />
          <button type="button" className="btn-ghost touch-target"
                  onClick={() => removePair(i)} aria-label={`hapus pasangan ${i + 1}`}>Hapus</button>
        </div>
      ))}
      <button type="button" className="btn-ghost self-start touch-target" onClick={addPair}>
        + Pasangan
      </button>
    </div>
  );
}

// Isian singkat: daftar kunci diterima, pencocokan tak peduli huruf besar/kecil
// (PRD FR-4.3). answer_key = { accepted: string[] }.
function ShortAnswerEditor({ q, onPatch }: { q: Question; onPatch: (p: Partial<Question>) => void }) {
  const accepted = answerKeysOf(q) ?? [];
  const setAccepted = (next: string[]) => onPatch({ answer_key: { accepted: next } });
  return (
    <div className="flex flex-col gap-2">
      <span className="text-sm text-muted">Kunci yang diterima (huruf besar/kecil diabaikan):</span>
      {accepted.map((a, i) => (
        <div key={i} className="flex items-center gap-2">
          <input value={a} className="input flex-1"
                 onChange={(e) => setAccepted(accepted.map((x, j) => (j === i ? e.target.value : x)))} />
          <button type="button" className="btn-ghost touch-target"
                  onClick={() => setAccepted(accepted.filter((_, j) => j !== i))}
                  aria-label={`hapus kunci ${i + 1}`}>Hapus</button>
        </div>
      ))}
      <button type="button" className="btn-ghost self-start touch-target"
              onClick={() => setAccepted([...accepted, ""])}>+ Kunci</button>
    </div>
  );
}

// Uraian: rubrik poin maksimum; penskoran manual guru (PRD FR-4.3).
function EssayEditor({ q, onPatch }: { q: Question; onPatch: (p: Partial<Question>) => void }) {
  const rubric = (q.rubric ?? {}) as { max_points?: number };
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span>Rubrik: poin maksimum</span>
      <input type="number" min={0} step={0.5} value={rubric.max_points ?? 0}
             onChange={(e) => onPatch({ rubric: { ...rubric, max_points: Number(e.target.value) || 0 } })}
             className="input w-32" />
    </label>
  );
}

function answerKeysOf(q: Question): string[] | undefined {
  const key = q.answer_key as { accepted?: string[] } | null | undefined;
  return Array.isArray(key?.accepted) ? key!.accepted : undefined;
}

function rubricOf(q: Question): number {
  const r = q.rubric as { max_points?: number } | null | undefined;
  return r?.max_points ?? 0;
}

// Input rich (bold/italic/list) dengan toolbar kecil; eksekusi execCommand.
export function RichInput({ html, onChange }: { html: string; onChange: (h: string) => void }) {
  const ref = useRef<HTMLDivElement>(null);
  const cmd = (c: string) => {
    document.execCommand(c, false);
    if (ref.current) onChange(ref.current.innerHTML);
  };
  return (
    <div className="rounded-lg border" style={{ borderColor: "var(--color-border)" }}>
      <Toolbar cmd={cmd} />
      <div
        ref={ref}
        contentEditable
        suppressContentEditableWarning
        className="rich min-h-[3rem] rounded-b-lg p-3 outline-none"
        style={{ background: "var(--color-bg)" }}
        onInput={(e) => onChange((e.target as HTMLDivElement).innerHTML)}
        onBlur={(e) => onChange((e.target as HTMLDivElement).innerHTML)}
        dangerouslySetInnerHTML={{ __html: html }}
      />
    </div>
  );
}

function Toolbar({ cmd }: { cmd: (c: string) => void }) {
  return (
    <div className="flex flex-wrap gap-1 border-b p-1.5" style={{ borderColor: "var(--color-border)" }}>
      {([
        ["bold", "B"], ["italic", "I"], ["underline", "U"],
        ["insertUnorderedList", "•"], ["insertOrderedList", "1."],
      ] as const).map(([c, lab]) => (
        <button key={c} type="button"
                className="touch-target rounded px-2 py-1 text-sm font-semibold"
                style={{ background: "var(--color-bg-muted)" }}
                onClick={() => cmd(c)} aria-label={c}>
          {lab}
        </button>
      ))}
    </div>
  );
}
