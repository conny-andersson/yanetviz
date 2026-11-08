import type { Plan, Selection } from "./model";
import {
  linkRate,
  observedTime,
  wifiState,
  type WiFiClient,
  type WiFiView,
} from "./wifi";

type Props = {
  plan: Plan;
  wifi: WiFiView;
  device: string;
  select: (selection: Selection) => void;
};

export function WiFiDetails({ plan, wifi, device, select }: Props) {
  const own = wifi.clients.filter((client) => client.device === device);
  const served = wifi.clients.filter(
    (client) => client.ap === device && client.present,
  );
  if (
    !own.length &&
    !served.length &&
    !wifi.sources.some((source) =>
      source.radios?.some((radio) => radio.ap === device),
    )
  )
    return null;
  const name = (id?: string) =>
    plan.devices.find((item) => item.id === id)?.name;
  const show = (client: WiFiClient, serving: boolean) => (
    <li key={`${client.source}/${client.mac}/${client.interfaceName}`}>
      <strong>{wifiState(client.state)}</strong>
      <div>
        {serving ? "Client: " : "AP: "}
        {(serving ? client.device : client.ap) ? (
          <button
            className="link-button"
            onClick={() =>
              select({
                kind: "device",
                id: (serving ? client.device : client.ap)!,
              })
            }
          >
            {name(serving ? client.device : client.ap)}
          </button>
        ) : serving ? (
          "Unmatched client"
        ) : (
          "Unresolved AP mapping"
        )}
      </div>
      <div>
        {client.mac} · {client.ssid || "SSID unknown"} ·{" "}
        {client.band || "Band unknown"}
      </div>
      <div>
        Signal: {client.signal == null ? "Unknown" : `${client.signal} dBm`}
      </div>
      <div>
        Access authorization:{" "}
        {client.authorized == null
          ? "Unknown"
          : client.authorized
            ? "Authorized"
            : "Not authorized"}
      </div>
      <div>
        AP receive / transmit link rates: {linkRate(client.rxRate)} /{" "}
        {linkRate(client.txRate)}
      </div>
      <div className="help">
        Source: {client.source} · {client.interfaceName}
        <br />
        Last seen: {observedTime(client.lastSeen)}
      </div>
    </li>
  );
  return (
    <section className="wifi-details">
      <h3>Wi-Fi observations</h3>
      {own.length > 0 && <ul>{own.map((client) => show(client, false))}</ul>}
      {(served.length > 0 || !own.length) && (
        <>
          <h4>Clients on this AP</h4>
          {served.length ? (
            <ul>{served.map((client) => show(client, true))}</ul>
          ) : (
            <p className="help">
              No retained associated clients. Check source freshness before
              interpreting absence.
            </p>
          )}
        </>
      )}
      <p className="help">
        Radio link rates are not measured internet speed. Association never
        changes a device’s documented location.
      </p>
    </section>
  );
}
