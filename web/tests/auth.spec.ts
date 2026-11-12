import { expect, test } from "@playwright/test";

test.use({ baseURL: "http://127.0.0.1:18081" });

test("anonymous viewers can inspect live documentation but must sign in to edit", async ({
  page,
}) => {
  await page.goto("/");
  await expect(
    page.getByRole("button", { name: "Sign in to edit" }),
  ).toBeVisible();
  await expect(page.locator(".floor-tabs button").first()).toBeVisible();
  await expect(
    page.getByRole("button", { name: "+ Equipment", exact: true }),
  ).toHaveCount(0);
  for (const path of [
    "/api/plan",
    "/api/observations/dhcp",
    "/api/observations/live",
    "/api/observations/wifi",
    "/api/observations/ports",
    "/api/export",
  ]) {
    expect((await page.request.get(path)).status(), path).toBe(200);
  }
  const original = await (await page.request.get("/api/plan")).json();
  const stream = await page.evaluate(async () => {
    const response = await fetch("/api/observations/events");
    const reader = response.body!.getReader();
    const first = await reader.read();
    await reader.cancel();
    return {
      status: response.status,
      text: new TextDecoder().decode(first.value),
    };
  });
  expect(stream.status).toBe(200);
  expect(stream.text).toContain("data:");
  expect((await page.request.get(original.floors[0].background)).status()).toBe(
    200,
  );
  for (const [method, path] of [
    ["PUT", "/api/plan"],
    ["POST", "/api/validate"],
    ["POST", "/api/cable-labels/preview"],
  ]) {
    const response = await page.request.fetch(path, {
      method,
      headers: { "X-Yanetviz": "1" },
      data: original,
    });
    expect(response.status(), path).toBe(401);
  }
  await page.getByRole("button", { name: "Sign in to edit" }).click();
  await page.getByLabel("Administrator password").fill("wrong-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("incorrect password");
  await page.getByRole("button", { name: "Continue viewing" }).click();
  await page.getByRole("button", { name: "Sign in to edit" }).click();
  await page.getByLabel("Administrator password").fill("test-admin-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "+ Equipment", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "+ Equipment", exact: true }).click();
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByRole("status")).toContainText("Documentation saved");
  const saved = await (await page.request.get("/api/plan")).json();
  expect(saved.devices.length).toBe(original.devices.length + 1);
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Sign in to edit" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "+ Equipment", exact: true }),
  ).toHaveCount(0);
  expect(await (await page.request.get("/api/plan")).json()).toEqual(saved);
  expect(
    (
      await page.request.put("/api/plan", {
        headers: { "X-Yanetviz": "1" },
        data: original,
      })
    ).status(),
  ).toBe(401);
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Sign in to edit" }),
  ).toBeVisible();
});

test("lost edit access preserves pending changes for signing in again", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Sign in to edit" }).click();
  await page.getByLabel("Administrator password").fill("test-admin-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  const original = await (await page.request.get("/api/plan")).json();
  await page.getByRole("button", { name: "+ Equipment", exact: true }).click();
  await expect(page.locator(".project-status")).toContainText(
    "Unsaved changes",
  );
  // Simulate a session expiring on the server, then the tab regaining focus.
  expect(
    (
      await page.request.delete("/api/session", {
        headers: { "X-Yanetviz": "1" },
      })
    ).status(),
  ).toBe(204);
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  await expect(
    page.getByRole("button", { name: "Sign in to edit" }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Save changes" })).toHaveCount(
    0,
  );
  await expect(page.locator(".project-status")).toContainText(
    "Unsaved changes",
  );
  expect(await (await page.request.get("/api/plan")).json()).toEqual(original);
  await page.getByRole("button", { name: "Sign in to edit" }).click();
  await page.getByLabel("Administrator password").fill("test-admin-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.locator(".project-status")).toContainText(
    "Unsaved changes",
  );
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByRole("status")).toContainText("Documentation saved");
  const saved = await (await page.request.get("/api/plan")).json();
  expect(saved.devices.length).toBe(original.devices.length + 1);
});
