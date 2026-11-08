import { InterfaceEditor } from "./InterfaceEditor";
import { useState } from "react";
import { Field, SelectField, TextField } from "./Fields";
import { RackMountFields } from "./Rack";
import {
  endpoint,
  location,
  passive,
  roles,
  uid,
  type DHCPObservation,
  type Device,
  type Plan,
  type Port,
  type Selection,
} from "./model";

export type ChangePlan = (update: (plan: Plan) => Plan) => void;

type Props = {
  device: Device;
  plan: Plan;
  observations: DHCPObservation[];
  observationError?: string;
  edit: boolean;
  floor: string;
  change: ChangePlan;
  select: (selection: Selection) => void;
};

export function DeviceEditor({
  device: d,
  plan: p,
  observations,
  observationError,
  edit,
  floor,
  change,
  select,
}: Props) {
  const [expanded, setExpanded] = useState("");
  const ports = p.ports.filter((port) => port.device === d.id);
  const cables = p.cables.filter(
    (cable) => cable.a.device === d.id || cable.b.device === d.id,
  );
  const patch = (update: Partial<Device>) =>
    change((plan) => ({
      ...plan,
      devices: plan.devices.map((device) =>
        device.id === d.id ? { ...device, ...update } : device,
      ),
    }));
  const updatePort = (id: string, update: Partial<Port>) =>
    change((plan) => ({
      ...plan,
      ports: plan.ports.map((port) =>
        port.id === id ? { ...port, ...update } : port,
      ),
    }));

  function addPort() {
    let index = ports.length + 1;
    while (ports.some((port) => port.name === `port${index}`)) index++;
    const port: Port = {
      id: uid(),
      device: d.id,
      name: `port${index}`,
      label: "",
      connector: "",
      position: "",
      side: "",
      poeMode: "unknown",
      poeStandard: "",
      voltage: "",
      notes: "",
    };
    change((plan) => ({ ...plan, ports: [...plan.ports, port] }));
    setExpanded(port.id);
  }

  function addPosition() {
    let index = 1;
    while (
      ports.some(
        (port) =>
          port.position === String(index) ||
          port.name === `${index} front` ||
          port.name === `${index} rear`,
      )
    )
      index++;
    const base = {
      device: d.id,
      label: "",
      connector: d.role === "termination" ? "" : "RJ45",
      position: String(index),
      poeMode: "unknown",
      poeStandard: "",
      voltage: "",
      notes: "",
    };
    const front: Port = {
      ...base,
      id: uid(),
      name: `${index} front`,
      side: "front",
    };
    const rear: Port = {
      ...base,
      id: uid(),
      name: `${index} rear`,
      side: "rear",
    };
    change((plan) => ({
      ...plan,
      ports: [...plan.ports, front, rear],
      passThroughs: [
        ...plan.passThroughs,
        { id: uid(), a: front.id, b: rear.id },
      ],
    }));
    setExpanded(front.id);
  }

  function deleteDevice() {
    const interfaces = (p.interfaces ?? []).filter(
      (item) => item.device === d.id,
    );
    const interfaceIDs = new Set(interfaces.map((item) => item.id));
    const affectedObservations = observations.filter(
      (item) => interfaceIDs.has(item.interface) || item.source === d.id,
    ).length;
    if (
      !window.confirm(
        `Delete ${d.name}? This removes its placement, ${ports.length} ports, their internal pass-throughs, and ${cables.length} cables, ${interfaces.length} interface identities, and ${affectedObservations} associated/source DHCP observations.`,
      )
    )
      return;
    const portIDs = new Set(ports.map((port) => port.id));
    change((plan) => ({
      ...plan,
      devices: plan.devices.filter((device) => device.id !== d.id),
      interfaces: (plan.interfaces ?? []).filter(
        (item) => item.device !== d.id,
      ),
      ports: plan.ports.filter((port) => port.device !== d.id),
      cables: plan.cables.filter(
        (cable) => cable.a.device !== d.id && cable.b.device !== d.id,
      ),
      passThroughs: plan.passThroughs.filter(
        (pair) => !portIDs.has(pair.a) && !portIDs.has(pair.b),
      ),
    }));
    select(null);
  }

  const placement = location(p, d);
  return (
    <>
      <div className="panel-heading">
        <span className="eyebrow">Equipment</span>
        <h2>{d.name}</h2>
        <span className="badge">{roles[d.role]}</span>
      </div>
      <TextField
        label="Friendly name"
        value={d.name}
        onChange={(name) => patch({ name })}
        disabled={!edit}
      />
      <TextField
        label="Short name (optional)"
        value={d.shortName ?? ""}
        onChange={(shortName) => patch({ shortName })}
        disabled={!edit}
      />
      <SelectField
        label="Role"
        value={d.role}
        options={Object.entries(roles).map(([value, label]) => ({
          value,
          label,
        }))}
        onChange={(role) => patch({ role })}
        disabled={!edit}
      />
      <TextField
        label="Model / operating system"
        value={d.model}
        onChange={(model) => patch({ model })}
        disabled={!edit}
      />
      <TextField
        label="Management address (documentation only)"
        value={d.address}
        onChange={(address) => patch({ address })}
        disabled={!edit}
      />
      <TextField
        label="Notes"
        value={d.notes}
        onChange={(notes) => patch({ notes })}
        disabled={!edit}
        multiline
      />
      <InterfaceEditor
        device={d}
        plan={p}
        observations={observations}
        observationError={observationError}
        edit={edit}
        change={change}
      />
      <h3>Placement</h3>
      <SelectField
        label="Placement mode"
        value={d.mode}
        options={[
          { value: "fixed", label: "Fixed equipment" },
          { value: "mobile", label: "Mobile / location unknown" },
        ]}
        onChange={(mode) =>
          patch({ mode, floor: "", cabinet: "", rackUnit: null })
        }
        disabled={!edit}
      />
      {d.mode === "fixed" && (
        <>
          <SelectField
            label="Cabinet"
            value={d.cabinet}
            options={[
              { value: "", label: "Outside a cabinet" },
              ...p.cabinets.map((cabinet) => ({
                value: cabinet.id,
                label: cabinet.name,
              })),
            ]}
            onChange={(cabinet) =>
              patch({ cabinet, floor: "", rackUnit: null })
            }
            disabled={!edit}
          />
          {d.cabinet && (
            <RackMountFields device={d} disabled={!edit} patch={patch} />
          )}
          {!d.cabinet && (
            <SelectField
              label="Floor"
              value={d.floor}
              options={[
                { value: "", label: "Unplaced" },
                ...p.floors.map((floor) => ({
                  value: floor.id,
                  label: floor.name,
                })),
              ]}
              onChange={(floor) => patch({ floor, x: 0.5, y: 0.5 })}
              disabled={!edit}
            />
          )}
          {d.cabinet && (
            <p className="help">
              Inherits the cabinet’s placement.{" "}
              {placement ? "" : "The cabinet has not been positioned yet."}
            </p>
          )}
          {edit && !d.cabinet && (
            <button onClick={() => patch({ floor, x: 0.5, y: 0.5 })}>
              Place on current floor
            </button>
          )}
          {placement && (
            <p className="help">
              {p.floors.find((floor) => floor.id === placement.floor)?.name} ·{" "}
              {edit
                ? "Drag the marker to position it."
                : "Switch to Edit to reposition."}
            </p>
          )}
        </>
      )}
      <div className="section-heading">
        <h3>
          Ports & terminals <small>{ports.length}</small>
        </h3>
        {edit && (
          <button
            className="small"
            onClick={passive(d) ? addPosition : addPort}
          >
            + {passive(d) ? "Position" : "Port"}
          </button>
        )}
      </div>
      {passive(d) && (
        <p className="help">
          Each position has separate front/rear terminals and an explicit
          internal pass-through.
        </p>
      )}
      {!ports.length && <p className="empty-small">No ports documented.</p>}
      {ports.map((port) => {
        const attached = p.cables.find(
          (cable) => cable.a.port === port.id || cable.b.port === port.id,
        );
        const pair = p.passThroughs.find(
          (pair) => pair.a === port.id || pair.b === port.id,
        );
        const other = p.ports.find(
          (other) => other.id === (pair?.a === port.id ? pair?.b : pair?.a),
        );
        const linkedInterfaces = (p.interfaces ?? []).filter(
          (item) => item.port === port.id,
        );
        return (
          <div className="port-card" key={port.id}>
            <button
              className="port-summary"
              onClick={() => setExpanded(expanded === port.id ? "" : port.id)}
            >
              <strong>{port.name}</strong>
              <span>
                {port.label || port.connector || "Details"}{" "}
                {expanded === port.id ? "−" : "+"}
              </span>
            </button>
            {expanded === port.id && (
              <div className="port-details">
                <TextField
                  label="Port name"
                  value={port.name}
                  onChange={(name) => updatePort(port.id, { name })}
                  disabled={!edit}
                />
                <Field label="Cable label port number (optional)">
                  <input
                    type="number"
                    min="1"
                    max="9999"
                    step="1"
                    value={port.labelNumber ?? ""}
                    disabled={!edit}
                    placeholder="Auto for etherN / PortN / sfpN"
                    onChange={(event) =>
                      updatePort(port.id, {
                        labelNumber:
                          event.target.value === ""
                            ? null
                            : Number(event.target.value),
                      })
                    }
                  />
                </Field>
                <p className="help">
                  Copper ports use P01; SFP ports use Psfp01. Names such as
                  sfp1, SFP+1, and sfp-sfpplus1 are recognized automatically.
                  Set a number for custom names; an SFP connector keeps the Psfp
                  prefix. Passive terminals add F/R for front/rear.
                </p>
                <TextField
                  label="Label"
                  value={port.label}
                  onChange={(label) => updatePort(port.id, { label })}
                  disabled={!edit}
                />
                <TextField
                  label="Connector / cable category or fiber type"
                  value={port.connector}
                  onChange={(connector) => updatePort(port.id, { connector })}
                  disabled={!edit}
                />
                {passive(d) && (
                  <>
                    <Field label="Position / side">
                      <input
                        value={`${port.position} / ${port.side || "unknown"}`}
                        disabled
                      />
                    </Field>
                    <p className="help">
                      {other
                        ? `Internal pass-through ↔ ${other.name}`
                        : "Internal mapping unknown"}
                    </p>
                  </>
                )}
                {!passive(d) && (
                  <>
                    <SelectField
                      label="Documented PoE-out mode"
                      value={port.poeMode}
                      options={[
                        "unknown",
                        "auto",
                        "forced",
                        "off",
                        "unsupported",
                      ].map((value) => ({ value, label: value }))}
                      onChange={(poeMode) => updatePort(port.id, { poeMode })}
                      disabled={!edit}
                    />
                    <TextField
                      label="PoE standard / type (if known)"
                      value={port.poeStandard}
                      onChange={(poeStandard) =>
                        updatePort(port.id, { poeStandard })
                      }
                      disabled={!edit}
                    />
                    <TextField
                      label="Voltage selection / vendor mode"
                      value={port.voltage}
                      onChange={(voltage) => updatePort(port.id, { voltage })}
                      disabled={!edit}
                    />
                    <p className="help">
                      Current power status unknown. Configured mode does not
                      establish power delivery.
                    </p>
                  </>
                )}
                <TextField
                  label="Port notes"
                  value={port.notes}
                  onChange={(notes) => updatePort(port.id, { notes })}
                  disabled={!edit}
                  multiline
                />
                {linkedInterfaces.length > 0 && (
                  <p className="help">
                    Network interfaces:{" "}
                    {linkedInterfaces.map((item) => item.name).join(", ")}
                  </p>
                )}
                {attached ? (
                  <button
                    className="link-button"
                    onClick={() => select({ kind: "cable", id: attached.id })}
                  >
                    Cable: {attached.label || attached.medium} →
                  </button>
                ) : (
                  <p className="help">No external cable documented</p>
                )}
                {edit && (
                  <button
                    className="danger small"
                    disabled={!!attached}
                    onClick={() => {
                      if (
                        !window.confirm(
                          `Delete ${port.name}${pair ? " and its internal mapping" : ""}?${linkedInterfaces.length ? ` This unlinks ${linkedInterfaces.length} network interface(s); their identities and DHCP evidence will be retained.` : ""}`,
                        )
                      )
                        return;
                      change((plan) => ({
                        ...plan,
                        ports: plan.ports.filter((item) => item.id !== port.id),
                        interfaces: (plan.interfaces ?? []).map((item) =>
                          item.port === port.id ? { ...item, port: "" } : item,
                        ),
                        passThroughs: plan.passThroughs.filter(
                          (item) => item.a !== port.id && item.b !== port.id,
                        ),
                      }));
                    }}
                  >
                    Remove terminal
                  </button>
                )}
              </div>
            )}
          </div>
        );
      })}
      <h3>Documented cables</h3>
      {cables.map((cable) => (
        <button
          key={cable.id}
          className="connection-row"
          onClick={() => select({ kind: "cable", id: cable.id })}
        >
          <span>
            {endpoint(p, cable.a.device === d.id ? cable.b : cable.a)}
          </span>
          <small>
            {cable.medium} · {cable.draft ? "draft" : "complete"}
          </small>
        </button>
      ))}
      {!cables.length && <p className="empty-small">No cables documented.</p>}
      {edit && (
        <button className="danger" onClick={deleteDevice}>
          Delete equipment…
        </button>
      )}
    </>
  );
}
