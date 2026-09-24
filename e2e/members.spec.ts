import { test, expect, type Page } from "./fixtures";
import { login } from "./helpers/login";

async function logOut(page: Page, username: string) {
  await page.getByRole("menuitem", { name: username }).hover();
  await page.getByRole("menuitem", { name: "Log Out" }).click();
  await expect(page).toHaveURL("/");
}

test("admin can promote and then demote another member", async ({ page }) => {
  // The first user to log in becomes the administrator.
  await login(page, "alice");
  await logOut(page, "alice");

  // The second user is an ordinary member.
  await login(page, "bob");
  await logOut(page, "bob");

  await login(page, "alice");

  await page.getByRole("menuitem", { name: "System" }).hover();
  await page.getByRole("menuitem", { name: "Members" }).click();
  await expect(page).toHaveURL("/members");

  const bobRow = page.getByRole("row", { name: /bob/ });
  await expect(bobRow.getByText("Member", { exact: true })).toBeVisible();

  await bobRow.getByRole("button", { name: "Promote to Admin" }).click();
  await expect(page.getByText("Promoted to administrator")).toBeVisible();
  await expect(
    bobRow.getByText("Administrator", { exact: true }),
  ).toBeVisible();

  await bobRow.getByRole("button", { name: "Revoke Admin" }).click();
  await expect(page.getByText("Administrator access revoked")).toBeVisible();
  await expect(bobRow.getByText("Member", { exact: true })).toBeVisible();
});

test("admin cannot revoke their own administrator access", async ({ page }) => {
  await login(page, "alice");

  await page.getByRole("menuitem", { name: "System" }).hover();
  await page.getByRole("menuitem", { name: "Members" }).click();
  await expect(page).toHaveURL("/members");

  const aliceRow = page.getByRole("row", { name: /alice/ });
  await expect(
    aliceRow.getByRole("button", { name: "Revoke Admin" }),
  ).toBeDisabled();
});
