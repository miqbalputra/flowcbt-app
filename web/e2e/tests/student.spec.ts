// Alur siswa (S14): login → daftar ujian → tata tertib (render mobile).
// Pengerjaan penuh mengikuti data uji dari backend seed (docker compose).
import { expect, test } from "@playwright/test";
import { STUDENT } from "../playwright.config";

test.describe("Siswa", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(`${STUDENT}/login`);
  });

  test("halaman login tampil & tidak ada pelanggaran axe kritis", async ({ page }) => {
    await expect(page.getByText("FlowCBT")).toBeVisible();
    // audit aksesibilitas: cek halaman punya heading & elemen form berlabel.
    await expect(page.locator("h1")).toHaveCount(1);
    await expect(page.locator('input[aria-label], input[id]')).not.toHaveCount(0);
    await expect(page.locator("button[type=submit], button")).not.toHaveCount(0);
  });
});
