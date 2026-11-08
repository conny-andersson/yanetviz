import { DeviceInventory, observedInventory } from "./DeviceInventory";
import type { Plan, Selection } from "./model";
import {
  linkDescription,
  portCapabilityLabel,
  type PortReading,
  type PortView,
} from "./ports";
import { observedTime } from "./wifi";

type Props = {
  plan: Plan;
  observations: PortView;
  error: string;
  select: (selection: Selection) => void;
};
export function PortOverview({ plan, observations, error, select }: Props) {
  const name = (id: string) =>
    plan.devices.find((device) => device.id === id)?.name ?? id;
  const sources = [
    ...new Set(observations.sources.map((source) => source.source)),
  ];
  const rows = new Map<string, { link?: PortReading; poe?: PortReading }>();
  for (const reading of observations.readings) {
    const key = `${reading.source}/${reading.defaultName || reading.name}`;
    const row = rows.get(key) ?? {};
    if (reading.channel === "link") row.link = reading;
    else row.poe = reading;
    rows.set(key, row);
  }
  return (
    <div className="port-overview">
      <h2>Link and PoE observations</h2>
      {error && <p role="alert">{error}</p>}
      {!sources.length && (
        <p>
          No port monitoring sources configured. See README.md for the
          monitoring configuration.
        </p>
      )}
      <div className="port-source-grid">
        {sources.map((source) => (
          <section className="port-source" key={source}>
            <h3>
              {name(
                observations.sources.find((item) => item.source === source)!
                  .device,
              )}
            </h3>
            <DeviceInventory
              inventory={observedInventory(
                observations,
                observations.sources.find((item) => item.source === source)!
                  .device,
              )}
            />
            {observations.sources
              .filter((item) => item.source === source)
              .map((item) => (
                <div key={item.channel}>
                  <b>
                    {item.channel === "poe" ? "PoE-out" : "Ethernet"}:{" "}
                    {portCapabilityLabel(item.state)}
                  </b>
                  {item.error && <p role="alert">{item.error}</p>}
                  {item.state === "unsupported" || item.state === "disabled" ? (
                    <p className="help">
                      Configured override; no PoE requests sent.
                    </p>
                  ) : (
                    <p className="help">
                      Last success: {observedTime(item.lastSuccess)}
                      <br />
                      Last attempt: {observedTime(item.lastAttempt)}
                    </p>
                  )}
                </div>
              ))}
          </section>
        ))}
      </div>
      <p className="help">
        Link-up does not verify a cable’s far endpoint. Measurements never
        change documented port names, cabling or PoE settings. Missing PoE
        metrics remain unknown. SwOS and unconfigured equipment stay manual.
      </p>
      <table>
        <thead>
          <tr>
            <th>Device / port</th>
            <th>Observed link</th>
            <th>PoE-out</th>
            <th>Last observation</th>
          </tr>
        </thead>
        <tbody>
          {[...rows.entries()].map(([key, row]) => {
            const reading = row.link ?? row.poe!;
            const port = plan.ports.find((item) => item.id === reading.port);
            return (
              <tr key={key}>
                <td>
                  <button
                    className="link-button"
                    onClick={() =>
                      select({ kind: "device", id: reading.device })
                    }
                  >
                    {name(reading.device)}
                  </button>
                  <br />
                  {port?.name ?? "Unmapped port"} · {reading.name}
                  <br />
                  <small>{reading.defaultName}</small>
                </td>
                <td>{linkDescription(row.link)}</td>
                <td>
                  {row.poe?.poe ? (
                    <>
                      {row.poe.poe.mode} ({row.poe.poe.vendorMode}) ·{" "}
                      {row.poe.poe.status}
                      <br />
                      {row.poe.poe.powerW == null
                        ? "Power unknown"
                        : `${row.poe.poe.powerW} W`}
                      {row.poe.state !== "fresh"
                        ? " · stale/current unknown"
                        : ""}
                    </>
                  ) : (
                    portCapabilityLabel(
                      observations.sources.find(
                        (item) =>
                          item.source === reading.source &&
                          item.channel === "poe" &&
                          ["unsupported", "disabled", "unavailable"].includes(
                            item.state,
                          ),
                      )?.state ?? "Not reported / unknown",
                    )
                  )}
                </td>
                <td>
                  {observedTime(reading.lastSeen)}
                  <br />
                  <small>{reading.source}</small>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
