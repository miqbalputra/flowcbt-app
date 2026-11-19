// E2E guru→siswa (S14, QA). Jalankan: cd web/e2e && pnpm i && pnpm e2e
// Butuh API + database (docker compose up) & dev server kedua app pada
// guru http://localhost:5173 / siswa http://localhost:5174 (default vite).
import { defineConfig, devices } from "@playwright/test";

const BASE_API = process.env.E2E_API ?? "http://localhost:8080";
const TEACHER = process.env.E2E_TEACHER ?? "http://localhost:5173";
const STUDENT = process.env.E2E_STUDENT ?? "http://localhost:5174";

const smoke = process.env.E2E_SMOKE === "1"; // tanpa backend: hanya render

export default defineConfig({
  testDir: "./tests",
  timeout: 60_000,
  retries: process.env.CI ? 1 : 0,
  use: {
    baseURL: TEACHER,
    trace: "on-first-retry",
    extraHTTPHeaders: { "x-e2e-api": BASE_API },
  },
  projects: [
    { name: "desktop", use: { ...devices["Desktop Chrome"] } },
    { name: "mobile-siswa", use: { ...devices["Pixel 7"] }, testMatch: /student\.spec\.ts/ },
  ],
  outputDir: "./test-results",
  ...(smoke ? { webServer: undefined } : {}),
});

export { TEACHER, STUDENT };
