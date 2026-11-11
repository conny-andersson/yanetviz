import { location, type Cable, type Plan, type RouteSection } from "./model";

export function cableLocation(plan: Plan, device: string) {
  const equipment = plan.devices.find((item) => item.id === device);
  return equipment ? location(plan, equipment) : null;
}

export function isWithinCabinet(plan: Plan, cable: Cable): boolean {
  const a = plan.devices.find((device) => device.id === cable.a.device);
  const b = plan.devices.find((device) => device.id === cable.b.device);
  return !!a?.cabinet && a.cabinet === b?.cabinet;
}

// Legacy cables acquire sections only when the user edits their drawing. Levels
// never change an existing route, and automatic geometry is not confirmation.
export function routeSections(plan: Plan, cable: Cable): RouteSection[] {
  // Cabinet patch leads remain part of physical traces, but have no floor-plan
  // drawing. Retain saved geometry in case cabinet membership changes later.
  if (isWithinCabinet(plan, cable)) return [];
  if (cable.sections?.length) return cable.sections;
  const floors = [
    cableLocation(plan, cable.a.device)?.floor,
    cableLocation(plan, cable.b.device)?.floor,
  ];
  return [...new Set(floors.filter((floor): floor is string => !!floor))].map(
    (floor) => ({ floor, confirmed: false }),
  );
}

export function replaceSections(
  plan: Plan,
  id: string,
  sections: RouteSection[],
): Plan {
  return {
    ...plan,
    cables: plan.cables.map((cable) =>
      cable.id === id ? { ...cable, sections } : cable,
    ),
  };
}
