import type { Plan } from "./model";
import { powerStatus, type PowerPath } from "./ports";
import { observedTime } from "./wifi";

export function PoEPathStatus({
  plan,
  paths,
}: {
  plan: Plan;
  paths: PowerPath[];
}) {
  if (!paths.length) return null;
  return (
    <section className="poe-path-status" aria-label="Traced upstream PoE">
      <h4>PoE along documented copper path</h4>
      {paths.length > 1 && (
        <p className="help">
          Multiple upstream reports; a single supplying path cannot be
          confirmed.
        </p>
      )}
      {paths.map((path) => {
        const source = path.source;
        const poe = source.poe!;
        const device = plan.devices.find((item) => item.id === source.device);
        const port = plan.ports.find((item) => item.id === source.port);
        return (
          <div key={`${source.source}/${source.port}`}>
            <strong>
              PoE via {device?.name ?? "Unknown source"} /{" "}
              {port?.name ?? source.name}: {powerStatus(source)}
            </strong>
            <div>
              Source mode: {poe.mode}
              {poe.vendorMode ? ` (${poe.vendorMode})` : ""}
            </div>
            <div>
              At source:{" "}
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
            <div className="help">
              Source: {source.source} · {observedTime(source.lastSeen)}
            </div>
          </div>
        );
      })}
      <p className="help">
        Status is traced from the supplying port through documented cables and
        passive pass-throughs. These are source measurements, not measurements
        at this port or device. Passive compatibility remains unverified.
      </p>
    </section>
  );
}
