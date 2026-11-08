import { PoEPathStatus } from "./PoEPathStatus";
import type { Plan, Port } from "./model";
import {
  linkDescription,
  portObservation,
  powerPaths,
  type PortView,
} from "./ports";
import { observedTime } from "./wifi";

export function PortStatus({
  port,
  plan,
  observations,
}: {
  port: Port;
  plan: Plan;
  observations: PortView;
}) {
  const link = portObservation(observations, port.id, "link");
  const power = portObservation(observations, port.id, "poe");
  const poe = power?.poe;
  const source = observations.sources.find(
    (item) => item.device === port.device && item.channel === "poe",
  );
  return (
    <section
      className="port-observations"
      aria-label={`Observed status for ${port.name}`}
    >
      <strong>{linkDescription(link)}</strong>
      {link && (
        <div className="help">
          Source: {link.source} · {link.name} · {observedTime(link.lastSeen)}
        </div>
      )}
      {source?.state === "unsupported" && (
        <p className="help">
          PoE-out not supported on this device (configured).
        </p>
      )}
      {source?.state === "disabled" && (
        <p className="help">
          PoE-out polling is disabled in the monitoring configuration.
        </p>
      )}
      {poe ? (
        <>
          <div>
            <b>PoE-out: {poe.mode}</b>
            {poe.vendorMode ? ` (${poe.vendorMode})` : ""} ·{" "}
            {poe.status === "powered-on" ? "Powered" : poe.status}
          </div>
          {power.state !== "fresh" && (
            <div className="help">
              Last known power reading ·{" "}
              {power.state === "missing" ? "not in latest inventory" : "stale"};
              current power unknown
            </div>
          )}
          <div>
            {poe.powerW == null
              ? "Power unknown"
              : `${poe.powerW.toLocaleString()} W`}{" "}
            ·{" "}
            {poe.voltageV == null
              ? "Voltage unknown"
              : `${poe.voltageV.toLocaleString()} V`}{" "}
            ·{" "}
            {poe.currentMA == null
              ? "Current unknown"
              : `${poe.currentMA.toLocaleString()} mA`}
          </div>
          {poe.voltageSelection && (
            <div>Observed voltage selection: {poe.voltageSelection}</div>
          )}
          <div className="help">
            Measured at supplying port · Source: {power.source} ·{" "}
            {observedTime(power.lastSeen)}
            <br />
            IEEE PoE standard is not inferred from mode or voltage.
          </div>
        </>
      ) : (
        <p className="help">
          {source?.state === "unavailable"
            ? "PoE monitoring menu unavailable on this source."
            : source?.state === "unsupported" || source?.state === "disabled"
              ? "No current PoE-out measurements are collected."
              : "PoE-out is not reported for this port; power status unknown."}
        </p>
      )}
      <PoEPathStatus
        plan={plan}
        paths={powerPaths(plan, observations).filter(
          (path) =>
            path.source.port !== port.id && path.ports.includes(port.id),
        )}
      />
    </section>
  );
}
