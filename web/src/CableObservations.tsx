import type { Cable, Plan } from "./model";
import { PortStatus } from "./PortStatus";
import { powerPaths, powerStatus, type PortView } from "./ports";

export function CableObservations({
  plan,
  cable,
  observations,
}: {
  plan: Plan;
  cable: Cable;
  observations: PortView;
}) {
  const flows = powerPaths(plan, observations)
    .flatMap((path) => path.segments)
    .filter((flow) => flow.cable === cable.id);
  return (
    <section className="cable-observations">
      <h3>Observed endpoint status</h3>
      {[cable.a, cable.b].map((endpoint, index) => {
        const port = plan.ports.find((item) => item.id === endpoint.port);
        return (
          <div key={index}>
            <h4>
              {index === 0 ? "A" : "B"} ·{" "}
              {plan.devices.find((item) => item.id === endpoint.device)?.name}
            </h4>
            {port ? (
              <PortStatus plan={plan} port={port} observations={observations} />
            ) : (
              <p className="help">
                Endpoint port unresolved; observed link and power unknown.
              </p>
            )}
          </div>
        );
      })}
      <p className="help">
        Endpoint link status does not prove the documented far-end connection.
      </p>
      {cable.medium === "fiber" ? (
        <p className="help">Fiber carries no PoE.</p>
      ) : flows.length > 0 ? (
        <>
          <h4>Upstream PoE status</h4>
          <ul>
            {flows.map((flow) => (
              <li key={`${flow.source.source}/${flow.source.port}`}>
                ⚡{" "}
                {
                  plan.devices.find((item) => item.id === flow.source.device)
                    ?.name
                }{" "}
                /{" "}
                {plan.ports.find((item) => item.id === flow.source.port)?.name}{" "}
                · {powerStatus(flow.source)} ·{" "}
                {flow.source.poe?.powerW == null
                  ? "Power unknown"
                  : `${flow.source.poe.powerW} W measured at source`}
                {flow.passive ? " · passive compatibility unverified" : ""}
              </li>
            ))}
          </ul>
          <p className="help">
            Direction follows the documented copper path, stopping at active
            devices or unresolved terminals. Voltage and watts at downstream
            equipment are not inferred.
          </p>
        </>
      ) : (
        <p className="help">
          No supplying-port observation for this documented copper path.
        </p>
      )}
    </section>
  );
}
