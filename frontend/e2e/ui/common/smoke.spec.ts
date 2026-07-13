import { expect, test } from "../../fixtures/ui";

test("app launches and renders", async ({ page }) => {
  await expect(page).toHaveTitle(/Wirexa/);
});
