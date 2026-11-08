import { expect, test } from "@playwright/test";
import type { Plan } from "../src/model";
import type { WiFiView } from "../src/wifi";

test("Wi-Fi updates preserve pending documentation and distinguish roaming, stale data and absence", async ({
  page,
  request,
}) => {
  const original: Plan = await (await request.get("/api/plan")).json();
  const headers = { "X-Yanetviz": "1" };
  try {
    const prepared = structuredClone(original);
    const template = prepared.devices[0];
    const base = {
      ...template,
      cabinet: "",
      rackUnit: null,
      mode: "fixed",
      x: 0.25,
      y: 0.35,
    };
    prepared.devices.push(
      {
        ...base,
        id: "wifi-client",
        name: "Wireless client",
        role: "client",
        floor: "basement",
      },
      {
        ...base,
        id: "wifi-mobile",
        name: "Mobile client",
        role: "client",
        floor: "",
        mode: "mobile",
      },
      {
        ...base,
        id: "wifi-ap",
        name: "Test upstairs AP",
        role: "access_point",
        floor: "attic",
      },
      {
        ...base,
        id: "wifi-ap-two",
        name: "Test downstairs AP",
        role: "access_point",
        floor: "basement",
        x: 0.7,
      },
    );
    prepared.interfaces = [
      ...(prepared.interfaces ?? []),
      {
        id: "wireless-identity",
        device: "wifi-client",
        name: "Wi-Fi",
        domain: "home",
        macAddresses: ["02:AA:BB:CC:DD:01"],
      },
      {
        id: "mobile-identity",
        device: "wifi-mobile",
        name: "Wi-Fi",
        domain: "home",
        macAddresses: ["02:AA:BB:CC:DD:03"],
      },
    ];
    expect(
      (await request.put("/api/plan", { headers, data: prepared })).ok(),
    ).toBe(true);
    const baseline: Plan = await (await request.get("/api/plan")).json();
    const lastSeen = new Date().toISOString();
    const initial: WiFiView = {
      sources: [
        {
          source: "controller",
          device: prepared.devices[0].id,
          menu: "interface/wifi/registration-table",
          lastAttempt: lastSeen,
          lastSuccess: lastSeen,
          state: "fresh",
          radios: [
            { mac: "02:11:22:33:44:55", interfaceName: "radio", ap: "wifi-ap" },
          ],
        },
      ],
      clients: [
        {
          source: "controller",
          domain: "home",
          mac: "02:AA:BB:CC:DD:01",
          interfaceName: "radio",
          ap: "wifi-ap",
          device: "wifi-client",
          interface: "wireless-identity",
          ssid: "Test LAN",
          band: "5ghz-ac",
          signal: -62,
          rxRate: 54000000,
          txRate: 72000000,
          lastSeen,
          present: true,
          state: "associated",
        },
        {
          source: "controller",
          domain: "home",
          mac: "02:AA:BB:CC:DD:02",
          interfaceName: "radio",
          ap: "wifi-ap",
          lastSeen,
          present: true,
          state: "associated",
        },
      ],
    };
    initial.clients.push({
      ...initial.clients[0],
      mac: "02:AA:BB:CC:DD:03",
      device: "wifi-mobile",
      interface: "mobile-identity",
    });
    await page.route("**/api/observations/wifi", (route) =>
      route.fulfill({ json: initial }),
    );
    // Drive observation events without modifying the temporary server's plan.
    // The real SSE response contract is also exercised by the Go API tests.
    await page.addInitScript(() => {
      class ObservationStream {
        onmessage: ((event: MessageEvent) => void) | null = null;
        onerror: (() => void) | null = null;
        constructor() {
          window.addEventListener("wifi-test-update", this.receive);
        }
        receive = (event: Event) => {
          this.onmessage?.(
            new MessageEvent("message", {
              data: JSON.stringify((event as CustomEvent).detail),
            }),
          );
        };
        close() {
          window.removeEventListener("wifi-test-update", this.receive);
        }
      }
      Object.defineProperty(window, "EventSource", {
        value: ObservationStream,
      });
    });
    await page.goto("/");
    await page
      .locator(".inventory")
      .getByRole("button", { name: /Wireless client/ })
      .click();
    const details = page.locator(".wifi-details");
    await expect(details).toContainText("Associated");
    await expect(details).toContainText("Test upstairs AP");
    await expect(details).toContainText("54 Mbps / 72 Mbps");
    await expect(details).toContainText("-62 dBm");
    await expect(page.locator(".wireless-link")).toHaveCount(1);
    await page.locator(".wireless-portal").click();
    await expect(
      page
        .getByRole("navigation", { name: "Floors" })
        .getByRole("button", { name: "Attic", exact: true }),
    ).toHaveClass("active");
    await expect(page.getByLabel("Friendly name", { exact: true })).toHaveValue(
      "Wireless client",
    );
    await page.getByRole("button", { name: "View mode", exact: true }).click();
    await page
      .getByLabel("Notes", { exact: true })
      .fill("Keep my pending documentation");
    const roaming = structuredClone(initial);
    roaming.clients[0].ap = "wifi-ap-two";
    await page.evaluate(
      (next) =>
        window.dispatchEvent(
          new CustomEvent("wifi-test-update", { detail: next }),
        ),
      roaming,
    );
    await expect(details).toContainText("Test downstairs AP");
    await expect(page.getByLabel("Notes", { exact: true })).toHaveValue(
      "Keep my pending documentation",
    );
    await page
      .getByRole("button", { name: "Wi-Fi collection", exact: true })
      .click();
    await expect(page.locator(".wifi-overview")).toContainText(
      "Unmatched client",
    );
    await expect(page.locator(".wifi-overview")).toContainText(
      "02:AA:BB:CC:DD:02",
    );
    const stale = structuredClone(roaming);
    stale.sources[0].state = "error";
    stale.sources[0].error = "connection failed";
    stale.clients.forEach((client) => (client.state = "stale"));
    await page.evaluate(
      (next) =>
        window.dispatchEvent(
          new CustomEvent("wifi-test-update", { detail: next }),
        ),
      stale,
    );
    await expect(details).toContainText("Stale · current association unknown");
    await expect(page.locator(".wifi-overview")).toContainText(
      "connection failed",
    );
    await page.getByRole("button", { name: "Floor plan", exact: true }).click();
    await page
      .getByRole("navigation", { name: "Floors" })
      .getByRole("button", { name: "Basement", exact: true })
      .click();
    await expect(page.locator(".wireless-link.stale")).toHaveCount(1);
    const absent = structuredClone(roaming);
    absent.clients[0].state = "not-associated";
    absent.clients[0].present = false;
    await page.evaluate(
      (next) =>
        window.dispatchEvent(
          new CustomEvent("wifi-test-update", { detail: next }),
        ),
      absent,
    );
    await expect(details).toContainText("Not associated");
    await expect(page.locator(".wireless-link")).toHaveCount(0);
    const beforeSave: Plan = await (await request.get("/api/plan")).json();
    expect(beforeSave.devices).toEqual(baseline.devices);
    expect(beforeSave.cables).toEqual(prepared.cables);
    await page
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(page.getByRole("status")).toContainText("Documentation saved");
    const saved: Plan = await (await request.get("/api/plan")).json();
    expect(
      saved.devices.find((device) => device.id === "wifi-client")?.floor,
    ).toBe("basement");
    expect(
      saved.devices.find((device) => device.id === "wifi-client")?.notes,
    ).toBe("Keep my pending documentation");
    expect(saved.devices).toHaveLength(prepared.devices.length);
    expect(saved.revision).toBe(beforeSave.revision + 1);
    await page
      .locator(".inventory")
      .getByRole("button", { name: /Mobile client/ })
      .click();
    await expect(details).toContainText("Test upstairs AP");
    await expect(page.locator(".wireless-link")).toHaveCount(0);
    await expect(
      page.getByLabel("Placement mode", { exact: true }),
    ).toHaveValue("mobile");
    expect(
      saved.devices.find((device) => device.id === "wifi-mobile")?.floor,
    ).toBe("");
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
