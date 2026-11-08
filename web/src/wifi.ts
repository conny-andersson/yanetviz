import { useEffect, useState } from "react";
import { api } from "./model";

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

// Observation updates never replace the editor's plan or undo history.
export function useWiFi(authenticated: boolean | null) {
  const [view, setView] = useState<WiFiView>(emptyWiFi);
  const [error, setError] = useState("");
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    if (!authenticated) {
      setView(emptyWiFi);
      return;
    }
    let active = true;
    const accept = (next: WiFiView) => {
      if (active) {
        setView(next);
        setError("");
      }
    };
    const failed = () => {
      if (active)
        setError(
          "Live Wi-Fi updates unavailable; retained observations are stale. Reconnecting…",
        );
    };
    api<WiFiView>("/api/observations/wifi").then(accept).catch(failed);
    const stream = new EventSource("/api/observations/wifi/events");
    stream.onmessage = (event) => {
      try {
        accept(JSON.parse(event.data));
      } catch {
        failed();
      }
    };
    stream.onerror = failed;
    const clock = window.setInterval(() => setNow(Date.now()), 1000);
    return () => {
      active = false;
      stream.close();
      window.clearInterval(clock);
    };
  }, [authenticated]);
  const expired = (source: WiFiSource) =>
    !!error ||
    !source.lastSuccess ||
    now - Date.parse(source.lastSuccess) >= 45000;
  const staleSources = new Set(
    view.sources.filter(expired).map((source) => source.source),
  );
  return {
    error,
    view: {
      sources: view.sources.map((source) =>
        source.state === "fresh" && staleSources.has(source.source)
          ? { ...source, state: "stale" }
          : source,
      ),
      clients: view.clients.map((client) =>
        staleSources.has(client.source)
          ? { ...client, state: "stale" }
          : client,
      ),
    },
  };
}

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
