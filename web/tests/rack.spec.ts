import { expect, test } from "@playwright/test";

test("cabinet capacity and multi-unit equipment persist, validate, and clear on reassignment", async ({
  page,
  request,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "View mode", exact: true }).click();
  const inventory = page.locator(".inventory");
  await inventory.getByRole("button", { name: /Basement cabinet/ }).click();
  await expect(page.getByLabel("Cabinet height (U)")).toHaveValue("");
  await page.getByLabel("Cabinet height (U)").fill("12");
  await inventory.getByRole("button", { name: /IV14-RT01/ }).click();
  await expect(
    page.getByLabel("Starting unit (bottom of equipment)"),
  ).toHaveValue("");
  await page.getByLabel("Equipment height (U)").fill("2");
  await page.getByLabel("Starting unit (bottom of equipment)").fill("3");
  await page.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Documentation saved");
  await page.reload();
  await inventory.getByRole("button", { name: /Basement cabinet/ }).click();
  await expect(page.getByLabel("Cabinet height (U)")).toHaveValue("12");
  await expect(page.locator(".rack-device")).toContainText("U3–U4 · 2 U");
  const mounted = page.locator(".rack-device").filter({ hasText: "IV14-RT01" });
  await expect(mounted).toHaveCSS("grid-row-start", "9");
  await expect(mounted).toHaveCSS("grid-row-end", "span 2");
  await page.screenshot({ path: "/tmp/yanetviz-rack.png", fullPage: true });
  await mounted.click();
  await expect(page.getByLabel("Equipment height (U)")).toHaveValue("2");
  await expect(
    page.getByLabel("Starting unit (bottom of equipment)"),
  ).toHaveValue("3");

  await page.getByRole("button", { name: "View mode", exact: true }).click();
  await inventory.getByRole("button", { name: /IV14-SW01/ }).click();
  await page.getByLabel("Equipment height (U)").fill("1");
  await page.getByLabel("Starting unit (bottom of equipment)").fill("4");
  await page.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("overlap at U4");
  await expect(
    page.getByLabel("Starting unit (bottom of equipment)"),
  ).toHaveValue("4");
  const preserved = await (await request.get("/api/plan")).json();
  expect(
    preserved.devices.find(
      (device: { name: string }) => device.name === "IV14-SW01",
    ).rackUnit,
  ).toBeUndefined();
  await page.getByLabel("Starting unit (bottom of equipment)").fill("12");
  await page.getByLabel("Equipment height (U)").fill("2");
  await page.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("beyond cabinet");
  await page.getByLabel("Starting unit (bottom of equipment)").fill("5");
  await page.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Documentation saved");

  await page
    .getByLabel("Cabinet", { exact: true })
    .selectOption({ label: "Attic cabinet" });
  await expect(
    page.getByLabel("Starting unit (bottom of equipment)"),
  ).toHaveValue("");
  await expect(page.getByLabel("Equipment height (U)")).toHaveValue("2");
  await page.getByLabel("Starting unit (bottom of equipment)").fill("1");
  await page.getByLabel("Placement mode").selectOption("mobile");
  await page.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Documentation saved");
  const final = await (await request.get("/api/plan")).json();
  const moved = final.devices.find(
    (device: { name: string }) => device.name === "IV14-SW01",
  );
  expect(moved.rackUnit).toBeUndefined();
  expect(moved.cabinet).toBe("");
  expect(moved.heightU).toBe(2);

  await inventory.getByRole("button", { name: /Basement cabinet/ }).click();
  page.once("dialog", (dialog) => dialog.accept());
  await page
    .getByRole("button", { name: "Delete cabinet…", exact: true })
    .click();
  await page.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Documentation saved");
  const withoutCabinet = await (await request.get("/api/plan")).json();
  const router = withoutCabinet.devices.find(
    (device: { name: string }) => device.name === "IV14-RT01",
  );
  expect(router.cabinet).toBe("");
  expect(router.rackUnit).toBeUndefined();
  expect(router.heightU).toBe(2);
  expect(withoutCabinet.cables).toEqual(final.cables);
});
