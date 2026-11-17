import { Route, Routes } from "react-router-dom";
import { ThemeToggle } from "flowcbt-ui";

// Klien ujian siswa: satu kolom, tombol besar, mobile-first (PRD 11).
const PAGES = [
  { path: "/login", title: "Masuk Siswa" },
  { path: "/guest", title: "Ujian Tanpa Akun" },
  { path: "/exams", title: "Ujian Saya" },
  { path: "/exams/:id/start", title: "Token & Tata Tertib" },
  { path: "/attempt/:id", title: "Butir Soal" },
  { path: "/attempt/:id/review", title: "Konfirmasi Selesai" },
  { path: "/attempt/:id/done", title: "Selesai" },
  { path: "/history", title: "Riwayat" },
];

export default function App() {
  return (
    <div className="min-h-dvh" style={{ background: "var(--color-bg)", color: "var(--color-text)" }}>
      <Routes>
        {PAGES.map((p) => (
          <Route key={p.path} path={p.path} element={<Placeholder title={p.title} />} />
        ))}
        <Route path="*" element={<Placeholder title="Halaman tidak ditemukan" />} />
      </Routes>
    </div>
  );
}

function Placeholder({ title }: { title: string }) {
  return (
    <div className="mx-auto flex min-h-dvh max-w-md flex-col px-4 py-6">
      <header className="flex items-center justify-between">
        <span className="font-bold">FlowCBT</span>
        <ThemeToggle />
      </header>
      <main className="flex flex-1 flex-col justify-center gap-3 py-8">
        <h1 className="text-xl font-semibold">{title}</h1>
        <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
          Kerangka halaman — fitur diisi pada sprint berikutnya sesuai PRD.
        </p>
      </main>
    </div>
  );
}
