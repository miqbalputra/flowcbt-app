import { useEffect, useState } from "react";
import {
  loadThemePreference,
  saveThemePreference,
  useThemeInitializer,
  type Theme,
} from "./theme";

/** Menulis preferensi tema ke <html> saat aplikasi dimuat. */
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [preference, setPreference] = useState<Theme>(() => loadThemePreference());
  useThemeInitializer(preference, setPreference);
  return <>{children}</>;
}

export function ThemeToggle() {
  const [preference, setPreference] = useState<Theme>(() => loadThemePreference());

  const cycle = () => {
    const next: Theme =
      preference === "system" ? "light" : preference === "light" ? "dark" : "system";
    setPreference(next);
    saveThemePreference(next);
  };

  useEffect(() => {
    const onThemeChange = () => setPreference(loadThemePreference());
    document.addEventListener("flowcbt:theme-change", onThemeChange);
    return () => document.removeEventListener("flowcbt:theme-change", onThemeChange);
  }, []);

  const label = preference === "system" ? "Ikuti sistem" : preference === "light" ? "Terang" : "Gelap";
  return (
    <button
      onClick={cycle}
      type="button"
      aria-label={`Tema saat ini: ${label}. Klik untuk mengubah.`}
      className="touch-target rounded-full border px-3 text-sm"
      style={{
        borderColor: "var(--color-border)",
        background: "var(--color-surface)",
        color: "var(--color-text)",
      }}
    >
      {label}
    </button>
  );
}
