import { test, expect } from "./fixtures";

test("logs in and logs out", async ({ page }) => {
  await page.goto("/");

  await page.getByRole("menuitem", { name: "Log In" }).click();

  await expect(page).toHaveURL("/login");
  await page.getByLabel("Username").fill("alice");
  await page.getByRole("button", { name: "Dev log in" }).click();

  await expect(page).toHaveURL("/");

  await page.getByRole("menuitem", { name: "alice" }).hover();
  await page.getByRole("menuitem", { name: "Log Out" }).click();

  await expect(page.getByRole("menuitem", { name: "Log In" })).toBeVisible();
});
