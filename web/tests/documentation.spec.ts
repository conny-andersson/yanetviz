import { test, expect } from "@playwright/test";

test("document a stationary client, connect its named port, and reload", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/");
  await page.getByRole("button", { name: "View mode", exact: true }).click();
  await page.getByRole("button", { name: "+ Equipment", exact: true }).click();
  await page
    .getByLabel("Friendly name", { exact: true })
    .fill("Entrance desktop");
  await page.getByLabel("Floor", { exact: true }).selectOption("entrance");
  await page
    .getByRole("button", { name: "Entrance floor", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Entrance desktop", exact: true }),
  ).toBeVisible();
  const marker = page
    .locator(".marker")
    .filter({ hasText: "Entrance desktop" });
  const markerBox = await marker.locator("rect").boundingBox();
  expect(markerBox).not.toBeNull();
  await page.mouse.move(
    markerBox!.x + markerBox!.width / 2,
    markerBox!.y + markerBox!.height / 2,
  );
  await page.mouse.down();
  await page.mouse.move(
    markerBox!.x + markerBox!.width / 2 + 45,
    markerBox!.y + markerBox!.height / 2 + 25,
    { steps: 5 },
  );
  await page.mouse.up();
  await page.getByRole("button", { name: "+ Port", exact: true }).click();
  await page.getByLabel("Port name", { exact: true }).fill("Ethernet");
  await page.getByRole("button", { name: "+ Cable", exact: true }).click();
  const endpointA = page.locator(".endpoint-box").nth(0);
  const endpointB = page.locator(".endpoint-box").nth(1);
  await endpointA
    .getByLabel("Port / terminal", { exact: true })
    .selectOption({ label: "Ethernet" });
  await endpointB
    .getByLabel("Equipment", { exact: true })
    .selectOption({ label: "IV14-SW04" });
  await endpointB
    .getByLabel("Port / terminal", { exact: true })
    .selectOption({ label: "Port4 · Desktop" });
  await page.getByLabel("Cable label", { exact: true }).fill("Desktop cable");
  await page.getByLabel("Medium", { exact: true }).selectOption("copper");
  await page.getByLabel("Draft / intermediate routing unresolved").uncheck();
  await page.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Documentation saved");
  const persisted = await (await page.request.get("/api/plan")).json();
  const desktop = persisted.devices.find(
    (device: { name: string }) => device.name === "Entrance desktop",
  );
  expect(desktop.floor).toBe("entrance");
  expect(desktop.x).not.toBeCloseTo(0.5, 3);
  await page.reload();
  await page
    .getByRole("button", { name: "Entrance floor", exact: true })
    .click();
  await expect(
    page.locator(".marker").filter({ hasText: "Entrance desktop" }),
  ).toBeVisible();
  await page.screenshot({
    path: "/tmp/yanetviz-floorplan.png",
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Topology & cables", exact: true })
    .click();
  await expect(page.locator(".cable-register")).toContainText(
    "Entrance desktop / Ethernet → IV14-SW04 / Port4",
  );
  await expect(page.locator(".cable-register")).toContainText("Complete");
  await page
    .getByRole("button", { name: /Desktop cable.*Entrance desktop/ })
    .click();
  await expect(page.locator(".trace")).toContainText(
    "Entrance desktop / Ethernet",
  );
  await page.screenshot({ path: "/tmp/yanetviz-topology.png", fullPage: true });
  expect(errors).toEqual([]);
});

test("conflicting edits keep the pending working copy", async ({ browser }) => {
  const context = await browser.newContext();
  const first = await context.newPage();
  const second = await context.newPage();
  for (const page of [first, second]) {
    await page.goto("/");
    await page.getByRole("button", { name: "View mode", exact: true }).click();
    await page
      .locator(".inventory")
      .getByRole("button", { name: /IV14-AP01/ })
      .click();
  }
  await first
    .getByLabel("Notes", { exact: true })
    .fill("Saved by the first tab");
  await second
    .getByLabel("Notes", { exact: true })
    .fill("Pending in the second tab");
  await first
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(first.getByRole("status")).toContainText("Documentation saved");
  await second
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(second.getByRole("alert")).toContainText(
    "the plan changed in another tab",
  );
  await expect(second.getByLabel("Notes", { exact: true })).toHaveValue(
    "Pending in the second tab",
  );
  // Closing a tab can prompt about pending edits, so discard them explicitly.
  second.on("dialog", (dialog) => dialog.accept());
  await second.getByRole("button", { name: "Reload", exact: true }).click();
  await expect(second.getByLabel("Notes", { exact: true })).toHaveValue(
    "Saved by the first tab",
  );
  await context.close();
});

test("passive positions trace separate front and rear cable segments", async ({
  page,
  request,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "View mode", exact: true }).click();
  await page.getByRole("button", { name: "+ Equipment", exact: true }).click();
  await page
    .getByLabel("Friendly name", { exact: true })
    .fill("Test patch panel");
  await page.getByLabel("Role", { exact: true }).selectOption("patch_panel");
  await page.getByRole("button", { name: "+ Position", exact: true }).click();
  await expect(
    page.getByText("Internal pass-through ↔ 1 rear", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Documentation saved");

  // Set up the surrounding documented cables, then exercise the UI path trace.
  const response = await request.get("/api/plan");
  const plan = await response.json();
  const panel = plan.devices.find(
    (device: { name: string }) => device.name === "Test patch panel",
  );
  const accessPoint = plan.devices.find(
    (device: { name: string }) => device.name === "IV14-AP03",
  );
  const switchDevice = plan.devices.find(
    (device: { name: string }) => device.name === "IV14-SW04",
  );
  const findPort = (device: string, name: string) =>
    plan.ports.find(
      (port: { device: string; name: string }) =>
        port.device === device && port.name === name,
    ).id;
  const apPort = "test-ap-ether1";
  plan.ports.push({
    id: apPort,
    device: accessPoint.id,
    name: "ether1",
    label: "",
    connector: "RJ45",
    position: "",
    side: "",
    poeMode: "unknown",
    poeStandard: "",
    voltage: "",
    notes: "",
  });
  const cable = { medium: "copper", length: null, notes: "", draft: false };
  plan.cables.push({
    ...cable,
    id: "test-panel-patch",
    label: "Panel patch lead",
    a: { device: switchDevice.id, port: findPort(switchDevice.id, "Port2") },
    b: { device: panel.id, port: findPort(panel.id, "1 front") },
  });
  plan.cables.push({
    ...cable,
    id: "test-panel-building",
    label: "Building segment",
    a: { device: panel.id, port: findPort(panel.id, "1 rear") },
    b: { device: accessPoint.id, port: apPort },
  });
  const saved = await request.put("/api/plan", {
    headers: { "X-Yanetviz": "1" },
    data: plan,
  });
  expect(saved.status()).toBe(200);
  await page.reload();
  await page
    .getByRole("button", { name: "Topology & cables", exact: true })
    .click();
  await page
    .locator(".cable-register-row")
    .filter({ hasText: "Panel patch lead" })
    .click();
  await expect(page.locator(".trace")).toContainText(
    "Test patch panel / 1 front",
  );
  await expect(page.locator(".trace")).toContainText(
    "Test patch panel / 1 rear",
  );
  await expect(page.locator(".trace")).toContainText("IV14-AP03 / ether1");
});
