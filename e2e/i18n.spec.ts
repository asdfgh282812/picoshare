import { test, expect } from "./fixtures";
import { login } from "./helpers/login";

test("switches the interface language from the navbar and it survives a reload", async ({
  page,
}) => {
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "What is PicoShare?" }),
  ).toBeVisible();

  await page.getByRole("menuitem", { name: "Language" }).hover();
  await page.getByRole("menuitem", { name: "繁體中文" }).click();

  await expect(page.locator("html")).toHaveAttribute("lang", "zh-TW");
  await expect(
    page.getByRole("heading", { name: "PicoShare 是什麼？" }),
  ).toBeVisible();

  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-TW");
  await expect(
    page.getByRole("heading", { name: "PicoShare 是什麼？" }),
  ).toBeVisible();
});

test("a logged-in user's language choice follows them across sessions", async ({
  page,
}) => {
  await login(page);

  await page.getByRole("menuitem", { name: "Language" }).hover();
  await page.getByRole("menuitem", { name: "繁體中文" }).click();
  await expect(page.getByRole("heading", { name: "上傳" })).toBeVisible();

  // Log out, clear the language cookie the way a different browser would
  // start out, and log back in. The account-level preference should still
  // apply.
  await page.getByRole("menuitem", { name: "alice" }).hover();
  await page.getByRole("menuitem", { name: "登出" }).click();
  await expect(page).toHaveURL("/");
  await expect(page.getByRole("menuitem", { name: "登入" })).toBeVisible();

  await page.context().clearCookies();

  await login(page);
  await expect(page.getByRole("heading", { name: "上傳" })).toBeVisible();
});

test("the administrator's default language applies to a visitor with no language chosen yet", async ({
  page,
}) => {
  await login(page);

  await page.getByRole("menuitem", { name: "System" }).hover();
  await page.getByRole("menuitem", { name: "Settings", exact: true }).click();
  await expect(page).toHaveURL("/settings");

  await page.locator("#default-language").selectOption("zh-TW");
  await page.getByRole("button", { name: "Save" }).click();

  // Clearing cookies drops both the session and any language cookie, leaving
  // a visitor who has never chosen a language and isn't logged in.
  await page.context().clearCookies();
  await page.goto("/");

  await expect(page.locator("html")).toHaveAttribute("lang", "zh-TW");
  await expect(
    page.getByRole("heading", { name: "PicoShare 是什麼？" }),
  ).toBeVisible();
});
