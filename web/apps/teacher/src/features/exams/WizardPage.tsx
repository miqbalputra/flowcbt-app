// Wizard buat ujian (S9): langkah 1 Info (identitas & jadwal FR-5.1–5.5),
// langkah 2 Soal (pratinjau paket terbit) → simpan sebagai draft ujian.
// Langkah Peserta & Publish menyusul di S10.
import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  createExam, errorOf, toWIB,
  type ExamDraft, type Exam,
} from "./api";
import {
  getPublishedQuestions, listSets,
  type Question, type QuestionSet,
} from "../question-sets/api";
import { MatchingView, OptionsView, QuestionBodyView } from "../question-sets/views";

const EMPTY: ExamDraft = {
  question_set_id: "",
  name: "",
  open_at: "",
  close_at: "",
  duration_minutes: 60,
  close_policy: "grace",
  login_mode: "both",
  max_attempts: 1,
  score_policy: "highest",
  passing_score: null,
};

export default function WizardPage() {
  const navigate = useNavigate();
  const [step, setStep] = useState<1 | 2>(1);
  const [draft, setDraft] = useState<ExamDraft>(EMPTY);
  const [sets, setSets] = useState<QuestionSet[] | null>(null);
  const [questions, setQuestions] = useState<Question[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // Hanya paket TERBIT yang bisa dipakai ujian (syarat S9).
  useEffect(() => {
    void (async () => {
      try {
        const all = await listSets();
        setSets(all.filter((s) => s.status === "published"));
      } catch (e) {
        setError(errorOf(e));
      }
    })();
  }, []);

  const loadPublishedQuestions = useCallback(async (setId: string) => {
    setQuestions(null);
    try {
      setQuestions(await getPublishedQuestions(setId));
    } catch (e) {
      setError(errorOf(e));
    }
  }, []);

  const set = (p: Partial<ExamDraft>) => setDraft((d) => ({ ...d, ...p }));

  const toStep2 = () => {
    if (!draft.name.trim()) return setError("Nama ujian wajib diisi.");
    if (!draft.question_set_id) return setError("Pilih paket soal terbit.");
    if (draft.duration_minutes <= 0) return setError("Durasi harus lebih dari 0 menit.");
    if (draft.max_attempts !== null && draft.max_attempts <= 0)
      return setError("Jumlah percobaan harus lebih dari 0.");
    setError(null);
    setStep(2);
    void loadPublishedQuestions(draft.question_set_id);
  };

  const submit = async () => {
    setBusy(true);
    try {
      const payload: ExamDraft = {
        ...draft,
        open_at: toWIB(draft.open_at),
        close_at: toWIB(draft.close_at),
      };
      const created: Exam = await createExam(payload);
      navigate("/exams", { state: { created: created.id } });
    } catch (e) {
      setError(errorOf(e));
    } finally {
      setBusy(false);
    }
  };

  const selected = sets?.find((s) => s.id === draft.question_set_id);

  return (
    <div className="mx-auto max-w-3xl p-4">
      <h1 className="text-xl font-semibold">Buat Ujian</h1>
      <p className="text-sm text-muted">
        Langkah {step} dari 2: {step === 1 ? "Info ujian" : "Soal"}
      </p>

      {error && <p role="alert" className="mt-3" style={{ color: "var(--color-danger)" }}>{error}</p>}

      {step === 1 && (
        <div className="mt-4 flex flex-col gap-4">
          <label className="flex flex-col gap-1 text-sm">
            <span>Nama ujian</span>
            <input value={draft.name} onChange={(e) => set({ name: e.target.value })}
                   placeholder="Ujian Harian Matematika" className="input" />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span>Paket soal (hanya yang terbit)</span>
            <select value={draft.question_set_id}
                    onChange={(e) => set({ question_set_id: e.target.value })}
                    className="input">
              <option value="">— Pilih paket —</option>
              {(sets ?? []).map((s) => (
                <option key={s.id} value={s.id}>
                  {s.title} ({s.subject_name})
                </option>
              ))}
            </select>
            {sets !== null && sets.length === 0 && (
              <span className="text-xs text-muted">
                Belum ada paket terbit — publikasikan paket di halaman Bank Soal dulu.
              </span>
            )}
          </label>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <label className="flex flex-col gap-1 text-sm">
              <span>Buka (FR-5.1)</span>
              <input type="datetime-local" value={draft.open_at}
                     onChange={(e) => set({ open_at: e.target.value })} className="input" />
            </label>
            <label className="flex flex-col gap-1 text-sm">
              <span>Tutup (FR-5.1)</span>
              <input type="datetime-local" value={draft.close_at}
                     onChange={(e) => set({ close_at: e.target.value })} className="input" />
            </label>
            <label className="flex flex-col gap-1 text-sm">
              <span>Durasi (menit, FR-5.2)</span>
              <input type="number" min={1} value={draft.duration_minutes}
                     onChange={(e) => set({ duration_minutes: Number(e.target.value) || 0 })}
                     className="input" />
            </label>
            <label className="flex flex-col gap-1 text-sm">
              <span>Maks. percobaan (FR-5.3, kosong = bebas)</span>
              <input type="number" min={1} value={draft.max_attempts ?? ""}
                     onChange={(e) => set({ max_attempts: e.target.value === "" ? null : Number(e.target.value) })}
                     className="input" />
            </label>
            <label className="flex flex-col gap-1 text-sm">
              <span>Nilai akhir bila &gt;1 percobaan</span>
              <select value={draft.score_policy} onChange={(e) => set({ score_policy: e.target.value as ExamDraft["score_policy"] })}
                      className="input">
                <option value="highest">Tertinggi</option>
                <option value="last">Terakhir</option>
                <option value="avg">Rata-rata</option>
              </select>
            </label>
            <label className="flex flex-col gap-1 text-sm">
              <span>KKM (opsional, 0–100)</span>
              <input type="number" min={0} max={100} value={draft.passing_score ?? ""}
                     onChange={(e) => set({ passing_score: e.target.value === "" ? null : Number(e.target.value) })}
                     className="input" />
            </label>
            <label className="flex flex-col gap-1 text-sm">
              <span>Mode masuk</span>
              <select value={draft.login_mode} onChange={(e) => set({ login_mode: e.target.value as ExamDraft["login_mode"] })}
                      className="input">
                <option value="both">Akun atau token</option>
                <option value="account">Akun siswa</option>
                <option value="guest">Token tamu</option>
              </select>
            </label>
            <label className="flex flex-col gap-1 text-sm">
              <span>Kebijakan akhir waktu</span>
              <select value={draft.close_policy} onChange={(e) => set({ close_policy: e.target.value as ExamDraft["close_policy"] })}
                      className="input">
                <option value="grace">Grace (boleh lewat sedikit)</option>
                <option value="hard">Hard (dipotong)</option>
              </select>
            </label>
          </div>

          <fieldset className="rounded-lg border p-3 text-sm" style={{ borderColor: "var(--color-border)" }}>
            <legend className="px-1 text-xs text-muted">Pengaturan tambahan (FR-5.4–5.5)</legend>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <Check label="Nilai tampil langsung" checked={draft.show_score_immediately ?? true}
                     onChange={(v) => set({ show_score_immediately: v })} />
              <Check label="Tampilkan pembahasan" checked={draft.show_explanations ?? false}
                     onChange={(v) => set({ show_explanations: v })} />
              <Check label="Acak urutan soal" checked={draft.shuffle_questions ?? true}
                     onChange={(v) => set({ shuffle_questions: v })} />
              <Check label="Acak urutan opsi" checked={draft.shuffle_options ?? true}
                     onChange={(v) => set({ shuffle_options: v })} />
              <Check label="Navigasi bebas antar soal" checked={draft.free_navigation ?? true}
                     onChange={(v) => set({ free_navigation: v })} />
              <Check label="Wajib jawab semua sebelum submit" checked={draft.require_all_answered ?? false}
                     onChange={(v) => set({ require_all_answered: v })} />
            </div>
          </fieldset>

          <div className="flex gap-2">
            <button type="button" className="btn-primary touch-target" onClick={toStep2}>
              Lanjut: Soal
            </button>
            <button type="button" className="btn-ghost touch-target" onClick={() => navigate("/exams")}>
              Batal
            </button>
          </div>
        </div>
      )}

      {step === 2 && (
        <div className="mt-4 flex flex-col gap-4">
          {selected && (
            <p className="text-sm text-muted">
              {selected.title} · {selected.subject_name} · kode {selected.exam_code}
            </p>
          )}
          {questions === null ? (
            <p className="text-sm text-muted">Memuat soal…</p>
          ) : questions.length === 0 ? (
            <p className="text-sm text-muted">Paket terbit ini tidak memiliki soal.</p>
          ) : (
            <ol className="flex list-decimal flex-col gap-3 pl-5">
              {questions.map((q) => (
                <li key={q.id} className="rounded-lg border p-3" style={{ borderColor: "var(--color-border)" }}>
                  <div className="text-xs text-muted">{q.type} · {q.points} poin</div>
                  <QuestionBodyView q={q} />
                  {(q.type === "mcq" || q.type === "mcq_complex") && <OptionsView q={q} />}
                  {q.type === "matching" && <MatchingView q={q} />}
                </li>
              ))}
            </ol>
          )}
          <div className="flex gap-2">
            <button type="button" className="btn-primary touch-target" onClick={() => void submit()} disabled={busy}>
              {busy ? "Menyimpan…" : "Simpan ujian (draft)"}
            </button>
            <button type="button" className="btn-ghost touch-target" onClick={() => setStep(1)} disabled={busy}>
              Kembali
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function Check({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex items-center gap-2">
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span>{label}</span>
    </label>
  );
}
