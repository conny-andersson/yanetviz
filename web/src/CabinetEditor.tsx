import { SelectField, TextField } from "./Fields";
import { type Cabinet, type Plan, type Selection } from "./model";
import type { ChangePlan } from "./DeviceEditor";

export function CabinetEditor({
  cabinet: c,
  plan: p,
  edit,
  change,
  select,
}: {
  cabinet: Cabinet;
  plan: Plan;
  edit: boolean;
  change: ChangePlan;
  select: (selection: Selection) => void;
}) {
  const members = p.devices.filter((device) => device.cabinet === c.id);
  const patch = (update: Partial<Cabinet>) =>
    change((plan) => ({
      ...plan,
      cabinets: plan.cabinets.map((cabinet) =>
        cabinet.id === c.id ? { ...cabinet, ...update } : cabinet,
      ),
    }));
  return (
    <>
      <div className="panel-heading">
        <span className="eyebrow">Equipment group</span>
        <h2>{c.name}</h2>
        <span className="badge">Cabinet</span>
      </div>
      <TextField
        label="Name"
        value={c.name}
        onChange={(name) => patch({ name })}
        disabled={!edit}
      />
      <SelectField
        label="Floor"
        value={c.floor}
        options={p.floors.map((floor) => ({
          value: floor.id,
          label: floor.name,
        }))}
        onChange={(floor) => patch({ floor, placed: false })}
        disabled={!edit}
      />
      <p className="help">
        {c.placed
          ? "All cabinet devices share this marker. Drag it in Edit mode to reposition."
          : "Floor is known; exact position is undocumented."}
      </p>
      {edit && (
        <button onClick={() => patch({ placed: !c.placed, x: 0.5, y: 0.5 })}>
          {c.placed
            ? "Remove floor-plan position"
            : "Place cabinet on floor plan"}
        </button>
      )}
      <h3>
        Equipment <small>{members.length}</small>
      </h3>
      {members.map((device) => (
        <button
          className="connection-row"
          key={device.id}
          onClick={() => select({ kind: "device", id: device.id })}
        >
          {device.name}
          <span>→</span>
        </button>
      ))}
      <p className="help">
        Assign equipment using its Cabinet field. Cabinet grouping is
        organizational and adds no network hop.
      </p>
      {edit && (
        <button
          className="danger"
          onClick={() => {
            if (
              !window.confirm(
                `Delete ${c.name}? ${members.length} devices will become unplaced; their ports and cables will be retained.`,
              )
            )
              return;
            change((plan) => ({
              ...plan,
              cabinets: plan.cabinets.filter((cabinet) => cabinet.id !== c.id),
              devices: plan.devices.map((device) =>
                device.cabinet === c.id
                  ? { ...device, cabinet: "", floor: "" }
                  : device,
              ),
            }));
            select(null);
          }}
        >
          Delete cabinet…
        </button>
      )}
    </>
  );
}
