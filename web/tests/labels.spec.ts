import { expect, test } from "@playwright/test";

test("auto-label previews unsaved short names, preserves manual labels, and requires an explicit save", async ({
  page,
  request,
}) => {
  const original = await (await request.get("/api/plan")).json();
  try {
    const plan = structuredClone(original);
    const cabinet = plan.cabinets.find(
      (cabinet: { name: string }) => cabinet.name === "Basement cabinet",
    );
    Object.assign(cabinet, { heightU: 12, shortName: "R01" });
    const first = plan.devices.find(
      (device: { name: string }) => device.name === "IV14-RT01",
    );
    const second = plan.devices.find(
      (device: { name: string }) => device.name === "IV14-SW01",
    );
    Object.assign(first, {
      cabinet: cabinet.id,
      floor: "",
      heightU: 1,
      rackUnit: 3,
    });
    Object.assign(second, {
      cabinet: cabinet.id,
      floor: "",
      heightU: 1,
      rackUnit: 12,
    });
    const pump = {
      id: "pump",
      name: "PumpRoom",
      role: "client",
      mode: "fixed",
      floor: "",
      cabinet: "",
      model: "",
      address: "",
      notes: "",
      x: 0.5,
      y: 0.5,
    };
    const desktop = {
      ...pump,
      id: "desktop",
      name: "Desktop",
      shortName: "ds01",
    };
    plan.devices.push(pump, desktop);
    const addPort = (device: string, id: string, name: string) => {
      plan.ports.push({
        id,
        device,
        name,
        label: "",
        connector: "",
        position: "",
        side: "",
        poeMode: "unknown",
        poeStandard: "",
        voltage: "",
        notes: "",
      });
      return id;
    };
    const ap = addPort(first.id, "first-label-port", "ether5");
    const bp = addPort(second.id, "second-label-port", "ether5");
    const pp = addPort(pump.id, "pump-port", "ether1");
    const dp = addPort(desktop.id, "desktop-port", "ether1");
    const sfpA = addPort(first.id, "fiber-a", "sfp-sfpplus1");
    const sfpB = addPort(second.id, "fiber-b", "SFP+1");
    addPort(first.id, "copper-one", "ether1");
    const cable = { medium: "copper", length: null, notes: "", draft: false };
    plan.cables = [
      {
        ...cable,
        id: "rack-cable",
        label: "",
        a: { device: first.id, port: ap },
        b: { device: second.id, port: bp },
      },
      {
        ...cable,
        id: "manual-cable",
        label: "Keep this label",
        a: { device: pump.id, port: pp },
        b: { device: desktop.id, port: dp },
      },
      {
        ...cable,
        id: "draft-cable",
        label: "",
        draft: true,
        a: { device: first.id, port: "" },
        b: { device: desktop.id, port: "" },
      },
      {
        ...cable,
        medium: "fiber",
        id: "fiber-cable",
        label: "",
        a: { device: first.id, port: sfpA },
        b: { device: second.id, port: sfpB },
      },
    ];
    plan.passThroughs = [];
    const prepared = await request.put("/api/plan", {
      headers: { "X-Yanetviz": "1" },
      data: plan,
    });
    expect(prepared.ok()).toBe(true);

    await page.goto("/");
    await page.getByRole("button", { name: "View mode", exact: true }).click();
    await page
      .locator(".inventory")
      .getByRole("button", { name: /Basement cabinet/ })
      .click();
    await page.getByLabel("Short name (optional)").fill("R02");
    await page
      .getByRole("button", { name: "Auto-label cables…", exact: true })
      .click();
    const dialog = page.getByRole("dialog", { name: "Auto-label cables" });
    await expect(dialog).toContainText("R02:U03:P05-R02:U12:P05");
    await expect(dialog).toContainText("R02:U03:Psfp01-R02:U12:Psfp01");
    await expect(dialog).toContainText("PumpRoom:P01-ds01:P01");
    await expect(dialog).toContainText("Keep this label · kept");
    await expect(dialog).toContainText("endpoint port is unknown");
    const previewOnly = await (await request.get("/api/plan")).json();
    expect(previewOnly.cables[0].label).toBe("");
    expect(
      previewOnly.cabinets.find(
        (entry: { id: string }) => entry.id === cabinet.id,
      ).shortName,
    ).toBe("R01");
    await page.screenshot({ path: "/tmp/yanetviz-labels.png", fullPage: true });
    await dialog
      .getByRole("button", { name: "Apply 2 labels", exact: true })
      .click();
    expect(
      (await (await request.get("/api/plan")).json()).cables[0].label,
    ).toBe("");
    await page
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(page.getByRole("status")).toContainText("Documentation saved");
    let saved = await (await request.get("/api/plan")).json();
    expect(saved.cables[0].label).toBe("R02:U03:P05-R02:U12:P05");
    expect(saved.cables[1].label).toBe("Keep this label");
    expect(saved.cables[2].label).toBe("");
    expect(saved.cables[3].label).toBe("R02:U03:Psfp01-R02:U12:Psfp01");

    await page
      .getByRole("button", { name: "Topology & cables", exact: true })
      .click();
    await page
      .locator(".cable-register-row")
      .filter({ hasText: "Keep this label" })
      .click();
    await page
      .getByRole("button", { name: "Auto-label cable…", exact: true })
      .click();
    await expect(
      dialog.getByRole("button", { name: "Apply 0 labels" }),
    ).toBeDisabled();
    await dialog.getByLabel("Allow replacement of existing labels").check();
    await dialog
      .getByRole("button", { name: "Apply 1 label", exact: true })
      .click();
    await expect(page.getByLabel("Cable label", { exact: true })).toHaveValue(
      "PumpRoom:P01-ds01:P01",
    );
    await page
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(page.getByRole("status")).toContainText("Documentation saved");
    saved = await (await request.get("/api/plan")).json();
    expect(saved.cables[1].label).toBe("PumpRoom:P01-ds01:P01");
    await page.reload();
    await page
      .getByRole("button", { name: "Topology & cables", exact: true })
      .click();
    await expect(page.locator(".cable-register")).toContainText(
      "R02:U03:P05-R02:U12:P05",
    );
  } finally {
    const current = await (await request.get("/api/plan")).json();
    const restored = await request.put("/api/plan", {
      headers: { "X-Yanetviz": "1" },
      data: { ...original, revision: current.revision },
    });
    expect(restored.ok()).toBe(true);
  }
});
