import { test, expect } from "./fixtures";
import { login } from "./helpers/login";

test("views system info", async ({ page }) => {
  await login(page);

  await page.getByRole("menuitem", { name: "System" }).hover();
  await page.getByRole("menuitem", { name: "Information" }).click();
  await expect(page).toHaveURL("/information");
  await expect(page.locator(".content")).toContainText(/Version:\s+[^v\s]\S*/);
});

test("cleans up the database on demand", async ({ page }) => {
  await login(page);

  await page.getByRole("menuitem", { name: "System" }).hover();
  await page.getByRole("menuitem", { name: "Information" }).click();
  await expect(page).toHaveURL("/information");

  await page.getByRole("button", { name: "Clean up now" }).click();

  await expect(
    page.getByText(/Last cleanup:\s+\d{4}-\d{2}-\d{2}/),
  ).toBeVisible();
  await expect(page.getByText("Reclaimable space: 0 B")).toBeVisible();
});
