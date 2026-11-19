import { Navigate, Route, Routes } from "react-router-dom";
import { ThemeToggle } from "flowcbt-ui";
import { AuthProvider, useAuth } from "../features/auth/AuthContext";
import Layout from "./Layout";
import LoginPage from "../features/auth/LoginPage";
import RegisterPage from "../features/auth/RegisterPage";
import QuestionSetsPage from "../features/question-sets/QuestionSetsPage";
import EditorPage from "../features/question-sets/EditorPage";
import SettingsPage from "../features/settings/SettingsPage";
import ClassesPage from "../features/classes/ClassesPage";
import StudentsPage from "../features/students/StudentsPage";
import ExamsPage from "../features/exams/ExamsPage";
import WizardPage from "../features/exams/WizardPage";
import ExamDetailPage from "../features/exams/ExamDetailPage";
import GradingQueuePage from "../features/exams/GradingQueuePage";
import GradingDetailPage from "../features/exams/GradingDetailPage";

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
                  <Route path="/settings/*" element={<SettingsPage />} />
                  <Route path="/classes" element={<ClassesPage />} />
                  <Route path="/students/*" element={<StudentsPage />} />
                  <Route path="/question-sets" element={<QuestionSetsPage />} />
                  <Route path="/question-sets/:id" element={<EditorPage />} />
                  <Route path="/exams" element={<ExamsPage />} />
                  <Route path="/exams/new" element={<WizardPage />} />
                  <Route path="/exams/:id" element={<ExamDetailPage />} />
                  <Route path="/exams/:id/grading" element={<GradingQueuePage />} />
                  <Route path="/grading/:id" element={<GradingDetailPage />} />
                  {PAGES.filter((p) => !p.path.startsWith("/settings") && p.path !== "/students/*" && !p.path.startsWith("/question-sets") && !p.path.startsWith("/exams")).map((p) => (
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
