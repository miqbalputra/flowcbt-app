// Indikator status sinkronisasi (S17): 🟢 tersinkron, 🟡 ada antrean,
// 🔴 offline. Dihitung dari outbox IndexedDB (live query Dexie).
import { useLiveQuery } from "dexie-react-hooks";
import { db } from "../lib/db";

export default function ConnBadge() {
  const pending = useLiveQuery(
    () => db.outbox.filter((e) => e.acked === 0).count(),
    [],
    0,
  );
  const online = typeof navigator !== "undefined" && navigator.onLine;

  let dot = "🟢";
  let label = "Tersinkron";
  if (!online) {
    dot = "🔴";
    label = "Offline";
  } else if ((pending ?? 0) > 0) {
    dot = "🟡";
    label = `${pending} menunggu kirim`;
  } else if
 (pending === undefined) {
    dot = "🟡";
    label = "Menyetel…";
  }

  return (
    <span
      role="status"
      aria-label={`Status koneksi: ${label}`}
      title={label}
      className="text-sm"
    >
      {dot} <span className="hidden sm:inline">{label}</span>
    </span>
  );
}
