// Detail ujian (S10): jadwal, peserta (FR-5.6), token & ruang (FR-4.9),
// siklus status + aksi pengawas per siswa (FR-5.7).
import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api } from "../../lib/api";
import {
  addTime, errorOf, forceSubmit, getExam, kickSession, listAssignments,
  listParticipants, listRooms, listTokens, publishExam, addRoom, issueToken,
  regenerateTokens, resetAttempts, revokeToken, setExamStatus,
  setAssignments as pushAssignments,
  toWIB, updateExam,
  type Assignment, type Exam, type Participant, type Room, type Token,
} from "./api";

interface ClassItem { id: string; name: string; student_count: number }
interface StudentItem { id: string; name: string; class_id?: string; class_name?: string }

export default function ExamDetailPage() {
  const { id = "" } = useParams();
  const [exam, setExam] = useState<Exam | null>(null);
  const [assignments, setAssignments] = useState<Assignment[]>([]);
  const [participants, setParticipants] = useState<Participant[]>([]);
  const [tokens, setTokens] = useState<Token[]>([]);
  const [rooms, setRooms] = useState<Room[]>([]);
  const [classes, setClasses] = useState<ClassItem[]>([]);
  const [students, setStudents] = useState<StudentItem[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [edit, setEdit] = useState(false);
  const [draft, setDraft] = useState({ name: "", open_at: "", close_at: "", duration_minutes: 60 });
  const [pickedClasses, setPickedClasses] = useState<string[]>([]);
  const [pickedStudents, setPickedStudents] = useState<string[]>([]);
  const [studentQuery, setStudentQuery] = useState("");
  const [newRoom, setNewRoom] = useState("");

  const loadAll = useCallback(async () => {
    try {
      const [x, as, ps, ts, rs] = await Promise.all([
        getExam(id), listAssignments(id), listParticipants(id), listTokens(id), listRooms(id),
      ]);
      setExam(x);
      setAssignments(as);
      setParticipants(ps);
      setTokens(ts);
      setRooms(rs);
      setDraft({
        name: x.name,
        open_at: toLocal(x.open_at),
        close_at: toLocal(x.close_at),
        duration_minutes: x.duration_minutes ?? 60,
      });
      setError(null);
    } catch (e) {
      setError(errorOf(e));
    }
  }, [id]);

  useEffect(() => { void loadAll(); }, [loadAll]);

  useEffect(() => {
    void (async () => {
      try {
        setClasses(await api<ClassItem[]>("/classes"));
        const qs = new URLSearchParams({ limit: "200" });
        setStudents(await api<StudentItem[]>(`/students?${qs.toString()}`));
      } catch (e) {
        setError(errorOf(e));
      }
    })();
  }, []);

  // Sinkronkan pilihan dari penugasan tersimpan.
  useEffect(() => {
    setPickedClasses(assignments.filter((a) => a.target_type === "class").map((a) => a.class_id!));
    setPickedStudents(assignments.filter((a) => a.target_type === "student").map((a) => a.student_id!));
  }, [assignments]);

  if (!exam) return <p className="p-6 text-sm text-muted">Memuat…</p>;

  const run = async (fn: () => Promise<unknown>, after?: () => Promise<void> | void) => {
    setBusy(true);
    try {
      await fn();
      setError(null);
      if (after) await after();
    } catch (e) {
      setError(errorOf(e));
    } finally {
      setBusy(false);
    }
  };

  const saveSchedule = () =>
    run(() => updateExam(id, {
      name: draft.name,
      open_at: draft.open_at ? toWIB(draft.open_at) : "",
      close_at: draft.close_at ? toWIB(draft.close_at) : "",
      duration_minutes: draft.duration_minutes,
    }), loadAll).then(() => setEdit(false));

  const saveParticipants = () =>
    run(() => pushAssignments(id, [
      ...pickedClasses.map((class_id) => ({ target_type: "class" as const, class_id })),
      ...pickedStudents.map((student_id) => ({ target_type: "student" as const, student_id })),
    ]), loadAll);

  return (
    <div className="mx-auto max-w-3xl p-4">
      <p className="text-sm text-muted"><Link to="/exams">&larr; Ujian</Link></p>
      <div className="flex items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">{exam.name}</h1>
          <p className="text-sm text-muted">
            {STATUS_LABEL[exam.status]}
            {exam.open_at && ` · buka ${new Date(exam.open_at).toLocaleString("id-ID")}`}
            {exam.close_at && ` · tutup ${new Date(exam.close_at).toLocaleString("id-ID")}`}
            {exam.duration_minutes ? ` · ${exam.duration_minutes} menit` : ""}
          </p>
        </div>
        <StatusActions exam={exam} busy={busy} run={run} reload={loadAll} />
      </div>

      {error && <p role="alert" className="mt-3" style={{ color: "var(--color-danger)" }}>{error}</p>}

      {/* --- Jadwal (FR-5.1–5.2) --- */}
      <Section title="Jadwal & durasi">
        {!edit ? (
          <div className="flex flex-col gap-2 text-sm">
            <span>Nama: {exam.name}</span>
            <span>Buka: {fmt(exam.open_at)} · Tutup: {fmt(exam.close_at)} · Durasi: {exam.duration_minutes ?? "-"} menit</span>
            {(exam.status === "draft" || exam.status === "published") && (
              <button type="button" className="btn-ghost self-start touch-target" onClick={() => setEdit(true)}>
                Ubah jadwal
              </button>
            )}
          </div>
        ) : (
          <div className="flex flex-col gap-3 text-sm">
            <label className="flex flex-col gap-1">
              <span>Nama</span>
              <input value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} className="input" />
            </label>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <label className="flex flex-col gap-1">
                <span>Buka</span>
                <input type="datetime-local" value={draft.open_at} onChange={(e) => setDraft({ ...draft, open_at: e.target.value })} className="input" />
              </label>
              <label className="flex flex-col gap-1">
                <span>Tutup</span>
                <input type="datetime-local" value={draft.close_at} onChange={(e) => setDraft({ ...draft, close_at: e.target.value })} className="input" />
              </label>
              <label className="flex flex-col gap-1">
                <span>Durasi (menit)</span>
                <input type="number" min={1} value={draft.duration_minutes} onChange={(e) => setDraft({ ...draft, duration_minutes: Number(e.target.value) || 0 })} className="input" />
              </label>
            </div>
            <div className="flex gap-2">
              <button type="button" className="btn-primary touch-target" onClick={() => void saveSchedule()} disabled={busy}>Simpan</button>
              <button type="button" className="btn-ghost touch-target" onClick={() => setEdit(false)}>Batal</button>
            </div>
          </div>
        )}
      </Section>

      {/* --- Peserta (FR-5.6) --- */}
      <Section title={`Peserta (${participants.length})`}>
        <div className="flex flex-col gap-3 text-sm">
          <div>
            <p className="mb-1 font-medium">Kelas</p>
            <div className="flex flex-wrap gap-2">
              {classes.map((c) => (
                <label key={c.id} className="flex items-center gap-1 rounded border px-2 py-1" style={{ borderColor: "var(--color-border)" }}>
                  <input type="checkbox" checked={pickedClasses.includes(c.id)}
                         onChange={(e) => setPickedClasses(e.target.checked ? [...pickedClasses, c.id] : pickedClasses.filter((x) => x !== c.id))} />
                  <span>{c.name} ({c.student_count})</span>
                </label>
              ))}
            </div>
          </div>
          <div>
            <p className="mb-1 font-medium">Siswa individual</p>
            <input placeholder="Cari nama siswa…" value={studentQuery} onChange={(e) => setStudentQuery(e.target.value)} className="input mb-2" />
            <div className="flex max-h-40 flex-wrap gap-2 overflow-y-auto">
              {students
                .filter((s) => s.name.toLowerCase().includes(studentQuery.toLowerCase()))
                .slice(0, 50)
                .map((s) => (
                  <label key={s.id} className="flex items-center gap-1 rounded border px-2 py-1" style={{ borderColor: "var(--color-border)" }}>
                    <input type="checkbox" checked={pickedStudents.includes(s.id)}
                           onChange={(e) => setPickedStudents(e.target.checked ? [...pickedStudents, s.id] : pickedStudents.filter((x) => x !== s.id))} />
                    <span>{s.name}{s.class_name ? ` · ${s.class_name}` : ""}</span>
                  </label>
                ))}
            </div>
          </div>
          <button type="button" className="btn-primary self-start touch-target" onClick={() => void saveParticipants()} disabled={busy}>
            Simpan penugasan
          </button>

          <table className="mt-2 w-full text-sm">
            <thead>
              <tr className="text-left text-muted">
                <th className="py-1">Nama</th>
                <th>Kelas</th>
                <th>Percobaan</th>
                <th>Status</th>
                <th>Pelanggaran</th>
                <th className="text-right">Aksi (FR-5.7)</th>
              </tr>
            </thead>
            <tbody>
              {participants.map((p) => (
                <tr key={p.student_id} className="border-t" style={{ borderColor: "var(--color-border)" }}>
                  <td className="py-1">{p.name}</td>
                  <td className="text-center">{p.class_name ?? "-"}</td>
                  <td className="text-center">{p.attempts}</td>
                  <td className="text-center">{p.attempt_status || "belum"}</td>
                  <td className="text-center">{p.violations}</td>
                  <td className="py-1 text-right">
                    <div className="flex flex-wrap justify-end gap-1">
                      <button type="button" className="btn-ghost touch-target" disabled={busy}
                              onClick={() => { const s = window.prompt("Tambah waktu (menit):", "5"); if (s) void run(() => addTime(id, p.student_id, Number(s) * 60), loadAll); }}>
                        +Waktu
                      </button>
                      <button type="button" className="btn-ghost touch-target" disabled={busy}
                              onClick={() => { if (window.confirm("Paksa selesai percobaan berjalan?")) void run(() => forceSubmit(id, p.student_id), loadAll); }}>
                        Paksa selesai
                      </button>
                      <button type="button" className="btn-ghost touch-target" disabled={busy}
                              onClick={() => { if (window.confirm("Keluarkan sesi aktif (login ulang)?")) void run(() => kickSession(id, p.student_id), loadAll); }}>
                        Keluarkan
                      </button>
                      <button type="button" className="btn-ghost touch-target" disabled={busy}
                              onClick={() => { if (window.confirm("Hapus semua percobaan siswa ini agar mulai lagi dari awal?")) void run(() => resetAttempts(id, p.student_id), loadAll); }}>
                        Reset
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {participants.length === 0 && (
                <tr><td colSpan={6} className="py-2 text-muted">Belum ada peserta — pilih kelas/siswa lalu simpan.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </Section>

      {/* --- Token & ruang (FR-4.9) --- */}
      <Section title="Token & ruang">
        <div className="flex flex-col gap-3 text-sm">
          <div className="flex flex-wrap items-end gap-2">
            <button type="button" className="btn-ghost touch-target" disabled={busy}
                    onClick={() => void run(() => issueToken(id), loadAll)}>
              + Token
            </button>
            <button type="button" className="btn-ghost touch-target" disabled={busy}
                    onClick={() => { if (window.confirm("Cabut semua token aktif dan terbitkan token baru?")) void run(() => regenerateTokens(id), loadAll); }}>
              Regenerasi semua
            </button>
            <input placeholder="Ruang baru…" value={newRoom} onChange={(e) => setNewRoom(e.target.value)} className="input w-40" />
            <button type="button" className="btn-ghost touch-target" disabled={busy || !newRoom.trim()}
                    onClick={() => void run(() => addRoom(id, newRoom.trim()).then(() => setNewRoom("")), loadAll)}>
              + Ruang
            </button>
          </div>
          <p className="text-xs text-muted">Ruang: {rooms.length === 0 ? "—" : rooms.map((r) => r.name).join(", ")}</p>
          <table className="w-full">
            <thead>
              <tr className="text-left text-muted">
                <th className="py-1">Token</th>
                <th>Status</th>
                <th className="text-right">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {tokens.map((t) => (
                <tr key={t.token} className="border-t" style={{ borderColor: "var(--color-border)" }}>
                  <td className="py-1 font-mono">{t.token}</td>
                  <td>{t.is_active ? "aktif" : "nonaktif"}</td>
                  <td className="text-right">
                    {t.is_active && (
                      <button type="button" className="btn-ghost touch-target" disabled={busy}
                              onClick={() => void run(() => revokeToken(t.token), loadAll)}>
                        Cabut
                      </button>
                    )}
                  </td>
                </tr>
              ))}
              {tokens.length === 0 && (
                <tr><td colSpan={3} className="py-2 text-muted">Belum ada token — klik "+ Token".</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </Section>
    </div>
  );
}

const STATUS_LABEL: Record<Exam["status"], string> = {
  draft: "Draft",
  published: "Terjadwal",
  paused: "Dijeda",
  closed: "Selesai",
  cancelled: "Dibatalkan",
};

function StatusActions({ exam, busy, run, reload }: {
  exam: Exam; busy: boolean;
  run: (fn: () => Promise<unknown>, after?: () => Promise<void> | void) => Promise<void>;
  reload: () => Promise<void>;
}) {
  return (
    <div className="flex shrink-0 flex-wrap justify-end gap-2">
      {exam.status === "draft" && (
        <button type="button" className="btn-primary touch-target" disabled={busy}
                onClick={() => { if (window.confirm("Publikasikan ujian? Peserta akan bisa masuk sesuai jadwal.")) void run(() => publishExam(exam.id), reload); }}>
          Publish
        </button>
      )}
      {exam.status === "published" && (
        <button type="button" className="btn-ghost touch-target" disabled={busy}
                onClick={() => void run(() => setExamStatus(exam.id, "pause"), reload)}>
          Jeda darurat
        </button>
      )}
      {exam.status === "paused" && (
        <button type="button" className="btn-primary touch-target" disabled={busy}
                onClick={() => void run(() => setExamStatus(exam.id, "resume"), reload)}>
          Lanjutkan
        </button>
      )}
      {(exam.status === "published" || exam.status === "paused") && (
        <button type="button" className="btn-ghost touch-target" disabled={busy}
                onClick={() => { if (window.confirm("Tutup ujian? Tidak bisa dibuka lagi.")) void run(() => setExamStatus(exam.id, "close"), reload); }}>
          Tutup
        </button>
      )}
      {exam.status !== "closed" && exam.status !== "cancelled" && (
        <button type="button" className="btn-ghost touch-target" disabled={busy}
                onClick={() => { if (window.confirm("Batalkan ujian ini?")) void run(() => setExamStatus(exam.id, "cancel"), reload); }}>
          Batalkan
        </button>
      )}
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mt-6 rounded-lg border p-4" style={{ borderColor: "var(--color-border)" }}>
      <h2 className="mb-3 text-base font-semibold">{title}</h2>
      {children}
    </section>
  );
}

function fmt(iso?: string) {
  return iso ? new Date(iso).toLocaleString("id-ID") : "-";
}

/** ISO dengan offset → nilai datetime-local (tampilkan dalam WIB). */
function toLocal(iso?: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  const wib = new Date(d.getTime() + 7 * 3600 * 1000);
  return `${wib.getUTCFullYear()}-${pad(wib.getUTCMonth() + 1)}-${pad(wib.getUTCDate())}T${pad(wib.getUTCHours())}:${pad(wib.getUTCMinutes())}`;
}
