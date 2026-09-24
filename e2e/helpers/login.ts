import { expect, type Page } from "../fixtures";

export async function login(page: Page, username = "alice"): Promise<void> {
  await page.goto("/");
  await page.getByRole("menuitem", { name: "Log In" }).click();

  await expect(page).toHaveURL("/login");
  await page.getByLabel("Username").fill(username);
  await page.getByRole("button", { name: "Dev log in" }).click();
  await expect(page).toHaveURL("/");
}
