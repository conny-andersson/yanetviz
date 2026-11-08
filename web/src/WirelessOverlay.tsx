import type { Plan, Selection } from "./model";
import { wifiState, type WiFiView } from "./wifi";

type Position = { floor: string; x: number; y: number } | null;
type Props = {
  plan: Plan;
  wifi: WiFiView;
  floor: string;
  height: number;
  selection: Selection;
  point: (id: string) => Position;
  navigate: (floor: string) => void;
};
export function WirelessOverlay({
  plan,
  wifi,
  floor,
  height,
  selection,
  point,
  navigate,
}: Props) {
  if (selection?.kind !== "device") return null;
  return (
    <g className="wireless-overlay">
      {wifi.clients.map((client, index) => {
        if (
          !client.device ||
          !client.ap ||
          !client.present ||
          client.state === "conflict" ||
          (client.device !== selection.id && client.ap !== selection.id)
        )
          return null;
        const a = point(client.device),
          b = point(client.ap);
        // Mobile/unplaced clients have no inferred coordinates, even when their AP is known.
        if (!a || !b || (a.floor !== floor && b.floor !== floor)) return null;
        const local = a.floor === floor ? a : b;
        const remote = a.floor === floor ? b : a;
        const sameFloor = local.floor === remote.floor;
        const end = sameFloor
          ? remote
          : { x: 0.08 + (index % 3) * 0.04, y: 0.1 };
        const label = `${wifiState(client.state)} · ${client.ssid || "Wi-Fi"}`;
        return (
          <g
            key={`${client.source}/${client.mac}/${client.interfaceName}`}
            className={`wireless-link ${client.state === "stale" ? "stale" : ""}`}
            aria-label={label}
          >
            <path
              d={`M${local.x * 1000},${local.y * height} L${end.x * 1000},${end.y * height}`}
            />
            <text
              x={(local.x + end.x) * 500}
              y={((local.y + end.y) * height) / 2 - 8}
            >
              {label}
            </text>
            {!sameFloor && (
              <g
                className="wireless-portal"
                role="button"
                tabIndex={0}
                aria-label={`Wi-Fi to ${plan.floors.find((item) => item.id === remote.floor)?.name}`}
                onClick={() => navigate(remote.floor)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") navigate(remote.floor);
                }}
              >
                <circle cx={end.x * 1000} cy={end.y * height} r="9" />
                <text x={end.x * 1000 + 15} y={end.y * height}>
                  Wi-Fi to{" "}
                  {plan.floors.find((item) => item.id === remote.floor)?.name}
                </text>
              </g>
            )}
          </g>
        );
      })}
    </g>
  );
}
