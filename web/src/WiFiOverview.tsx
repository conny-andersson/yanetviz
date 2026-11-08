import type { Plan, Selection } from "./model";
import { observedTime, wifiState, type WiFiView } from "./wifi";

type Props = {
  plan: Plan;
  wifi: WiFiView;
  error: string;
  select: (selection: Selection) => void;
};
export function WiFiOverview({ plan, wifi, error, select }: Props) {
  const name = (id?: string) =>
    plan.devices.find((device) => device.id === id)?.name;
  return (
    <div className="wifi-overview">
      <h2>Wi-Fi collection</h2>
      {error && <p role="alert">{error}</p>}
      {!wifi.sources.length && (
        <p>
          No CAPsMAN collection source configured. Start the application with a
          Wi-Fi configuration file; see README.md.
        </p>
      )}
      {wifi.sources.map((source) => (
        <section className="wifi-source" key={source.source}>
          <h3>
            {name(source.device) ?? source.source} · {wifiState(source.state)}
          </h3>
          {source.error && <p role="alert">{source.error}</p>}
          <p>
            Last successful snapshot: {observedTime(source.lastSuccess)}
            <br />
            Last attempt: {observedTime(source.lastAttempt)}
          </p>
          <p className="help">
            {source.menu || "Waiting for capability discovery"}
          </p>
          <details>
            <summary>Radio mappings ({source.radios?.length ?? 0})</summary>
            <ul>
              {source.radios?.map((radio) => (
                <li key={radio.mac}>
                  {radio.interfaceName} · {radio.mac} →{" "}
                  {name(radio.ap) ?? "Unresolved physical AP"}
                </li>
              ))}
            </ul>
          </details>
        </section>
      ))}
      <h3>Observed clients</h3>
      <p className="help">
        Clients match only by network domain and documented MAC aliases. To
        resolve an unmatched client, add its MAC to an equipment network
        interface. Names and IP addresses never establish ownership.
      </p>
      <table>
        <thead>
          <tr>
            <th>Client / MAC</th>
            <th>State</th>
            <th>Serving AP / radio</th>
            <th>SSID / band</th>
            <th>Last seen</th>
          </tr>
        </thead>
        <tbody>
          {wifi.clients.map((client) => (
            <tr key={`${client.source}/${client.mac}/${client.interfaceName}`}>
              <td>
                {client.device ? (
                  <button
                    className="link-button"
                    onClick={() =>
                      select({ kind: "device", id: client.device! })
                    }
                  >
                    {name(client.device)}
                  </button>
                ) : (
                  <strong>Unmatched client</strong>
                )}
                <br />
                {client.mac}
                <br />
                <small>
                  {client.domain} · {client.source}
                </small>
              </td>
              <td>{wifiState(client.state)}</td>
              <td>
                {name(client.ap) ?? "Unresolved AP"}
                <br />
                <small>{client.interfaceName}</small>
              </td>
              <td>
                {client.ssid || "Unknown"}
                <br />
                {client.band || "Unknown"}
              </td>
              <td>{observedTime(client.lastSeen)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {!wifi.clients.length && <p>No retained Wi-Fi registrations.</p>}
    </div>
  );
}
