import { Field, SelectField, TextField } from "./Fields";
import type { ChangePlan } from "./DeviceEditor";
import {
  uid,
  type Device,
  type DHCPObservation,
  type NetworkInterface,
  type Plan,
} from "./model";

type Props = {
  device: Device;
  plan: Plan;
  observations: DHCPObservation[];
  observationError?: string;
  edit: boolean;
  change: ChangePlan;
};

export function InterfaceEditor({
  device,
  plan,
  observations,
  observationError,
  edit,
  change,
}: Props) {
  const interfaces = (plan.interfaces ?? []).filter(
    (item) => item.device === device.id,
  );
  const patch = (id: string, update: Partial<NetworkInterface>) =>
    change((plan) => ({
      ...plan,
      interfaces: (plan.interfaces ?? []).map((item) =>
        item.id === id ? { ...item, ...update } : item,
      ),
    }));
  function add() {
    const item: NetworkInterface = {
      id: uid(),
      device: device.id,
      name: `Interface ${interfaces.length + 1}`,
      domain: "home",
      macAddresses: [],
    };
    change((plan) => ({
      ...plan,
      interfaces: [...(plan.interfaces ?? []), item],
    }));
  }
  return (
    <section className="network-identities">
      <div className="section-heading">
        <h3>
          Network interfaces <small>{interfaces.length}</small>
        </h3>
        {edit && (
          <button className="small" onClick={add}>
            + Interface
          </button>
        )}
      </div>
      {observationError && <p className="error-text">{observationError}</p>}
      {!interfaces.length && (
        <p className="empty-small">No interface identities documented.</p>
      )}
      {interfaces.map((item) => {
        const records = observations.filter(
          (record) => record.interface === item.id,
        );
        return (
          <details
            className="interface-card"
            key={item.id}
            open={device.role === "client"}
          >
            <summary>
              {item.name}
              <small>
                {item.domain} · {item.macAddresses.length} MAC
                {item.macAddresses.length === 1 ? "" : "s"}
              </small>
            </summary>
            <div className="interface-details">
              <TextField
                label="Interface name"
                value={item.name}
                disabled={!edit}
                onChange={(name) => patch(item.id, { name })}
              />
              <SelectField
                label="Physical port (optional)"
                value={item.port ?? ""}
                disabled={!edit}
                onChange={(port) => patch(item.id, { port })}
                options={[
                  { value: "", label: "No port / wireless / unknown" },
                  ...plan.ports
                    .filter((port) => port.device === device.id)
                    .map((port) => ({
                      value: port.id,
                      label: `${port.name}${port.label ? ` · ${port.label}` : ""}`,
                    })),
                ]}
              />
              <p className="help">
                Optional link to a connector on this device. Multiple virtual
                interfaces may share a port. Leave blank for wireless or unknown
                connections.
              </p>
              <TextField
                label="Network identity domain"
                value={item.domain}
                disabled={!edit}
                onChange={(domain) => patch(item.id, { domain })}
              />
              <Field label="MAC aliases (one per line)">
                <textarea
                  rows={3}
                  value={item.macAddresses.join("\n")}
                  disabled={!edit}
                  onChange={(event) =>
                    patch(item.id, {
                      macAddresses: event.target.value.split("\n"),
                    })
                  }
                />
              </Field>
              <p className="help">
                Use the same domain for sources observing the same LAN. Add
                aliases only when you know they belong to this interface;
                duplicate domain/MAC identities are rejected.
              </p>
              <h4>
                DHCP evidence <small>{records.length}</small>
              </h4>
              <p className="help">
                Saved collection snapshots, not live presence or AP association.
                Client IDs are supporting evidence; they do not merge devices
                automatically.
              </p>
              {records.map((record, index) => (
                <div className="dhcp-evidence" key={index}>
                  <span className="badge">Snapshot · {record.status}</span>
                  <dl>
                    <dt>Source / server</dt>
                    <dd>
                      {plan.devices.find(
                        (source) => source.id === record.source,
                      )?.name ?? record.source}{" "}
                      / {record.server || "Unknown"}
                    </dd>
                    <dt>MAC</dt>
                    <dd>{record.macAddress}</dd>
                    <dt>Client ID</dt>
                    <dd>
                      <code>{record.clientId || "Not supplied"}</code>
                      {record.clientIdField && (
                        <small>{record.clientIdField}</small>
                      )}
                    </dd>
                    <dt>Observed IP</dt>
                    <dd>{record.address || "Unknown"}</dd>
                    <dt>Observed hostname</dt>
                    <dd>{record.hostname || "Unknown"}</dd>
                    <dt>First observed</dt>
                    <dd>
                      <time dateTime={record.firstSeen}>
                        {new Date(record.firstSeen).toLocaleString()}
                      </time>
                    </dd>
                    <dt>Last observed</dt>
                    <dd>
                      <time dateTime={record.lastSeen}>
                        {new Date(record.lastSeen).toLocaleString()}
                      </time>
                    </dd>
                  </dl>
                </div>
              ))}
              {!records.length && (
                <p className="empty-small">
                  No DHCP observations collected for this interface.
                </p>
              )}
              {edit && (
                <button
                  className="danger small"
                  onClick={() => {
                    if (
                      !window.confirm(
                        `Remove ${item.name} and its ${records.length} saved DHCP observations? The device, ports and cables will remain.`,
                      )
                    )
                      return;
                    change((plan) => ({
                      ...plan,
                      interfaces: (plan.interfaces ?? []).filter(
                        (candidate) => candidate.id !== item.id,
                      ),
                    }));
                  }}
                >
                  Remove interface…
                </button>
              )}
            </div>
          </details>
        );
      })}
    </section>
  );
}
