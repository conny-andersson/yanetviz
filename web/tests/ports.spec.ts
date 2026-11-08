import { expect, test } from "@playwright/test";
import type { Plan, Port } from "../src/model";
import type { PortView } from "../src/ports";

test("link and PoE observations preserve manual settings and trace only documented copper passive paths", async ({
  page,
  request,
}) => {
  const original: Plan = await (await request.get("/api/plan")).json();
  const headers = { "X-Yanetviz": "1" };
  try {
    const prepared = structuredClone(original);
    const template = {
      ...prepared.devices[0],
      cabinet: "",
      rackUnit: null,
      floor: "basement",
      mode: "fixed",
    };
    prepared.devices.push(
      {
        ...template,
        id: "poe-source",
        name: "PoE switch",
        role: "switch",
        x: 0.15,
        y: 0.3,
      },
      {
        ...template,
        id: "poe-panel",
        name: "Test panel",
        role: "patch_panel",
        x: 0.4,
        y: 0.4,
      },
      {
        ...template,
        id: "poe-ap",
        name: "Powered AP",
        role: "access_point",
        x: 0.7,
        y: 0.4,
      },
      {
        ...template,
        id: "poe-client",
        name: "Beyond active device",
        role: "client",
        x: 0.8,
        y: 0.7,
      },
    );
    const port = (
      id: string,
      device: string,
      name: string,
      side = "",
    ): Port => ({
      id,
      device,
      name,
      label: "",
      connector: "RJ45",
      position: side ? "1" : "",
      side,
      poeMode: "off",
      poeStandard: "",
      voltage: "",
      notes: "",
    });
    prepared.ports.push(
      port("poe-source-port", "poe-source", "ether1"),
      port("poe-front", "poe-panel", "1 front", "front"),
      port("poe-rear", "poe-panel", "1 rear", "rear"),
      port("poe-ap-in", "poe-ap", "ether1"),
      port("poe-ap-out", "poe-ap", "ether2"),
      port("poe-client-in", "poe-client", "ether1"),
    );
    prepared.passThroughs.push({
      id: "poe-pair",
      a: "poe-front",
      b: "poe-rear",
    });
    const wire = {
      medium: "copper",
      label: "",
      length: null,
      notes: "",
      draft: false,
    };
    prepared.cables.push(
      {
        ...wire,
        id: "poe-patch",
        a: { device: "poe-source", port: "poe-source-port" },
        b: { device: "poe-panel", port: "poe-front" },
      },
      {
        ...wire,
        id: "poe-building",
        a: { device: "poe-panel", port: "poe-rear" },
        b: { device: "poe-ap", port: "poe-ap-in" },
      },
      {
        ...wire,
        id: "poe-beyond",
        a: { device: "poe-ap", port: "poe-ap-out" },
        b: { device: "poe-client", port: "poe-client-in" },
      },
    );
    expect(
      (await request.put("/api/plan", { headers, data: prepared })).ok(),
    ).toBe(true);
    const baseline: Plan = await (await request.get("/api/plan")).json();
    const timestamp = new Date().toISOString();
    const view: PortView = {
      sources: ["link", "poe"].map((channel) => ({
        source: "switch",
        device: "poe-source",
        channel,
        capability: "supported",
        state: "fresh",
        lastAttempt: timestamp,
        lastSuccess: timestamp,
      })),
      readings: [
        {
          source: "switch",
          device: "poe-source",
          channel: "link",
          state: "fresh",
          port: "poe-source-port",
          name: "ether1",
          defaultName: "ether1",
          link: "up",
          rateMbps: 1000,
          fullDuplex: true,
          present: true,
          lastSeen: timestamp,
        },
        {
          source: "switch",
          device: "poe-source",
          channel: "poe",
          state: "fresh",
          port: "poe-source-port",
          name: "ether1",
          defaultName: "ether1",
          poe: {
            mode: "forced",
            vendorMode: "forced-on",
            status: "powered-on",
            voltageV: 47.1,
            currentMA: 94,
            powerW: 4.4,
          },
          present: true,
          lastSeen: timestamp,
        },
      ],
    };
    view.sources.push(
      {
        source: "ap-source",
        device: "poe-ap",
        channel: "link",
        capability: "supported",
        state: "fresh",
        lastAttempt: timestamp,
        lastSuccess: timestamp,
        inventory: {
          model: "TEST-wAP",
          boardName: "wAP",
          osVersion: "7.20.1",
          architecture: "mipsbe",
          lastAttempt: timestamp,
          lastSuccess: timestamp,
        },
      },
      {
        source: "ap-source",
        device: "poe-ap",
        channel: "poe",
        capability: "unsupported",
        state: "unsupported",
        lastAttempt: timestamp,
      },
    );
    await page.route("**/api/observations/live", (route) =>
      route.fulfill({
        json: { wifi: { sources: [], clients: [] }, ports: view },
      }),
    );
    await page.addInitScript(() => {
      class ObservationStream {
        onmessage: ((event: MessageEvent) => void) | null = null;
        constructor() {
          window.addEventListener("port-test-update", this.receive);
        }
        receive = (event: Event) =>
          this.onmessage?.(
            new MessageEvent("message", {
              data: JSON.stringify({
                wifi: { sources: [], clients: [] },
                ports: (event as CustomEvent).detail,
              }),
            }),
          );
        close() {
          window.removeEventListener("port-test-update", this.receive);
        }
      }
      Object.defineProperty(window, "EventSource", {
        value: ObservationStream,
      });
    });
    await page.goto("/");
    await expect(page.locator(".poe-flow")).toHaveCount(2); // Stops at the active AP.
    await page
      .locator(".inventory")
      .getByRole("button", { name: /Powered AP/ })
      .click();
    await expect(
      page.locator(".details-panel > .poe-path-status"),
    ).toContainText("PoE via PoE switch / ether1: Powered");
    await expect(
      page.locator(".details-panel .device-inventory"),
    ).toContainText("Hardware model: TEST-wAP");
    await expect(
      page.locator(".details-panel .device-inventory"),
    ).toContainText("RouterOS version: 7.20.1");
    const receivingPort = page
      .locator(".port-card")
      .filter({ hasText: "ether1" });
    await receivingPort.locator(".port-summary").click();
    await expect(receivingPort.locator(".port-observations")).toContainText(
      "PoE-out not supported on this device (configured)",
    );
    await expect(receivingPort.locator(".poe-path-status")).toContainText(
      "At source: 4.4 W · 47.1 V · 94 mA",
    );
    await expect(receivingPort.locator(".poe-path-status")).toContainText(
      "not measurements at this port or device",
    );

    await page
      .locator(".inventory")
      .getByRole("button", { name: /PoE switch/ })
      .click();
    const card = page.locator(".port-card").filter({ hasText: "ether1" });
    await card.locator(".port-summary").click();
    await expect(card.locator(".port-observations")).toContainText(
      "Link up · 1,000 Mbps · Full duplex",
    );
    await expect(card.locator(".port-observations")).toContainText(
      "4.4 W · 47.1 V · 94 mA",
    );
    await expect(card.getByLabel("Documented PoE-out mode")).toHaveValue("off");
    await page.getByRole("button", { name: "View mode", exact: true }).click();
    await card.getByLabel("Documented PoE-out mode").selectOption("auto");
    await expect(card.locator(".port-observations")).toContainText("forced-on");
    const cable = page.locator(
      '.cable[aria-label="PoE switch / ether1 to Test panel / 1 front"]',
    );
    await cable.press("Enter");
    await expect(page.locator(".cable-observations")).toContainText(
      "4.4 W measured at source",
    );
    await expect(page.locator(".cable-observations")).toContainText(
      "passive compatibility unverified",
    );
    await page.getByLabel("Medium", { exact: true }).selectOption("fiber");
    await expect(page.locator(".poe-flow")).toHaveCount(0);
    await expect(page.locator(".cable-observations")).toContainText(
      "Fiber carries no PoE",
    );
    await page
      .locator(".inventory")
      .getByRole("button", { name: /Powered AP/ })
      .click();
    await expect(page.locator(".poe-path-status")).toHaveCount(0);
    await cable.press("Enter");
    await page.getByRole("button", { name: "Undo", exact: true }).click();
    await expect(page.locator(".poe-flow")).toHaveCount(2);
    // A fiber segment blocks the receiver but not the preceding passive pair.
    const buildingCable = page.locator(
      '.cable[aria-label="Test panel / 1 rear to Powered AP / ether1"]',
    );
    await buildingCable.press("Enter");
    await page.getByLabel("Medium", { exact: true }).selectOption("fiber");
    await expect(page.locator(".poe-flow")).toHaveCount(1);
    await page
      .locator(".inventory")
      .getByRole("button", { name: /Powered AP/ })
      .click();
    await expect(page.locator(".poe-path-status")).toHaveCount(0);
    await page
      .locator(".inventory")
      .getByRole("button", { name: /Test panel/ })
      .click();
    const rearPort = page.locator(".port-card").filter({ hasText: "1 rear" });
    await rearPort.locator(".port-summary").click();
    await expect(rearPort.locator(".poe-path-status")).toContainText(
      "PoE via PoE switch / ether1: Powered",
    );
    await buildingCable.press("Enter");
    await page.getByRole("button", { name: "Undo", exact: true }).click();
    await expect(page.locator(".poe-flow")).toHaveCount(2);
    const waiting = structuredClone(view);
    waiting.readings[1].poe = {
      mode: "auto",
      vendorMode: "auto-on",
      status: "waiting-for-load",
    };
    await page.evaluate(
      (next) =>
        window.dispatchEvent(
          new CustomEvent("port-test-update", { detail: next }),
        ),
      waiting,
    );
    await expect(page.locator(".poe-flow")).toHaveCount(0); // Mode alone never means powered.
    await page
      .locator(".inventory")
      .getByRole("button", { name: /Powered AP/ })
      .click();
    await expect(
      page.locator(".details-panel > .poe-path-status"),
    ).toContainText("PoE via PoE switch / ether1: waiting-for-load");
    await expect(
      page.locator(".details-panel > .poe-path-status"),
    ).toContainText("Power unknown");

    await page
      .locator(".inventory")
      .getByRole("button", { name: /PoE switch/ })
      .click();
    await card.locator(".port-summary").click();
    await expect(card.getByLabel("Documented PoE-out mode")).toHaveValue(
      "auto",
    );
    await expect(card.locator(".port-observations")).toContainText(
      "Power unknown",
    );
    const failed = structuredClone(view);
    failed.sources[1].state = "error";
    failed.sources[1].error = "permission denied";
    failed.readings[1].state = "stale";
    failed.readings[0].link = "down";
    failed.readings[0].rateMbps = undefined;
    failed.readings[0].fullDuplex = undefined;
    await page.evaluate(
      (next) =>
        window.dispatchEvent(
          new CustomEvent("port-test-update", { detail: next }),
        ),
      failed,
    );
    await expect(card.locator(".port-observations")).toContainText("Link down");
    await expect(card.locator(".port-observations")).toContainText(
      "stale; current power unknown",
    );
    await expect(card.locator(".port-observations")).toContainText("4.4 W");
    await page
      .locator(".inventory")
      .getByRole("button", { name: /Powered AP/ })
      .click();
    await expect(
      page.locator(".details-panel > .poe-path-status"),
    ).toContainText("Current power unknown · last reported: Powered (stale)");
    await page
      .getByRole("button", { name: "Links & PoE", exact: true })
      .click();
    await expect(page.locator(".port-overview")).toContainText(
      "permission denied",
    );
    await expect(page.locator(".port-overview")).toContainText(
      "PoE-out not supported (configured)",
    );
    const unchanged: Plan = await (await request.get("/api/plan")).json();
    expect(unchanged).toEqual(baseline);
    await page
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(page.getByRole("status")).toContainText("Documentation saved");
    const saved: Plan = await (await request.get("/api/plan")).json();
    expect(
      saved.ports.find((item) => item.id === "poe-source-port")?.poeMode,
    ).toBe("auto");
    expect(saved.cables).toEqual(baseline.cables);
    expect(saved.devices).toEqual(baseline.devices);
  } finally {
    const current: Plan = await (await request.get("/api/plan")).json();
    expect(
      (
        await request.put("/api/plan", {
          headers,
          data: { ...original, revision: current.revision },
        })
      ).ok(),
    ).toBe(true);
  }
});
