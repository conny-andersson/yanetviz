import { useEffect, useState } from "react";
import { api } from "./model";
import { emptyWiFi, type WiFiView } from "./wifi";
import { emptyPorts, type PortView } from "./ports";

type LiveView = { wifi: WiFiView; ports: PortView };
const empty: LiveView = { wifi: emptyWiFi, ports: emptyPorts };

// One stream per tab updates only observations, preserving working plans/undo.
export function useLiveObservations(authenticated: boolean | null) {
  const [view, setView] = useState<LiveView>(empty);
  const [error, setError] = useState("");
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    if (!authenticated) {
      setView(empty);
      return;
    }
    let active = true;
    let streamed = false;
    const accept = (next: LiveView) => {
      if (active) {
        setView(next);
        setError("");
      }
    };
    const failed = () => {
      if (active)
        setError(
          "Live observation updates disconnected. Reconnecting… Readings expire 180 seconds after their last successful poll.",
        );
    };
    api<LiveView>("/api/observations/live")
      .then((next) => {
        if (!streamed) accept(next);
      })
      .catch(() => {
        if (!streamed) failed();
      });
    const stream = new EventSource("/api/observations/events");
    stream.onmessage = (event) => {
      try {
        streamed = true;
        accept(JSON.parse(event.data));
      } catch {
        failed();
      }
    };
    stream.onerror = failed;
    stream.onopen = () => {
      if (active) setError("");
    };
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => {
      active = false;
      stream.close();
      window.clearInterval(timer);
    };
  }, [authenticated]);
  // Stream renewal and transport failures do not invalidate a successful poll.
  // Keep connection health separate, and let retained readings age normally.
  const expired = (lastSuccess?: string) =>
    !lastSuccess || now - Date.parse(lastSuccess) >= 180000;
  const staleWiFi = new Set(
    view.wifi.sources
      .filter((source) => expired(source.lastSuccess))
      .map((source) => source.source),
  );
  const stalePorts = new Set(
    view.ports.sources
      .filter((source) => expired(source.lastSuccess))
      .map((source) => `${source.source}/${source.channel}`),
  );
  return {
    error,
    wifi: {
      sources: view.wifi.sources.map((source) =>
        source.state === "fresh" && staleWiFi.has(source.source)
          ? { ...source, state: "stale" }
          : source,
      ),
      clients: view.wifi.clients.map((client) =>
        staleWiFi.has(client.source) ? { ...client, state: "stale" } : client,
      ),
    },
    ports: {
      sources: view.ports.sources.map((source) =>
        source.state === "fresh" &&
        stalePorts.has(`${source.source}/${source.channel}`)
          ? { ...source, state: "stale" }
          : source,
      ),
      readings: view.ports.readings.map((reading) =>
        stalePorts.has(`${reading.source}/${reading.channel}`)
          ? { ...reading, state: "stale" }
          : reading,
      ),
    },
  };
}
