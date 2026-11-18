import { Route, Routes } from "react-router-dom";
import { ThemeToggle } from "flowcbt-ui";
import AttemptPage from "../features/Attempt";
import { ExamsPage, GuestPage, LoginPage, StartPage } from "../features/StudentPages";

// Klien ujian siswa: satu kolom, tombol besar, mobile-first (PRD 11).
export default function App() {
  return (
    <div className="min-h-dvh" style={{ background: "var(--color-bg)", color: "var(--color-text)" }}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/guest" element={<GuestPage />} />
        <Route path="/exams" element={<ExamsPage />} />
        <Route path="/exams/:id/start" element={<StartPage />} />
        <Route path="/attempt/:id" element={<AttemptPage />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
    </div>
  );
}

function NotFound() {
  return (
    <div className="mx-auto flex min-h-dvh max-w-md flex-col px-4 py-6">
      <header className="flex items-center justify-between">
        <span className="font-bold">FlowCBT</span>
        <ThemeToggle />
      </header>
      <main className="flex flex-1 flex-col justify-center gap-3 py-8">
        <h1 className="text-xl font-semibold">Halaman tidak ditemukan</h1>
      </main>
    </div>
  );
}
