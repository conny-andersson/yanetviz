import { expect, test, type Page } from "@playwright/test";

async function expectPlanToFit(page: Page) {
  await expect(
    page.getByRole("button", { name: "Fit floor plan" }),
  ).toHaveAttribute("aria-pressed", "true");
  // Check the complete drawing after image dimensions and layout have settled.
  await expect
    .poll(async () =>
      page.locator(".map-scroll").evaluate(async (viewport) => {
        const canvas = viewport.querySelector("svg")!;
        const background = canvas.querySelector("image")!;
        const image = new Image();
        image.src = background.getAttribute("href")!;
        await image.decode();
        const bounds = canvas.getBoundingClientRect();
        const frame = viewport.getBoundingClientRect();
        const padding = getComputedStyle(viewport);
        const left = frame.left + parseFloat(padding.paddingLeft);
        const top = frame.top + parseFloat(padding.paddingTop);
        const right = frame.right - parseFloat(padding.paddingRight);
        const bottom = frame.bottom - parseFloat(padding.paddingBottom);
        const aspectMatches =
          Math.abs(
            (bounds.width - 2) / (bounds.height - 2) -
              image.naturalWidth / image.naturalHeight,
          ) < 0.01;
        return (
          bounds.width > 50 &&
          bounds.height > 50 &&
          bounds.left >= left - 1 &&
          bounds.top >= top - 1 &&
          bounds.right <= right + 1 &&
          bounds.bottom <= bottom + 1 &&
          viewport.scrollHeight <= viewport.clientHeight + 1 &&
          viewport.scrollWidth <= viewport.clientWidth + 1 &&
          aspectMatches
        );
      }),
    )
    .toBe(true);
}

test("floor plans fit on floor changes, resizing, and entering Edit mode", async ({
  page,
}) => {
  await page.goto("/");
  for (const name of ["Basement", "Entrance floor", "Attic", "Garage"]) {
    await page
      .getByRole("navigation", { name: "Floors" })
      .getByRole("button", { name, exact: true })
      .click();
    await expectPlanToFit(page);
  }
  await page.getByRole("button", { name: "View mode", exact: true }).click();
  await expectPlanToFit(page);
  for (const size of [
    { width: 1100, height: 700 },
    { width: 1920, height: 1080 },
  ]) {
    await page.setViewportSize(size);
    await expectPlanToFit(page);
  }
});

test("manual zoom resets to fit through the Fit button and floor navigation", async ({
  page,
}) => {
  await page.goto("/");
  await expectPlanToFit(page);
  await page.getByRole("button", { name: "Zoom in", exact: true }).click();
  await expect(page.getByRole("button", { name: "Fit floor plan" })).toHaveText(
    "125%",
  );
  await page.getByRole("button", { name: "Fit floor plan" }).click();
  await expectPlanToFit(page);
  await page.getByRole("button", { name: "Zoom in", exact: true }).click();
  await page
    .getByRole("navigation", { name: "Floors" })
    .getByRole("button", { name: "Entrance floor", exact: true })
    .click();
  await expectPlanToFit(page);
  await page.getByRole("button", { name: "Zoom in", exact: true }).click();
  await page
    .locator(".inventory")
    .getByRole("button", { name: /Attic cabinet/ })
    .click();
  await expectPlanToFit(page);
});
