import { expect, test } from "@playwright/test";

for (const kind of ["device", "cabinet"] as const) {
  test(`${kind} selection ignores clicks and jitter, while dragging preserves the grab offset`, async ({
    page,
    request,
  }) => {
    // Seed a placed marker through the public API in the temporary test database.
    const plan = await (await request.get("/api/plan")).json();
    const entity =
      kind === "device"
        ? plan.devices.find(
            (device: { name: string }) => device.name === "IV14-AP04",
          )
        : plan.cabinets.find(
            (cabinet: { name: string }) => cabinet.name === "Basement cabinet",
          );
    Object.assign(entity, {
      floor: "basement",
      x: kind === "cabinet" ? 0.25 : 0.45,
      y: 0.4,
    });
    if (kind === "cabinet") entity.placed = true;
    const saved = await request.put("/api/plan", {
      headers: { "X-Yanetviz": "1" },
      data: plan,
    });
    expect(saved.ok()).toBe(true);

    await page.goto("/");
    await page.getByRole("button", { name: "View mode", exact: true }).click();
    const marker = page.locator(".marker").filter({ hasText: entity.name });
    const save = page.getByRole("button", {
      name: "Save changes",
      exact: true,
    });
    const original = await marker.getAttribute("transform");
    await marker.locator("rect").click({ position: { x: 2, y: 3 } });
    await expect(page.locator(".details-panel h2")).toHaveText(entity.name);
    await expect(marker).toHaveAttribute("transform", original!);
    await expect(save).toBeDisabled();
    await marker.locator(".marker-name").click();
    await expect(marker).toHaveAttribute("transform", original!);
    await expect(save).toBeDisabled();

    const bounds = await marker.locator("rect").boundingBox();
    expect(bounds).not.toBeNull();
    const x = bounds!.x + 3;
    const y = bounds!.y + 4;
    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x + 1, y + 1);
    await page.mouse.up();
    await expect(marker).toHaveAttribute("transform", original!);
    await expect(save).toBeDisabled();

    const matrix = await page.locator(".floor-map").evaluate((element) => {
      const transform = (element as SVGSVGElement).getScreenCTM()!;
      return { xScale: transform.a, yScale: transform.d };
    });
    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x + 40, y + 30, { steps: 5 });
    await page.mouse.up();
    await expect(save).toBeEnabled();
    const translated = await marker.getAttribute("transform");
    const coordinates = translated!.match(/translate\(([^ ]+) ([^)]+)\)/)!;
    const previous = original!.match(/translate\(([^ ]+) ([^)]+)\)/)!;
    expect(Number(coordinates[1]) - Number(previous[1])).toBeCloseTo(
      40 / matrix.xScale,
      3,
    );
    expect(Number(coordinates[2]) - Number(previous[2])).toBeCloseTo(
      30 / matrix.yScale,
      3,
    );
    await save.click();
    await expect(page.getByRole("status")).toContainText("Documentation saved");
    await page.reload();
    await expect(marker).toHaveAttribute("transform", translated!);

    // View mode allows selecting the marker, without changing its placement.
    await marker.locator("rect").hover();
    await page.mouse.down();
    await page.mouse.move(x + 70, y + 70, { steps: 3 });
    await page.mouse.up();
    await expect(marker).toHaveAttribute("transform", translated!);
  });
}
