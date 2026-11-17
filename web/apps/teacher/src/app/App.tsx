import { Navigate, Route, Routes } from "react-router-dom";
import { ThemeToggle } from "flowcbt-ui";
import { AuthProvider, useAuth } from "../features/auth/AuthContext";
import Layout from "./Layout";
import LoginPage from "../features/auth/LoginPage";
import RegisterPage from "../features/auth/RegisterPage";

// Placeholder per peta halaman (PRD 11.1); diisi fitur nyata mulai Sprint S3–S4.
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
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route
          path="*"
          element={
            <RequireAuth>
              <Layout>
                <Routes>
                  {PAGES.map((p) => (
                    <Route key={p.path} path={p.path} element={<Placeholder title={p.title} />} />
                  ))}
                  <Route path="*" element={<Placeholder title="Halaman tidak ditemukan" />} />
                </Routes>
              </Layout>
            </RequireAuth>
          }
        />
      </Routes>
    </AuthProvider>
  );
}

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { authenticated, booting } = useAuth();
  if (booting) return <FullScreenCenter>Membuka sesi…</FullScreenCenter>;
  if (!authenticated) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function FullScreenCenter({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-dvh items-center justify-center" style={{ background: "var(--color-bg)" }}>
      <div className="flex flex-col items-center gap-3">
        <span>{children}</span>
        <ThemeToggle />
      </div>
    </div>
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
