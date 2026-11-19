// Penyimpanan lokal siswa (S15, FR-11): IndexedDB via Dexie.
// Tabel: attempts (sesi & paket ringkas), questions (butir per attempt),
// outbox (event jawaban menunggu ACK — dipakai penuh S16), media (blob).
// Prinsip: paket soal diunduh sekali sebelum mulai; app tetap terbuka offline.
import Dexie, { type Table } from "dexie";
import type { ClientQuestion, StartResult } from "./api";

export interface StoredAttempt extends StartResult {
  attempt_id: string;
  saved_at: number;      // epoch ms
  status: string;        // in_progress | pending_sync | submitted | ...
}

export interface StoredQuestion extends ClientQuestion {
  id: string;
  attempt_id: string;
  order_no: number;
}

export interface OutboxEvent {
  id?: number;            // auto-increment
  attempt_id: string;
  client_event_id: string; // idempotensi (S16)
  seq: number;             // sequence_number per attempt
  kind: "answer" | "flag" | "submit";
  payload: unknown;
  created_at: number;
  acked: 0 | 1;
  synced_at?: number;
}

export interface StoredMedia {
  id: string;   // media_id
  attempt_id: string;
  blob: Blob;
  content_type: string;
}

class FlowDB extends Dexie {
  attempts!: Table<StoredAttempt, string>;
  questions!: Table<StoredQuestion, string>;
  outbox!: Table<OutboxEvent, number>;
  media!: Table<StoredMedia, string>;

  constructor() {
    super("flowcbt-student");
    this.version(1).stores({
      attempts: "attempt_id, exam_id, saved_at",
      questions: "id, attempt_id, [attempt_id+order_no]",
      outbox: "++id, client_event_id, attempt_id, acked",
      media: "id, attempt_id",
    });
  }
}

export const db = new FlowDB();

/** Simpan paket attempt (header + soal) dari StartResult — idempoten. */
export async function saveAttemptPack(res: StartResult): Promise<void> {
  const now = Date.now();
  await db.transaction("rw", db.attempts, db.questions, async () => {
    await db.attempts.put({ ...res, saved_at: now, status: "in_progress" });
    const old = await db.questions.where("attempt_id").equals(res.attempt_id).toArray();
    if (old.length) await db.questions.bulkDelete(old.map((q) => q.id));
    await db.questions.bulkPut(res.questions.map((q) => ({ ...q, attempt_id: res.attempt_id })));
  });
}

/** Ambil paket attempt dari IndexedDB (pemulihan offline). */
export async function loadAttemptPack(attemptId: string): Promise<StoredAttempt | null> {
  return (await db.attempts.get(attemptId)) ?? null;
}

/** Unduh & simpan media paket; panggil setelah saveAttemptPack. */
export async function saveAttemptMedia(res: StartResult): Promise<void> {
  const ids = [...new Set(res.questions.flatMap((q) => q.media_ids ?? []))];
  if (ids.length === 0) return;
  const existing = await db.media.where("attempt_id").equals(res.attempt_id).toArray();
  const have = new Set(existing.map((m) => m.id));
  await Promise.all(
    ids
      .filter((id) => !have.has(id))
      .map(async (id) => {
        const url = `/api/media/${id}`;
        const resp = await fetch(url);
        if (!resp.ok) throw new Error(`media ${resp.status}`);
        const blob = await resp.blob();
        await db.media.put({ id, attempt_id: res.attempt_id, blob, content_type: blob.type });
      }),
  );
}

/** URL media untuk render: blob lokal bila ada, fallback ke endpoint. */
export async function mediaURL(id: string): Promise<string> {
  const m = await db.media.get(id);
  return m ? URL.createObjectURL(m.blob) : `/api/media/${id}`;
}

/** Hapus seluruh data attempt (dipanggil setelah sinkron penuh — S18). */
export async function purgeAttempt(attemptId: string): Promise<void> {
  await db.transaction("rw", db.attempts, db.questions, db.outbox, db.media, async () => {
    await db.attempts.delete(attemptId);
    const qs = await db.questions.where("attempt_id").equals(attemptId).toArray();
    await db.questions.bulkDelete(qs.map((q) => q.id));
    await db.outbox.where("attempt_id").equals(attemptId).delete();
    await db.media.where("attempt_id").equals(attemptId).delete();
  });
}
