import { expect, test } from "@playwright/test";
import type { Plan } from "../src/model";
import type { WiFiView } from "../src/wifi";
import type { PortView } from "../src/ports";

test("stream reconnects preserve readings until the 180-second freshness deadline", async ({
  page,
  request,
}) => {
  const plan: Plan = await (await request.get("/api/plan")).json();
  const start = new Date("2026-11-08T10:00:00Z");
  const timestamp = start.toISOString();
  const view: { wifi: WiFiView; ports: PortView } = {
    wifi: {
      sources: [
        {
          source: "controller",
          device: plan.devices[0].id,
          state: "fresh",
          lastSuccess: timestamp,
          radios: [],
        },
      ],
      clients: [
        {
          source: "controller",
          domain: "home",
          mac: "02:AA:BB:CC:DD:01",
          interfaceName: "radio",
          ssid: "Test LAN",
          present: true,
          state: "associated",
          lastSeen: timestamp,
        },
      ],
    },
    ports: {
      sources: [
        {
          source: "switch",
          device: plan.ports[0].device,
          channel: "link",
          capability: "supported",
          state: "fresh",
          lastSuccess: timestamp,
        },
      ],
      readings: [
        {
          source: "switch",
          device: plan.ports[0].device,
          channel: "link",
          port: plan.ports[0].id,
          name: "ether1",
          link: "up",
          state: "fresh",
          lastSeen: timestamp,
          present: true,
        },
      ],
    },
  };
  await page.clock.install({ time: start });
  await page.route("**/api/observations/live", (route) =>
    route.fulfill({ json: view }),
  );
  await page.addInitScript(() => {
    class ObservationStream {
      onmessage: ((event: MessageEvent) => void) | null = null;
      onerror: (() => void) | null = null;
      onopen: (() => void) | null = null;
      constructor() {
        window.addEventListener("stream-test", this.receive);
      }
      receive = (event: Event) => {
        const detail = (event as CustomEvent).detail;
        if (detail === "error") this.onerror?.();
        else if (detail === "open") this.onopen?.();
        else
          this.onmessage?.(
            new MessageEvent("message", { data: JSON.stringify(detail) }),
          );
      };
      close() {
        window.removeEventListener("stream-test", this.receive);
      }
    }
    Object.defineProperty(window, "EventSource", { value: ObservationStream });
  });
  await page.goto("/");
  await page
    .getByRole("button", { name: "Wi-Fi collection", exact: true })
    .click();
  const wifiRow = page.locator(".wifi-overview tbody tr");
  await expect(wifiRow).toContainText("Associated");

  // The server renews its stream every minute. Neither the renewal error nor
  // temporary transport loss is evidence that the source's successful poll aged.
  await page.clock.fastForward(55000);
  await page.evaluate(() =>
    window.dispatchEvent(new CustomEvent("stream-test", { detail: "error" })),
  );
  await expect(page.locator(".wifi-overview")).toContainText("Reconnecting");
  await expect(wifiRow).toContainText("Associated");
  await expect(wifiRow).not.toContainText("Stale");
  await page.getByRole("button", { name: "Links & PoE", exact: true }).click();
  await expect(page.locator(".port-overview tbody tr")).toContainText(
    "Link up",
  );
  await expect(page.locator(".port-overview tbody tr")).not.toContainText(
    "stale",
  );
  await page
    .getByRole("button", { name: "Wi-Fi collection", exact: true })
    .click();
  await page.clock.fastForward(5000);
  await expect(wifiRow).toContainText("Associated");
  await page.evaluate(() =>
    window.dispatchEvent(new CustomEvent("stream-test", { detail: "open" })),
  );
  await expect(page.locator(".wifi-overview")).not.toContainText(
    "Reconnecting",
  );

  // No new snapshots: keep the last reading through 179 seconds, expire at 180.
  await page.clock.fastForward(119000);
  await expect(wifiRow).toContainText("Associated");
  await page.clock.fastForward(1000);
  await expect(wifiRow).toContainText("Stale · current association unknown");
  await page.getByRole("button", { name: "Links & PoE", exact: true }).click();
  await expect(page.locator(".port-overview tbody tr")).toContainText(
    "stale; current status unknown",
  );
  await page
    .getByRole("button", { name: "Wi-Fi collection", exact: true })
    .click();

  const refreshed = structuredClone(view);
  const next = new Date(start.getTime() + 180000).toISOString();
  refreshed.wifi.sources[0].lastSuccess = next;
  refreshed.wifi.clients[0].lastSeen = next;
  refreshed.ports.sources[0].lastSuccess = next;
  refreshed.ports.readings[0].lastSeen = next;
  await page.evaluate(
    (snapshot) =>
      window.dispatchEvent(
        new CustomEvent("stream-test", { detail: snapshot }),
      ),
    refreshed,
  );
  await expect(wifiRow).toContainText("Associated");
  await expect(wifiRow).not.toContainText("Stale");
  expect(await (await request.get("/api/plan")).json()).toEqual(plan);
});
