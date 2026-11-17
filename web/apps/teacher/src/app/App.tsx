import { Route, Routes } from "react-router-dom";
import Layout from "./Layout";

// Placeholder per peta halaman (PRD 11.1); diisi fitur nyata mulai Sprint S2–S4.
const PAGES = [
  { path: "/dashboard", title: "Dashboard" },
  { path: "/students/*", title: "Siswa & Kelas" },
  { path: "/question-sets/*", title: "Bank Soal" },
  { path: "/exams/*", title: "Ujian" },
  { path: "/reports/live", title: "Laporan Real-Time" },
  { path: "/reports/*", title: "Laporan" },
  { path: "/billing", title: "Langganan & Billing" },
  { path: "/settings/*", title: "Pengaturan" },
];

export default function App() {
  return (
    <Layout>
      <Routes>
        {PAGES.map((p) => (
          <Route key={p.path} path={p.path} element={<Placeholder title={p.title} />} />
        ))}
        <Route path="*" element={<Placeholder title="Halaman tidak ditemukan" />} />
      </Routes>
    </Layout>
  );
}

function Placeholder({ title }: { title: string }) {
  return (
    <div>
      <h1 className="text-xl font-semibold">{title}</h1>
      <p className="mt-2 text-sm" style={{ color: "var(--color-text-muted)" }}>
        Kerangka halaman — fitur diisi pada sprint berikutnya sesuai PRD.
      </p>
    </div>
  );
}
