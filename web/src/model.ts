export type Floor = { id: string; name: string; background: string };
export type Cabinet = {
  id: string;
  name: string;
  floor: string;
  x: number;
  y: number;
  placed: boolean;
};
export type Device = {
  id: string;
  name: string;
  role: string;
  model: string;
  address: string;
  notes: string;
  mode: string;
  floor: string;
  cabinet: string;
  x: number;
  y: number;
};
export type Port = {
  id: string;
  device: string;
  name: string;
  label: string;
  connector: string;
  position: string;
  side: string;
  poeMode: string;
  poeStandard: string;
  voltage: string;
  notes: string;
};
export type Endpoint = { device: string; port: string };
export type Cable = {
  id: string;
  a: Endpoint;
  b: Endpoint;
  medium: string;
  label: string;
  length: number | null;
  notes: string;
  draft: boolean;
};
export type PassThrough = { id: string; a: string; b: string };
export type Plan = {
  version: number;
  revision: number;
  floors: Floor[];
  cabinets: Cabinet[];
  devices: Device[];
  ports: Port[];
  cables: Cable[];
  passThroughs: PassThrough[];
};
export type Selection = {
  kind: "device" | "cabinet" | "cable";
  id: string;
} | null;
export const uid = () => crypto.randomUUID();
export const roles: Record<string, string> = {
  router: "Router",
  switch: "Switch",
  access_point: "Access point",
  client: "Client",
  patch_panel: "Patch panel",
  wall_socket: "Wall socket",
  termination: "Fiber termination",
  injector: "PoE injector",
  other: "Other",
};
export const passive = (d: Device) =>
  ["patch_panel", "wall_socket", "termination"].includes(d.role);
export function location(p: Plan, d: Device) {
  if (d.mode === "mobile") return null;
  if (d.cabinet) {
    const c = p.cabinets.find((c) => c.id === d.cabinet);
    return c?.placed ? c : null;
  }
  return d.floor ? d : null;
}
export function endpoint(p: Plan, e: Endpoint) {
  const d = p.devices.find((d) => d.id === e.device);
  const port = p.ports.find((v) => v.id === e.port);
  return `${d?.name ?? "?"} / ${port?.name ?? "port unknown"}`;
}
export function trace(p: Plan, cable: Cable): string[] {
  const walk = (start: Endpoint, seen: Set<string>): string[] => {
    const out = [endpoint(p, start)];
    let current = start;
    for (let i = 0; i < p.ports.length + p.cables.length + 1; i++) {
      if (!current.port) return [...out, "Unresolved port — end of known path"];
      const device = p.devices.find((d) => d.id === current.device)!;
      if (!passive(device)) return out;
      const pairs = p.passThroughs.filter(
        (v) => v.a === current.port || v.b === current.port,
      );
      if (pairs.length !== 1)
        return [
          ...out,
          pairs.length
            ? "Ambiguous pass-through"
            : "Unmapped passive terminal — end of known path",
        ];
      const nextPort = pairs[0].a === current.port ? pairs[0].b : pairs[0].a;
      out.push(
        "↔ Internal pass-through",
        endpoint(p, { device: current.device, port: nextPort }),
      );
      const links = p.cables.filter(
        (c) => c.a.port === nextPort || c.b.port === nextPort,
      );
      if (links.length !== 1)
        return [
          ...out,
          links.length
            ? "Ambiguous branch"
            : "No documented cable — end of known path",
        ];
      const link = links[0];
      if (seen.has(link.id)) return [...out, "Loop detected — ambiguous path"];
      seen.add(link.id);
      out.push(`— ${link.medium} ${link.label}${link.draft ? " (draft)" : ""}`);
      current = link.a.port === nextPort ? link.b : link.a;
      out.push(endpoint(p, current));
    }
    return [...out, "Loop detected"];
  };
  const seen = new Set([cable.id]);
  return [
    ...walk(cable.a, seen).reverse(),
    `— ${cable.medium} ${cable.label}${cable.draft ? " (draft)" : ""}`,
    ...walk(cable.b, seen),
  ];
}
export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: { "Content-Type": "application/json", "X-Yanetviz": "1" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}
export function download(p: Plan) {
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(p, null, 2)], { type: "application/json" }),
  );
  const a = document.createElement("a");
  a.href = url;
  a.download = "yanetviz-plan.json";
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
