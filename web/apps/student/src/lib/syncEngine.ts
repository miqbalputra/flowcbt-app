// Outbox & engine sinkronisasi (S16, AC-1..AC-3):
// - Event jawaban/flag persisten di IndexedDB (bertahan reload/browser restart).
// - Batch dikirim tiap ~7 detik saat ada antrean dan online (PRD 5-10 dtk).
// - Event 'online' → sinkron segera; kegagalan → backoff eksponensial (maks 60 dtk).
// - Antrean dihapus HANYA setelah ACK server; idempoten via client_event_id
//   (duplikat aman di DO NOTHING sisi backend). Last-write-wins by seq.
import { db } from "./db";
import { api } from "./api";

type OutEv = { type: string; question_id?: string; answer?: unknown; flagged?: boolean };

/** Masukkan event ke outbox persisten; seq per attempt bertambah. */
export async function enqueue(attemptId: string, ev: OutEv): Promise<void> {
  await db.transaction("rw", db.attempts, db.outbox, async () => {
    const stored = await db.attempts.get(attemptId);
    const seq = (stored?.outbox_seq ?? 0) + 1;
    if (stored) {
      await db.attempts.update(attemptId, { outbox_seq: seq });
    }
    await db.outbox.add({
      attempt_id: attemptId,
      client_event_id: (globalThis.crypto?.randomUUID?.() ??
        `${Date.now()}-${Math.random().toString(16).slice(2)}`),
      seq,
      kind: ev.type === "flag" ? "flag" : ev.type === "submit" ? "submit" : "answer",
      payload: { ...ev, elapsed_ms: 0 },
      created_at: Date.now(),
      acked: 0 as const,
    });
  });
}

export interface FlushResult {
  sent: number;
  lastSeq: number;
  serverTime: string;
}

/** Kirim satu batch (maks `count`) event dari outbox. Gagal → throw (backoff di engine). */
export async function flushOutbox(attemptId: string, count = 100): Promise<FlushResult | null> {
  const pending = await db.outbox
    .where("attempt_id")
    .equals(attemptId)
    .filter((e) => e.acked === 0)
    .sortBy("seq");
  if (pending.length === 0) return null;
  const batch = pending.slice(0, count);
  const events = batch.map((e) => ({
    client_event_id: e.client_event_id,
    elapsed_ms: 0,
    ...(e.payload as Record<string, unknown>),
  }));
  const maxSeq = Math.max(...batch.map((e) => e.seq));
  const res = await api<{ accepted: number; rejected: number; last_seq: number; server_time: string }>(
    `/attempts/${attemptId}/answers:sync`,
    { method: "POST", body: { seq: maxSeq, events } },
  );
  // ACK diterima (2xx): server sudah mencatat semuanya (accepted atau rejected
  // dengan alasan); hapus batch — duplikat kirim ulang tetap aman.
  await db.outbox.bulkDelete(batch.map((e) => e.id!).filter(Boolean));
  return { sent: events.length, lastSeq: res.last_seq, serverTime: res.server_time };
}

/**
 * Engine siklus: mulai loop untuk satu attempt, kembalikan fungsi stop.
 * Interval dasar 7 detik (batch 5-10 dtk PRD); backoff 1·2^f gagal (maks 60 dtk).
 */
export function startSyncEngine(attemptId: string, baseIntervalMs = 7000): () => void {
  let stopped = false;
  let fails = 0;
  let timer: number | null = null;

  const schedule = (ms: number) => {
    if (timer !== null) window.clearTimeout(timer);
    timer = window.setTimeout(() => void tick(), ms);
  };

  const tick = async () => {
    if (stopped) return;
    const pending = await db.outbox
      .where("attempt_id")
      .equals(attemptId)
      .filter((e) => e.acked === 0)
      .count();
    if (pending > 0) {
      try {
        await flushOutbox(attemptId);
        fails = 0;
        schedule(baseIntervalMs);
      } catch {
        fails += 1;
        schedule(Math.min(60_000, baseIntervalMs * 2 ** Math.min(fails, 3)));
      }
    } else {
      schedule(baseIntervalMs);
    }
  };

  // Kembali online → sinkron segera (PRD AC-2).
  const onOnline = () => { void tick(); };
  window.addEventListener("online", onOnline);

  void tick();
  return () => {
    stopped = true;
    if (timer !== null) window.clearTimeout(timer);
    window.removeEventListener("online", onOnline);
  };
}
