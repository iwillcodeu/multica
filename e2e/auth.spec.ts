import { test, expect } from "@playwright/test";
import { loginAsDefault, openWorkspaceMenu } from "./helpers";

test.describe("Authentication", () => {
  test("login page renders correctly", async ({ page }) => {
    await page.goto("/login");

    await expect(page.locator("h1")).toContainText("Multica");
    await expect(page.locator('input[placeholder="Email"]')).toBeVisible();
    await expect(page.locator('input[placeholder="Name"]')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toContainText(
      "Sign in",
    );
  });

  test("login and land on a project board", async ({ page }) => {
    await loginAsDefault(page);

    await expect(page).toHaveURL(/\/projects\//);
    await expect(page.getByRole("link", { name: "Projects" }).first()).toBeVisible();
  });

  test("unauthenticated user is redirected to /login", async ({ page }) => {
    await page.goto("/login");
    await page.evaluate(() => {
      localStorage.removeItem("multica_token");
      localStorage.removeItem("multica_workspace_id");
    });

    await page.goto("/projects");
    await page.waitForURL("**/login", { timeout: 10000 });
  });

  test("logout redirects to /login", async ({ page }) => {
    await loginAsDefault(page);

    // Open the workspace dropdown menu
    await openWorkspaceMenu(page);

    // Click Sign out
    await page.locator("text=Sign out").click();

    await page.waitForURL("**/login", { timeout: 10000 });
    await expect(page).toHaveURL(/\/login/);
  });
});
