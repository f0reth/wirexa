import { expect, test } from "../../fixtures/ui";

// 切り詰められたレスポンスの全文は backend が execution ID で追跡する一時ファイルにある。
// RPC 境界には一時ファイルのパスではなく、送信時の execution ID だけが渡ることを確かめる。

test.beforeEach(async ({ page }) => {
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(
    page.getByPlaceholder("https://api.example.com/endpoint"),
  ).toBeVisible();
});

/** SendRequest に渡った n 番目の execution ID (第 1 引数)。 */
async function sentExecutionId(
  fake: { args(name: string): Promise<unknown[][]> },
  n = 0,
): Promise<string> {
  const calls = await fake.args("SendRequest");
  return calls[n][0] as string;
}

test.describe("truncated response body", () => {
  test.use({ seed: { httpResponse: { bodyTruncated: true } } });

  test("saving passes only the execution ID and cannot be repeated", async ({
    page,
    app,
    fake,
  }) => {
    await app.urlInput.fill("https://example.com/large");
    await app.sendButton.click();
    await page.getByRole("button", { name: "Save body to file" }).click();

    await expect(page.getByText("Saved the full body to a file.")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Save body to file" }),
    ).toHaveCount(0);

    const [saveArgs] = await fake.args("SaveResponseBody");
    expect(saveArgs).toEqual([await sentExecutionId(fake)]);
  });

  test("sending again discards the previous body", async ({
    page,
    app,
    fake,
  }) => {
    await app.urlInput.fill("https://example.com/large");
    await app.sendButton.click();
    await expect(
      page.getByRole("button", { name: "Save body to file" }),
    ).toBeVisible();

    await app.sendButton.click();
    await fake.waitForCalls("DiscardResponseBody");

    const [discardArgs] = await fake.args("DiscardResponseBody");
    expect(discardArgs).toEqual([await sentExecutionId(fake, 0)]);
  });
});

test.describe("reclaimed response body", () => {
  test.use({
    seed: {
      httpResponse: { bodyTruncated: true },
      saveResponseError: "response body unavailable",
    },
  });

  test("shows a resend hint when the backend no longer has the body", async ({
    page,
    app,
  }) => {
    await app.urlInput.fill("https://example.com/large");
    await app.sendButton.click();
    await page.getByRole("button", { name: "Save body to file" }).click();

    await expect(
      page.getByText("The full body is no longer available."),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Save body to file" }),
    ).toHaveCount(0);
  });
});
