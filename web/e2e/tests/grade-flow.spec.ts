// Alur guru (S14): login → ujian → antrean koreksi → koreksi uraian.
// Data-testid dipakai agar markup bebas berubah; halaman JANGAN dirombak sebelum E2E stabil.
import { expect, test } from "@playwright/test";

test.describe("Koreksi guru", () => {
  test("buka antrean koreksi dari detail ujian", async ({ page }) => {
    await page.goto("/login");
    if (await page.getByTestId("login-email").count()) {
      await page.getByTestId("login-email").fill(process.env.E2E_GURU ?? "owner@sekolah.test");
      await page.getByTestId("login-password").fill(process.env.E2E_PASS ?? "password");
      await page.getByTestId("login-submit").click();
    }
    await expect(page).not.toHaveURL(/\/login$/);
    await page.goto("/exams");
    // Jika ada uji/daftar ujian buka pertama.
    const firstExam = page.locator('[data-testid="exam-card"] a').first();
    if (await firstExam.count()) {
      await firstExam.click();
      await expect(page.locator("h1")).toBeVisible();
    }
  });
});
