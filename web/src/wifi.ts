export type WiFiRadio = {
  mac: string;
  interfaceName: string;
  cap?: string;
  ap?: string;
};
export type WiFiSource = {
  source: string;
  device: string;
  menu?: string;
  lastAttempt?: string;
  lastSuccess?: string;
  error?: string;
  state: string;
  radios: WiFiRadio[] | null;
};
export type WiFiClient = {
  authorized?: boolean;
  source: string;
  domain: string;
  mac: string;
  interfaceName: string;
  radioMac?: string;
  ap?: string;
  device?: string;
  interface?: string;
  ssid?: string;
  band?: string;
  signal?: number;
  rxRate?: number;
  txRate?: number;
  lastSeen: string;
  present: boolean;
  state: string;
};
export type WiFiView = { sources: WiFiSource[]; clients: WiFiClient[] };
export const emptyWiFi: WiFiView = { sources: [], clients: [] };

export const wifiState = (state: string) =>
  ({
    associated: "Associated",
    "not-associated": "Not associated",
    stale: "Stale · current association unknown",
    conflict: "Conflicting registrations",
    fresh: "Live",
    error: "Collection error",
    disabled: "Collection disabled",
  })[state] ?? state;
export const observedTime = (value?: string) =>
  value ? new Date(value).toLocaleString() : "No successful collection yet";
export const linkRate = (value?: number) =>
  value == null ? "Unknown" : `${(value / 1_000_000).toLocaleString()} Mbps`;
