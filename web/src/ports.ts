import type { Cable, Plan } from "./model";

export type PoEReading = {
  mode: string;
  vendorMode?: string;
  voltageSelection?: string;
  status: string;
  voltageV?: number;
  currentMA?: number;
  powerW?: number;
};
export type PortReading = {
  source: string;
  device: string;
  channel: string;
  state: string;
  port?: string;
  name: string;
  defaultName?: string;
  mac?: string;
  disabled?: boolean;
  link?: string;
  rateMbps?: number;
  fullDuplex?: boolean;
  poe?: PoEReading;
  present: boolean;
  lastSeen: string;
};
export type DeviceInventory = {
  model?: string;
  boardName?: string;
  osVersion?: string;
  architecture?: string;
  lastAttempt: string;
  lastSuccess?: string;
  error?: string;
};
export type PortSource = {
  inventory?: DeviceInventory;
  source: string;
  device: string;
  channel: string;
  capability: string;
  state: string;
  lastAttempt?: string;
  lastSuccess?: string;
  error?: string;
};
export type PortView = { sources: PortSource[]; readings: PortReading[] };
export const emptyPorts: PortView = { sources: [], readings: [] };

export function portObservation(view: PortView, port: string, channel: string) {
  return view.readings
    .filter((reading) => reading.port === port && reading.channel === channel)
    .sort((a, b) => {
      if ((a.state === "fresh") !== (b.state === "fresh"))
        return a.state === "fresh" ? -1 : 1;
      return Date.parse(b.lastSeen) - Date.parse(a.lastSeen);
    })[0];
}

export function linkDescription(reading?: PortReading) {
  if (!reading) return "Link status unknown";
  const state =
    reading.link === "up"
      ? "Link up"
      : reading.link === "down"
        ? "Link down"
        : "Link status unknown";
  const rate =
    reading.link === "up" && reading.rateMbps != null
      ? ` · ${reading.rateMbps.toLocaleString()} Mbps`
      : "";
  const duplex =
    reading.fullDuplex == null
      ? ""
      : reading.fullDuplex
        ? " · Full duplex"
        : " · Half duplex";
  const freshness =
    reading.state === "fresh"
      ? ""
      : reading.state === "missing"
        ? " · not in latest inventory; current status unknown"
        : " · stale; current status unknown";
  return `${state}${rate}${duplex}${freshness}${reading.disabled ? " · disabled" : ""}`;
}

export type PowerFlow = {
  cable: string;
  fromA: boolean;
  source: PortReading;
  passive: boolean;
};

export type PowerPath = {
  source: PortReading;
  segments: PowerFlow[];
  ports: string[];
};

// Propagate the supplying port's status through documented copper and explicit
// passive pairs. Active equipment receives power but never joins its own ports.
// Keeping all observation states lets downstream details show faults and stale
// evidence without drawing them as current power flow.
export function powerPaths(plan: Plan, view: PortView): PowerPath[] {
  const paths: PowerPath[] = [];
  const devices = new Map(plan.devices.map((device) => [device.id, device]));
  const ports = new Map(plan.ports.map((port) => [port.id, port]));
  const cablesByPort = new Map<string, Cable[]>();
  for (const cable of plan.cables) {
    for (const terminal of [cable.a.port, cable.b.port]) {
      if (terminal)
        cablesByPort.set(terminal, [
          ...(cablesByPort.get(terminal) ?? []),
          cable,
        ]);
    }
  }
  const pairs = new Map<string, string[]>();
  for (const pair of plan.passThroughs) {
    pairs.set(pair.a, [...(pairs.get(pair.a) ?? []), pair.b]);
    pairs.set(pair.b, [...(pairs.get(pair.b) ?? []), pair.a]);
  }
  const sourcePorts = new Set(
    view.readings
      .filter(
        (reading) => reading.channel === "poe" && reading.port && reading.poe,
      )
      .map((reading) => reading.port!),
  );
  for (const sourcePort of sourcePorts) {
    const source = portObservation(view, sourcePort, "poe");
    if (!source?.poe || ports.get(sourcePort)?.device !== source.device)
      continue;
    let terminal = sourcePort;
    let passive = false;
    const visited = new Set<string>();
    const traversedPorts = new Set<string>([terminal]);
    const segments: PowerFlow[] = [];
    while (terminal) {
      const cables = cablesByPort.get(terminal) ?? [];
      if (cables.length !== 1) break;
      const cable = cables[0];
      if (visited.has(cable.id) || cable.medium !== "copper") break;
      visited.add(cable.id);
      const fromA = cable.a.port === terminal;
      const next = fromA ? cable.b : cable.a;
      const device = devices.get(next.device);
      const isPassive =
        !!device &&
        ["patch_panel", "wall_socket", "termination"].includes(device.role);
      segments.push({
        cable: cable.id,
        fromA,
        source,
        passive: passive || isPassive,
      });
      if (!next.port || ports.get(next.port)?.device !== next.device) break;
      traversedPorts.add(next.port);
      if (!isPassive) break;
      passive = true;
      const peers = pairs.get(next.port) ?? [];
      if (peers.length !== 1 || ports.get(peers[0])?.device !== next.device)
        break;
      terminal = peers[0];
      traversedPorts.add(terminal);
    }
    if (segments.length)
      paths.push({ source, segments, ports: [...traversedPorts] });
  }
  return paths;
}

// Only fresh, present, explicitly powered outputs draw current flow arrows.
export function powerFlows(plan: Plan, view: PortView): PowerFlow[] {
  return powerPaths(plan, view)
    .filter(
      (path) =>
        path.source.state === "fresh" &&
        path.source.present &&
        path.source.poe?.status === "powered-on",
    )
    .flatMap((path) => path.segments);
}

export function powerStatus(source: PortReading) {
  const status = source.poe?.status ?? "unknown";
  const label = status === "powered-on" ? "Powered" : status;
  return source.state === "fresh" && source.present
    ? label
    : `Current power unknown · last reported: ${label} (${source.state})`;
}

export function portCapabilityLabel(state: string) {
  return (
    {
      fresh: "Live",
      unavailable: "Menu/monitor unavailable",
      unsupported: "PoE-out not supported (configured)",
      disabled: "Polling disabled (configured)",
      error: "Collection error",
      stale: "Stale",
    }[state] ?? state
  );
}
