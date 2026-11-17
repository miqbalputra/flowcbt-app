import { ThemeToggle } from "flowcbt-ui";
import { useAuth } from "../features/auth/AuthContext";

const NAV = [
  { to: "/dashboard", label: "Dashboard" },
  { to: "/students", label: "Siswa" },
  { to: "/question-sets", label: "Soal" },
  { to: "/exams", label: "Ujian" },
  { to: "/reports/live", label: "Laporan" },
  { to: "/billing", label: "Langganan" },
  { to: "/settings", label: "Pengaturan" },
];

// Shell guru: sidebar di desktop (>= 1025 px), bottom nav di mobile/tablet (PRD 11).
export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-dvh" style={{ background: "var(--color-bg)", color: "var(--color-text)" }}>
      <div className="flex">
        {/* Sidebar — desktop */}
        <aside
          className="hidden w-60 shrink-0 border-r px-3 py-4 md:block"
          style={{ borderColor: "var(--color-border)", position: "sticky", top: 0, height: "100dvh" }}
        >
          <div className="mb-6 px-2 text-lg font-bold">FlowCBT</div>
          <nav className="flex flex-col gap-1">
            {NAV.map((item) => (
              <a key={item.to} href={item.to} className="nav-link touch-target">
                {item.label}
              </a>
            ))}
          </nav>
        </aside>

        {/* Konten */}
        <main className="min-w-0 flex-1 pb-20 md:pb-6">
          <header
            className="flex items-center justify-between border-b px-4 py-3"
            style={{ borderColor: "var(--color-border)" }}
          >
            <span className="font-semibold md:hidden">FlowCBT</span>
            <span className="text-sm" style={{ color: "var(--color-text-muted)" }}>
              Guru / Admin
            </span>
            <div className="flex items-center gap-2">
              <LogoutButton />
              <ThemeToggle />
            </div>
          </header>
          <div className="p-4">{children}</div>
        </main>
      </div>

      {/* Bottom nav — mobile */}
      <nav
        className="fixed inset-x-0 bottom-0 z-10 flex justify-around border-t py-1 md:hidden"
        style={{
          borderColor: "var(--color-border)",
          background: "var(--color-surface)",
        }}
      >
        {NAV.slice(0, 5).map((item) => (
          <a
            key={item.to}
            href={item.to}
            className="touch-target flex flex-col items-center justify-center px-2 text-xs"
            style={{ color: "var(--color-text-muted)" }}
          >
            {item.label}
          </a>
        ))}
      </nav>
    </div>
  );
}

function LogoutButton() {
  const { logout } = useAuth();
  return (
    <button
      type="button"
      onClick={logout}
      className="touch-target rounded-full border px-3 py-1.5 text-sm"
      style={{
        borderColor: "var(--color-border)",
        background: "var(--color-surface)",
        color: "var(--color-text)",
      }}
    >
      Keluar
    </button>
  );
}
