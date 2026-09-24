import { test, expect } from "./fixtures";
import { login } from "./helpers/login";

test("admin can update SSO settings after they've already been configured", async ({
  page,
}) => {
  await login(page, "alice");

  await page.getByRole("menuitem", { name: "System" }).hover();
  await page.getByRole("menuitem", { name: "SSO Settings" }).click();
  await expect(page).toHaveURL("/sso-settings");

  await page.locator("#issuer-url").fill("https://idp.example.com");
  await page.locator("#client-id").fill("initial-client-id");
  await page.locator("#client-secret").fill("initial-secret");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("SSO settings saved")).toBeVisible();

  await page.reload();
  await expect(page.locator("#issuer-url")).toHaveValue(
    "https://idp.example.com",
  );
  await expect(page.locator("#client-id")).toHaveValue("initial-client-id");
  // The page never echoes back the client secret.
  await expect(page.locator("#client-secret")).toHaveValue("");

  // Update only the client ID, leaving the secret blank. It should keep the
  // previously configured secret rather than rejecting the request.
  await page.locator("#client-id").fill("updated-client-id");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("SSO settings saved")).toBeVisible();

  await page.reload();
  await expect(page.locator("#client-id")).toHaveValue("updated-client-id");
});
