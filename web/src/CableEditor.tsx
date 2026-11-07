import { Field, SelectField, TextField } from "./Fields";
import {
  endpoint,
  trace,
  type Cable,
  type Endpoint,
  type Plan,
  type Selection,
} from "./model";
import type { ChangePlan } from "./DeviceEditor";

type Props = {
  cable: Cable;
  plan: Plan;
  edit: boolean;
  change: ChangePlan;
  select: (selection: Selection) => void;
};

export function CableEditor({
  cable: c,
  plan: p,
  edit,
  change,
  select,
}: Props) {
  const patch = (update: Partial<Cable>) =>
    change((plan) => ({
      ...plan,
      cables: plan.cables.map((cable) =>
        cable.id === c.id ? { ...cable, ...update } : cable,
      ),
    }));
  function renderEndpoint(side: "a" | "b", title: string) {
    const value: Endpoint = c[side];
    const other = c[side === "a" ? "b" : "a"];
    const ports = p.ports.filter((port) => port.device === value.device);
    const occupied = (id: string) =>
      p.cables.some(
        (cable) =>
          cable.id !== c.id && (cable.a.port === id || cable.b.port === id),
      );
    return (
      <div className="endpoint-box">
        <h3>{title}</h3>
        <SelectField
          label="Equipment"
          value={value.device}
          options={p.devices
            .filter((device) => device.id !== other.device)
            .map((device) => ({ value: device.id, label: device.name }))}
          onChange={(device) =>
            patch({ [side]: { device, port: "" }, draft: true })
          }
          disabled={!edit}
        />
        <SelectField
          label="Port / terminal"
          value={value.port}
          options={[
            { value: "", label: "Unknown — keep as draft" },
            ...ports
              .filter((port) => !occupied(port.id) || port.id === value.port)
              .map((port) => ({
                value: port.id,
                label: `${port.name}${port.label ? ` · ${port.label}` : ""}`,
              })),
          ]}
          onChange={(port) =>
            patch({
              [side]: { ...value, port },
              draft: !port || !other.port || c.draft,
            })
          }
          disabled={!edit}
        />
        {!ports.length && (
          <p className="help">Select this device and add its ports first.</p>
        )}
        <button
          className="link-button"
          onClick={() => select({ kind: "device", id: value.device })}
        >
          Open equipment →
        </button>
      </div>
    );
  }
  return (
    <>
      <div className="panel-heading">
        <span className="eyebrow">Physical connection</span>
        <h2>{c.label || "Cable"}</h2>
        <span className={`badge ${c.draft ? "warning" : ""}`}>
          {c.draft
            ? "Draft · routing incomplete"
            : "Port-to-port documentation"}
        </span>
      </div>
      <TextField
        label="Cable label"
        value={c.label}
        onChange={(label) => patch({ label })}
        disabled={!edit}
      />
      <SelectField
        label="Medium"
        value={c.medium}
        options={["copper", "fiber", "unknown"].map((value) => ({
          value,
          label: value,
        }))}
        onChange={(medium) => patch({ medium })}
        disabled={!edit}
      />
      <Field label="Documented length (metres, optional)">
        <input
          type="number"
          min="0"
          step="any"
          value={c.length ?? ""}
          disabled={!edit}
          onChange={(event) =>
            patch({
              length:
                event.target.value === "" ? null : Number(event.target.value),
            })
          }
        />
      </Field>
      {renderEndpoint("a", "Endpoint A")}
      {renderEndpoint("b", "Endpoint B")}
      <label className="checkbox">
        <input
          type="checkbox"
          checked={c.draft}
          disabled={!edit || !c.a.port || !c.b.port}
          onChange={(event) => patch({ draft: event.target.checked })}
        />
        Draft / intermediate routing unresolved
      </label>
      <p className="help">
        Document each patch lead and building cable separately. Resolve the
        entire segment before marking it complete.
      </p>
      <TextField
        label="Notes / routing details"
        value={c.notes}
        onChange={(notes) => patch({ notes })}
        disabled={!edit}
        multiline
      />
      <details className="trace" open>
        <summary>Trace physical path</summary>
        <ol>
          {trace(p, c).map((step, index) => (
            <li key={index}>{step}</li>
          ))}
        </ol>
      </details>
      <p className="help">
        Link status unknown · manual documentation.{" "}
        {c.medium === "fiber"
          ? "Fiber carries no PoE."
          : "Power delivery has not been observed."}
      </p>
      {edit && (
        <button
          className="danger"
          onClick={() => {
            if (
              !window.confirm(
                `Delete cable between ${endpoint(p, c.a)} and ${endpoint(p, c.b)}?`,
              )
            )
              return;
            change((plan) => ({
              ...plan,
              cables: plan.cables.filter((cable) => cable.id !== c.id),
            }));
            select(null);
          }}
        >
          Delete cable…
        </button>
      )}
    </>
  );
}
