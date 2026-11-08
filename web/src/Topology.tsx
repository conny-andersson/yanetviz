import { endpoint, roles, type Plan, type Selection } from "./model";
import { rackLabel } from "./Rack";

export type CableSort = "original" | "ascending" | "descending";
const labelCollator = new Intl.Collator(undefined, {
  numeric: true,
  sensitivity: "base",
});

export function Topology({
  plan,
  selection,
  select,
  cableSort,
  setCableSort,
}: {
  plan: Plan;
  selection: Selection;
  select: (selection: Selection) => void;
  cableSort: CableSort;
  setCableSort: (order: CableSort) => void;
}) {
  // Sort a copy for display; saved cable order and endpoint order stay intact.
  const cables =
    cableSort === "original"
      ? plan.cables
      : [...plan.cables].sort((left, right) => {
          const a = left.label.trim();
          const b = right.label.trim();
          if (!a || !b) return Number(!a) - Number(!b);
          return (
            labelCollator.compare(a, b) * (cableSort === "descending" ? -1 : 1)
          );
        });
  return (
    <div className="topology-view">
      <div className="topology-groups">
        {[
          ...plan.floors.map((floor) => ({
            id: floor.id,
            name: floor.name,
          })),
          { id: "", name: "Mobile / unplaced" },
        ].map((group) => {
          const devices = plan.devices.filter((device) => {
            const cabinet = plan.cabinets.find(
              (cabinet) => cabinet.id === device.cabinet,
            );
            return (cabinet?.floor || device.floor || "") === group.id;
          });
          return (
            <section className="topology-group" key={group.id}>
              <span className="eyebrow">{group.name}</span>
              {devices.map((device) => (
                <button
                  key={device.id}
                  className={`topology-device ${selection?.id === device.id ? "active" : ""}`}
                  onClick={() => select({ kind: "device", id: device.id })}
                >
                  <strong>{device.name}</strong>
                  <small>
                    {plan.cabinets.find(
                      (cabinet) => cabinet.id === device.cabinet,
                    )?.name || roles[device.role]}
                  </small>
                  {device.cabinet && <small>{rackLabel(device)}</small>}
                  <span>
                    {plan.ports
                      .filter((port) => port.device === device.id)
                      .map((port) => port.name)
                      .join(" · ") || "Ports not documented"}
                  </span>
                </button>
              ))}
              {!devices.length && (
                <p className="help">No equipment documented</p>
              )}
            </section>
          );
        })}
      </div>
      <div className="cable-register">
        <div className="section-heading">
          <h2>Cable register</h2>
          <span>{plan.cables.length} segments</span>
          <label className="cable-sort">
            <span>Sort cables</span>
            <select
              value={cableSort}
              onChange={(event) =>
                setCableSort(event.target.value as CableSort)
              }
            >
              <option value="original">Original order</option>
              <option value="ascending">Label A–Z</option>
              <option value="descending">Label Z–A</option>
            </select>
          </label>
        </div>
        {cables.map((cable) => (
          <button
            className={`cable-register-row ${selection?.id === cable.id ? "active" : ""}`}
            key={cable.id}
            onClick={() => select({ kind: "cable", id: cable.id })}
          >
            <span className={`medium-dot ${cable.medium}`} />
            <span>
              <strong>{cable.label || "Unlabeled cable"}</strong>
              <small>
                {endpoint(plan, cable.a)} → {endpoint(plan, cable.b)}
              </small>
            </span>
            <span className="badge">{cable.medium}</span>
            <span className={`badge ${cable.draft ? "warning" : ""}`}>
              {cable.draft ? "Draft" : "Complete"}
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}
