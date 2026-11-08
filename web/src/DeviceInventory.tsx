import { observedTime } from "./wifi";
import type { DeviceInventory as Inventory, PortView } from "./ports";

export function observedInventory(
  observations: PortView,
  device: string,
): Inventory | undefined {
  return observations.sources
    .filter((source) => source.device === device && source.inventory)
    .sort(
      (a, b) =>
        Date.parse(b.inventory!.lastAttempt) -
        Date.parse(a.inventory!.lastAttempt),
    )[0]?.inventory;
}

export function DeviceInventory({ inventory }: { inventory?: Inventory }) {
  if (!inventory) return null;
  return (
    <section
      className="device-inventory"
      aria-label="Observed hardware and software"
    >
      <h4>Observed hardware / software</h4>
      <div>Hardware model: {inventory.model || "Unknown"}</div>
      <div>Board name: {inventory.boardName || "Unknown"}</div>
      <div>RouterOS version: {inventory.osVersion || "Unknown"}</div>
      <div>Architecture: {inventory.architecture || "Unknown"}</div>
      {inventory.error && <p className="help">Discovery: {inventory.error}</p>}
      <p className="help">
        Last successful discovery: {observedTime(inventory.lastSuccess)}
        <br />
        Read from the device API; manual model and naming remain unchanged.
      </p>
    </section>
  );
}
