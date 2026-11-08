import { expect, test, type Locator, type Page } from "@playwright/test";
import type { Plan } from "../src/model";

async function drag(page: Page, marker: Locator, dx: number, dy: number) {
  const box = await marker.boundingBox();
  expect(box).not.toBeNull();
  const x = box!.x + box!.width / 2;
  const y = box!.y + box!.height / 2;
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x + dx, y + dy, { steps: 5 });
  await page.mouse.up();
}

test("route portals, bends and explicitly chosen intermediate areas survive saves and conflicts", async ({
  page,
  request,
}) => {
  const original: Plan = await (await request.get("/api/plan")).json();
  const headers = { "X-Yanetviz": "1" };
  try {
    const prepared = structuredClone(original);
    prepared.floors.forEach((floor) => delete floor.level);
    const equipment = prepared.devices[0];
    prepared.devices.push(
      {
        ...equipment,
        id: "route-a",
        name: "Route A",
        cabinet: "",
        rackUnit: null,
        mode: "fixed",
        floor: "basement",
        x: 0.2,
        y: 0.3,
      },
      {
        ...equipment,
        id: "route-b",
        name: "Route B",
        cabinet: "",
        rackUnit: null,
        mode: "fixed",
        floor: "attic",
        x: 0.4,
        y: 0.6,
      },
    );
    prepared.cables = [
      {
        id: "route-test",
        a: { device: "route-a", port: "" },
        b: { device: "route-b", port: "" },
        label: "Route test",
        medium: "fiber",
        length: null,
        notes: "",
        draft: true,
      },
    ];
    expect(
      (await request.put("/api/plan", { headers, data: prepared })).ok(),
    ).toBe(true);
    await page.goto("/");
    await page.getByRole("button", { name: "View mode", exact: true }).click();
    const level = page.getByLabel("Floor level (optional)");
    await expect(level).toHaveValue("");
    const tabs = page.getByRole("navigation", { name: "Floors" });
    for (const [name, value] of [
      ["Basement", "-1"],
      ["Entrance floor", "0"],
      ["Attic", "1"],
      ["Garage", "0"],
    ]) {
      await tabs.getByRole("button", { name, exact: true }).click();
      await level.fill(value);
    }
    await tabs.getByRole("button", { name: "Basement", exact: true }).click();
    const cable = page.locator(".cable");
    const portal = cable.locator(".portal circle");
    const save = page.getByRole("button", {
      name: "Save changes",
      exact: true,
    });
    await save.click();
    await expect(page.getByRole("status")).toContainText("Documentation saved");
    const initialX = await portal.getAttribute("cx");
    await portal.click();
    await expect(
      tabs.getByRole("button", { name: "Basement", exact: true }),
    ).toHaveClass("active");
    await expect(portal).toHaveAttribute("cx", initialX!);
    await expect(save).toBeDisabled();
    await drag(page, portal, -70, 40);
    await expect(save).toBeEnabled();
    const movedX = await portal.getAttribute("cx");
    expect(movedX).not.toBe(initialX);
    await page.getByRole("button", { name: "Undo", exact: true }).click();
    await expect(portal).toHaveAttribute("cx", initialX!);
    await drag(page, portal, -70, 40);
    const pathPoint = await cable.locator(".cable-hit").evaluate((element) => {
      const path = element as SVGPathElement;
      const point = path.getPointAtLength(path.getTotalLength() * 0.5);
      const screen = new DOMPoint(point.x, point.y).matrixTransform(
        path.getScreenCTM()!,
      );
      return { x: screen.x, y: screen.y };
    });
    await page.mouse.dblclick(pathPoint.x, pathPoint.y);
    await expect(cable.locator(".route-handle")).toHaveCount(1);
    await drag(page, cable.locator(".route-handle circle"), 0, 40);
    const bendX = await cable
      .locator(".route-handle circle")
      .getAttribute("cx");
    const bendY = await cable
      .locator(".route-handle circle")
      .getAttribute("cy");
    const routing = page.locator(".routing-editor");
    const chooser = routing.getByLabel(
      "Insert intermediate floor before endpoint B",
    );
    await expect(chooser).toHaveValue("");
    await expect(chooser.locator("option")).toHaveText([
      "Choose a floor / area…",
      "Entrance floor · level 0 · intermediate level",
      "Garage · level 0 · intermediate level",
    ]);
    await expect(routing.locator("li")).toHaveCount(2);
    await chooser.selectOption("entrance");
    await expect(routing.locator("li")).toHaveCount(3);
    await routing.locator("li").first().getByRole("checkbox").check();
    const marker = page
      .locator(".marker")
      .filter({ hasText: "Route A" })
      .locator("rect");
    const beforeDeviceMove = await cable
      .locator(".cable-line")
      .getAttribute("d");
    await drag(page, marker, 30, 20);
    await cable.press("Enter");
    expect(await cable.locator(".cable-line").getAttribute("d")).not.toBe(
      beforeDeviceMove,
    );
    await expect(cable.locator(".route-handle circle")).toHaveAttribute(
      "cx",
      bendX!,
    );
    await expect(cable.locator(".route-handle circle")).toHaveAttribute(
      "cy",
      bendY!,
    );
    await page.getByRole("button", { name: "Zoom in", exact: true }).click();
    await expect(cable.locator(".route-handle circle")).toHaveAttribute(
      "cx",
      bendX!,
    );
    await routing
      .getByRole("button", { name: "Entrance floor", exact: true })
      .click();
    await expect(cable.locator(".portal circle")).toHaveCount(2);
    await drag(page, cable.locator(".portal circle").first(), -60, -40);
    await drag(page, cable.locator(".portal circle").last(), -20, 40);
    await routing.locator("li").nth(1).getByRole("checkbox").check();
    // Floor metadata must not rewrite a confirmed route.
    await level.fill("5");
    await save.click();
    await expect(page.getByRole("status")).toContainText("Documentation saved");
    const saved: Plan = await (await request.get("/api/plan")).json();
    expect(saved.cables).toHaveLength(1);
    expect(saved.cables[0].a).toEqual(prepared.cables[0].a);
    expect(saved.cables[0].sections?.map((section) => section.floor)).toEqual([
      "basement",
      "entrance",
      "attic",
    ]);
    expect(saved.cables[0].sections?.[1].confirmed).toBe(true);
    await page.reload();
    // Navigation in View mode retains the cable selection across both portals.
    await tabs.getByRole("button", { name: "Basement", exact: true }).click();
    await cable.locator(".portal circle").click();
    await expect(
      tabs.getByRole("button", { name: "Entrance floor", exact: true }),
    ).toHaveClass("active");
    await expect(page.locator(".details-panel h2")).toHaveText("Route test");
    await cable.locator(".portal circle").last().click();
    await expect(
      tabs.getByRole("button", { name: "Attic", exact: true }),
    ).toHaveClass("active");
    await expect(page.locator(".details-panel h2")).toHaveText("Route test");
    await page.getByRole("button", { name: "View mode", exact: true }).click();
    await tabs.getByRole("button", { name: "Basement", exact: true }).click();
    await expect(cable.locator(".route-handle circle")).toHaveAttribute(
      "cx",
      bendX!,
    );
    await expect(cable.locator(".route-handle circle")).toHaveAttribute(
      "cy",
      bendY!,
    );
    await drag(page, cable.locator(".route-handle circle"), 15, 15);
    const pendingPath = await cable.locator(".cable-line").getAttribute("d");
    expect(
      (await request.put("/api/plan", { headers, data: saved })).ok(),
    ).toBe(true);
    await save.click();
    await expect(page.getByRole("alert")).toContainText(
      "the plan changed in another tab",
    );
    await expect(cable.locator(".cable-line")).toHaveAttribute(
      "d",
      pendingPath!,
    );
    const current: Plan = await (await request.get("/api/plan")).json();
    expect(current.cables).toEqual(saved.cables);
    page.once("dialog", (dialog) => dialog.accept());
    await page.getByRole("button", { name: "Reload", exact: true }).click();
    await expect(cable.locator(".route-handle circle")).toHaveAttribute(
      "cx",
      bendX!,
    );
    await page.screenshot({
      path: "/tmp/yanetviz-cable-routing.png",
      fullPage: true,
    });
    await cable.locator(".route-handle").dblclick();
    await expect(cable.locator(".route-handle")).toHaveCount(0);
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
