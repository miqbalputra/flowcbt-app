import { useEffect } from "react";

export type Theme = "light" | "dark" | "system";

const STORAGE_KEY = "flowcbt:theme";

export function resolveTheme(preference: Theme): "light" | "dark" {
  if (preference !== "system") return preference;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function loadThemePreference(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY);
  return stored === "light" || stored === "dark" || stored === "system" ? stored : "system";
}

export function saveThemePreference(theme: Theme) {
  localStorage.setItem(STORAGE_KEY, theme);
  document.dispatchEvent(new CustomEvent("flowcbt:theme-change", { detail: theme }));
}

/** Menerapkan tema ke <html> dan mengikuti preferensi OS saat mode 'system'. */
export function useThemeInitializer(preference: Theme, setPreference: (t: Theme) => void) {
  useEffect(() => {
    const apply = () => {
      const resolved = resolveTheme(preference);
      document.documentElement.classList.toggle("dark", resolved === "dark");
      document.documentElement.style.colorScheme = resolved;
    };
    apply();
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const onMedia = () => {
      if (loadThemePreference() === "system") apply();
    };
    const onStorage = () => setPreference(loadThemePreference());
    media.addEventListener("change", onMedia);
    document.addEventListener("flowcbt:theme-change", onStorage);
    return () => {
      media.removeEventListener("change", onMedia);
      document.removeEventListener("flowcbt:theme-change", onStorage);
    };
  }, [preference, setPreference]);
}
