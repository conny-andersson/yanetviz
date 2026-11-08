import { expect, test } from "@playwright/test";

test("interfaces normalize MAC aliases and show DHCP evidence without replacing documentation", async ({
  page,
  request,
}) => {
  const original = await (await request.get("/api/plan")).json();
  try {
    const prepared = structuredClone(original);
    const owner = prepared.devices.find(
      (device: { name: string }) => device.name === "IV14-AP01",
    );
    const port = {
      id: "interface-port",
      device: owner.id,
      name: "client-ether1",
      label: "",
      connector: "RJ45",
      position: "",
      side: "",
      poeMode: "unknown",
      poeStandard: "",
      voltage: "",
      notes: "",
    };
    const foreignPort = {
      ...port,
      id: "foreign-interface-port",
      device: prepared.devices[0].id,
      name: "foreign-port",
    };
    prepared.ports.push(port, foreignPort);
    const setup = await request.put("/api/plan", {
      headers: { "X-Yanetviz": "1" },
      data: prepared,
    });
    expect(setup.ok()).toBe(true);
    await page.goto("/");
    await page.getByRole("button", { name: "View mode", exact: true }).click();
    const inventory = page.locator(".inventory");
    await inventory.getByRole("button", { name: /IV14-AP01/ }).click();
    await page
      .getByRole("button", { name: "+ Interface", exact: true })
      .click();
    const section = page.locator(".network-identities");
    await section.locator("summary").click();
    await section
      .getByLabel("Interface name", { exact: true })
      .fill("Client Wi-Fi");
    await section.getByLabel("Network identity domain").fill(" HOME ");
    const portSelector = section.getByLabel("Physical port (optional)");
    await expect(portSelector).toHaveValue("");
    await expect(portSelector.locator("option")).toHaveText([
      "No port / wireless / unknown",
      "client-ether1",
    ]);
    await portSelector.selectOption(port.id);
    await section
      .getByLabel("MAC aliases (one per line)")
      .fill("02-11-22-33-44-55\n02:11:22:33:44:66");
    await page
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(page.getByRole("status")).toContainText("Documentation saved");
    let saved = await (await request.get("/api/plan")).json();
    const item = saved.interfaces.find(
      (item: { name: string }) => item.name === "Client Wi-Fi",
    );
    expect(item.domain).toBe("home");
    expect(item.port).toBe(port.id);
    expect(item.macAddresses).toEqual([
      "02:11:22:33:44:55",
      "02:11:22:33:44:66",
    ]);
    const record = {
      interface: item.id,
      domain: "home",
      source: saved.devices[0].id,
      server: "test-lan",
      macAddress: "02:11:22:33:44:55",
      clientId: "01:Ab:00:FF",
      clientIdField: "active-client-id",
      address: "192.0.2.10",
      hostname: "observed-host",
      status: "bound",
      firstSeen: "2026-11-08T09:00:00Z",
      lastSeen: "2026-11-08T10:00:00Z",
    };
    const collected = await request.post("/api/observations/dhcp", {
      headers: { "X-Yanetviz": "1" },
      data: [record],
    });
    expect(collected.ok()).toBe(true);
    expect((await (await request.get("/api/plan")).json()).revision).toBe(
      saved.revision,
    );
    await page.reload();
    await page.getByLabel("Search equipment").fill("192.0.2.10");
    await inventory.getByRole("button", { name: /IV14-AP01/ }).click();
    await section.locator("summary").click();
    await expect(portSelector).toHaveValue(port.id);
    await expect(section).toContainText("01:Ab:00:FF");
    await expect(section).toContainText("observed-host");
    await expect(page.getByLabel("Friendly name", { exact: true })).toHaveValue(
      "IV14-AP01",
    );
    await page.screenshot({
      path: "/tmp/yanetviz-interface.png",
      fullPage: true,
    });
    await page.getByRole("button", { name: "View mode", exact: true }).click();
    await page
      .getByLabel("Notes", { exact: true })
      .fill("Keep my documentation");
    await page
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(page.getByRole("status")).toContainText("Documentation saved");
    expect(
      await (await request.get("/api/observations/dhcp")).json(),
    ).toHaveLength(1);
    const physicalCard = page
      .locator(".port-card")
      .filter({ hasText: "client-ether1" });
    await physicalCard.locator(".port-summary").click();
    await expect(physicalCard).toContainText(
      "Network interfaces: Client Wi-Fi",
    );
    page.once("dialog", async (dialog) => {
      expect(dialog.message()).toContain("unlinks 1 network interface");
      await dialog.accept();
    });
    await physicalCard
      .getByRole("button", { name: "Remove terminal", exact: true })
      .click();
    await expect(portSelector).toHaveValue("");
    await page
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(page.getByRole("status")).toContainText("Documentation saved");
    expect(
      await (await request.get("/api/observations/dhcp")).json(),
    ).toHaveLength(1);
    await page
      .getByRole("button", { name: "+ Interface", exact: true })
      .click();
    const duplicate = section.locator(".interface-card").last();
    await duplicate.locator("summary").click();
    await duplicate
      .getByLabel("MAC aliases (one per line)")
      .fill("02:11:22:33:44:55");
    await page
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(page.getByRole("alert")).toContainText(
      "claimed more than once",
    );
    await duplicate.getByLabel("Network identity domain").fill("guest");
    await page
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(page.getByRole("status")).toContainText("Documentation saved");
    page.once("dialog", (dialog) => dialog.accept());
    await section
      .locator(".interface-card")
      .first()
      .getByRole("button", { name: "Remove interface…", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(page.getByRole("status")).toContainText("Documentation saved");
    expect(
      await (await request.get("/api/observations/dhcp")).json(),
    ).toHaveLength(0);
    saved = await (await request.get("/api/plan")).json();
    expect(saved.devices).toHaveLength(original.devices.length);
    expect(saved.ports).toEqual([...original.ports, foreignPort]);
    expect(saved.cables).toEqual(original.cables);
  } finally {
    const current = await (await request.get("/api/plan")).json();
    const restored = await request.put("/api/plan", {
      headers: { "X-Yanetviz": "1" },
      data: { ...original, revision: current.revision },
    });
    expect(restored.ok()).toBe(true);
  }
});
