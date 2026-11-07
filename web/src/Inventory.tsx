import { useState } from "react";
import { location, roles, type Plan, type Selection } from "./model";

export function Inventory({
  plan,
  selection,
  select,
  navigate,
}: {
  plan: Plan;
  selection: Selection;
  select: (selection: Selection) => void;
  navigate: (floor: string) => void;
}) {
  const [query, setQuery] = useState("");
  const matches = (value: string) =>
    value.toLowerCase().includes(query.toLowerCase());
  const deviceRow = (id: string) => {
    const device = plan.devices.find((device) => device.id === id)!;
    if (
      !matches(
        `${device.name} ${device.model} ${device.address} ${roles[device.role]}`,
      )
    )
      return null;
    return (
      <button
        key={id}
        className={`device-row ${selection?.id === id ? "active" : ""}`}
        onClick={() => {
          select({ kind: "device", id });
          const position = location(plan, device);
          if (position) navigate(position.floor);
        }}
      >
        <span className="device-symbol">
          {device.role === "access_point" ? "◉" : "⇄"}
        </span>
        <span>
          <strong>{device.name}</strong>
          <small>{roles[device.role]}</small>
        </span>
      </button>
    );
  };

  return (
    <aside className="inventory">
      <div className="inventory-header">
        <span className="eyebrow">Workspace</span>
        <h2>
          Equipment <span>{plan.devices.length}</span>
        </h2>
        <input
          aria-label="Search equipment"
          placeholder="Search name, model, address…"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
      </div>
      <div className="inventory-scroll">
        {plan.cabinets.map((cabinet) => (
          <div className="inventory-group" key={cabinet.id}>
            <button
              className={`group-title ${selection?.id === cabinet.id ? "active" : ""}`}
              onClick={() => {
                select({ kind: "cabinet", id: cabinet.id });
                navigate(cabinet.floor);
              }}
            >
              <span>▤ {cabinet.name}</span>
              <small>{cabinet.placed ? "" : "Unplaced"}</small>
            </button>
            {plan.devices
              .filter((device) => device.cabinet === cabinet.id)
              .map((device) => deviceRow(device.id))}
          </div>
        ))}
        {plan.floors.map((floor) => {
          const devices = plan.devices.filter(
            (device) => !device.cabinet && device.floor === floor.id,
          );
          return (
            devices.length > 0 && (
              <div className="inventory-group" key={floor.id}>
                <div className="group-title">{floor.name}</div>
                {devices.map((device) => deviceRow(device.id))}
              </div>
            )
          );
        })}
        <div className="inventory-group">
          <div className="group-title">Unplaced</div>
          {plan.devices
            .filter(
              (device) =>
                !device.cabinet && !device.floor && device.mode === "fixed",
            )
            .map((device) => deviceRow(device.id))}
        </div>
        <div className="inventory-group">
          <div className="group-title">Mobile / location unknown</div>
          {plan.devices
            .filter((device) => device.mode === "mobile")
            .map((device) => deviceRow(device.id))}
        </div>
      </div>
      <div className="inventory-footer">
        <span className="status-dot" />
        Local plan · no live collection
      </div>
    </aside>
  );
}
