import { CableLabelButton } from "./CableLabels";
import { Inventory } from "./Inventory";
import { Topology, type CableSort } from "./Topology";
import { useCallback, useEffect, useRef, useState } from "react";
import { CabinetEditor } from "./CabinetEditor";
import { CableEditor } from "./CableEditor";
import { DeviceEditor, type ChangePlan } from "./DeviceEditor";
import { Map } from "./Map";
import {
  api,
  download,
  uid,
  type DHCPObservation,
  type Plan,
  type Selection,
} from "./model";

export function App() {
  const [plan, setPlan] = useState<Plan | null>(null);
  const [observations, setObservations] = useState<DHCPObservation[]>([]);
  const [observationError, setObservationError] = useState("");
  const [saved, setSaved] = useState<Plan | null>(null);
  const [history, setHistory] = useState<Plan[]>([]);
  const [authenticated, setAuthenticated] = useState<boolean | null>(null);
  const [loginRequired, setLoginRequired] = useState(false);
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [saving, setSaving] = useState(false);
  const [edit, setEdit] = useState(false);
  const [floor, setFloor] = useState("basement");
  const [view, setView] = useState("floor");
  const [cableSort, setCableSort] = useState<CableSort>("original");
  const [selection, setSelection] = useState<Selection>(null);
  const [zoom, setZoom] = useState(1);
  const [showCables, setShowCables] = useState(true);
  const importInput = useRef<HTMLInputElement>(null);
  const backgroundInput = useRef<HTMLInputElement>(null);
  const dirty = plan !== saved;

  useEffect(() => {
    setZoom(1);
  }, [floor]);

  const loadObservations = useCallback(() => {
    api<DHCPObservation[]>("/api/observations/dhcp")
      .then((records) => {
        setObservations(records);
        setObservationError("");
      })
      .catch(() => {
        setObservations([]);
        setObservationError(
          "DHCP observations could not be loaded. Documentation remains available.",
        );
      });
  }, []);

  const load = useCallback(async () => {
    const current = await api<Plan>("/api/plan");
    setPlan(current);
    setSaved(current);
    loadObservations();
    setHistory([]);
    setError("");
    setFloor((previous) =>
      current.floors.some((floor) => floor.id === previous)
        ? previous
        : current.floors[0].id,
    );
  }, [loadObservations]);

  useEffect(() => {
    api<{ authenticated: boolean; loginRequired: boolean }>("/api/session")
      .then((session) => {
        setAuthenticated(session.authenticated);
        setLoginRequired(session.loginRequired);
        if (session.authenticated) return load();
      })
      .catch((error) => setError(String(error.message)));
  }, [load]);

  useEffect(() => {
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (dirty) event.preventDefault();
    };
    window.addEventListener("beforeunload", beforeUnload);
    return () => window.removeEventListener("beforeunload", beforeUnload);
  }, [dirty]);

  const change: ChangePlan = (update) => {
    if (!edit || saving || !plan) return;
    setHistory((previous) => [...previous.slice(-29), plan]);
    setPlan(update(plan));
    setNotice("");
    setError("");
  };

  async function save() {
    if (!plan || saving) return;
    setSaving(true);
    setError("");
    try {
      const next = await api<Plan>("/api/plan", "PUT", plan);
      setPlan(next);
      setSaved(next);
      setHistory([]);
      loadObservations();
      setNotice("Documentation saved");
    } catch (error) {
      setError(
        (error as Error).message + " Your pending edits are still here.",
      );
    } finally {
      setSaving(false);
    }
  }

  async function reload() {
    if (
      dirty &&
      !window.confirm(
        "Discard pending edits and reload the saved plan? Download the working copy first if you want to keep it.",
      )
    )
      return;
    try {
      await load();
      setNotice("Loaded saved documentation");
    } catch (error) {
      setError((error as Error).message);
    }
  }

  function add(kind: "device" | "cabinet" | "cable") {
    if (!plan) return;
    const id = uid();
    if (kind === "device")
      change((plan) => ({
        ...plan,
        devices: [
          ...plan.devices,
          {
            id,
            name: "New equipment",
            role: "client",
            model: "",
            address: "",
            notes: "",
            mode: "fixed",
            floor: "",
            cabinet: "",
            x: 0.5,
            y: 0.5,
          },
        ],
      }));
    if (kind === "cabinet")
      change((plan) => ({
        ...plan,
        cabinets: [
          ...plan.cabinets,
          { id, name: "New cabinet", floor, x: 0.5, y: 0.5, placed: false },
        ],
      }));
    if (kind === "cable") {
      if (plan.devices.length < 2) {
        setError("Add at least two devices before documenting a cable.");
        return;
      }
      const first =
        selection?.kind === "device" ? selection.id : plan.devices[0].id;
      const second = plan.devices.find((device) => device.id !== first)!.id;
      change((plan) => ({
        ...plan,
        cables: [
          ...plan.cables,
          {
            id,
            a: { device: first, port: "" },
            b: { device: second, port: "" },
            medium: "unknown",
            label: "",
            length: null,
            notes: "",
            draft: true,
          },
        ],
      }));
    }
    setSelection({ kind, id });
  }

  async function importPlan(file?: File) {
    if (!file || !plan) return;
    if (file.size > 32 * 1024 * 1024) {
      setError("Plan file must be smaller than 32 MiB.");
      return;
    }
    if (
      !window.confirm(
        "Replace the working plan with this file? Changes only become permanent when you Save.",
      )
    )
      return;
    setSaving(true);
    try {
      const imported = JSON.parse(await file.text());
      const validated = await api<Plan>("/api/validate", "POST", imported);
      setHistory((previous) => [...previous.slice(-29), plan]);
      setPlan({ ...validated, revision: plan.revision });
      setFloor(validated.floors[0].id);
      setSelection(null);
      setNotice("Imported into working copy. Review and Save to keep it.");
      setError("");
    } catch (error) {
      setError(`Import failed: ${(error as Error).message}`);
    } finally {
      setSaving(false);
    }
  }

  async function replaceBackground(file?: File) {
    if (!file || !plan) return;
    if (
      !["image/png", "image/jpeg"].includes(file.type) ||
      file.size > 8 * 1024 * 1024
    ) {
      setError("Choose a PNG or JPEG smaller than 8 MiB.");
      return;
    }
    const reader = new FileReader();
    reader.onerror = () => setError("Could not read the image.");
    reader.onload = () => {
      const background = String(reader.result);
      change((plan) => ({
        ...plan,
        floors: plan.floors.map((item) =>
          item.id === floor ? { ...item, background } : item,
        ),
      }));
      setNotice(
        "Background preview: positions are unchanged. Review before saving; use Undo to revert.",
      );
    };
    reader.readAsDataURL(file);
  }

  if (authenticated === false)
    return (
      <main className="login">
        <form
          onSubmit={async (event) => {
            event.preventDefault();
            setError("");
            try {
              await api("/api/session", "POST", { password });
              setPassword("");
              await load();
              setAuthenticated(true);
            } catch (error) {
              setError((error as Error).message);
            }
          }}
        >
          <div className="brand-mark">Y</div>
          <span className="eyebrow">Your network, documented</span>
          <h1>Welcome to Yanetviz</h1>
          <label className="field">
            <span>Administrator password</span>
            <input
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              required
              autoFocus
            />
          </label>
          {error && (
            <p role="alert" className="error-text">
              {error}
            </p>
          )}
          <button className="primary">Sign in</button>
        </form>
      </main>
    );
  if (!plan)
    return (
      <main className="loading">
        <h1>Yanetviz</h1>
        <p>{error || "Opening your network documentation…"}</p>
        <button onClick={() => window.location.reload()}>Retry</button>
      </main>
    );

  const device =
    selection?.kind === "device"
      ? plan.devices.find((device) => device.id === selection.id)
      : null;
  const cabinet =
    selection?.kind === "cabinet"
      ? plan.cabinets.find((cabinet) => cabinet.id === selection.id)
      : null;
  const cable =
    selection?.kind === "cable"
      ? plan.cables.find((cable) => cable.id === selection.id)
      : null;

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">
          <span className="brand-mark">Y</span>
          <div>
            <strong>yanetviz</strong>
            <small>Home network atlas</small>
          </div>
        </div>
        <div className="project-status">
          <span className="status-dot" />
          Manual documentation <span className="subtle">/</span>{" "}
          <span>
            {dirty ? "Unsaved changes" : `Saved · revision ${plan.revision}`}
          </span>
        </div>
        <div className="top-actions">
          <button onClick={reload} disabled={saving}>
            Reload
          </button>
          <button onClick={() => download(plan)}>Download working copy</button>
          <a className="button" href="/api/export">
            Export saved plan
          </a>
          {loginRequired && (
            <button
              onClick={async () => {
                if (
                  dirty &&
                  !window.confirm("Sign out and discard unsaved changes?")
                )
                  return;
                await fetch("/api/session", {
                  method: "DELETE",
                  headers: { "X-Yanetviz": "1" },
                });
                setPlan(null);
                setSaved(null);
                setAuthenticated(false);
              }}
            >
              Sign out
            </button>
          )}
        </div>
      </header>
      {error && (
        <div className="message error" role="alert">
          {error}
          <button onClick={() => setError("")} aria-label="Dismiss error">
            ×
          </button>
        </div>
      )}
      {notice && (
        <div className="message" role="status">
          {notice}
          <button
            onClick={() => setNotice("")}
            aria-label="Dismiss notification"
          >
            ×
          </button>
        </div>
      )}
      <div className="workspace" inert={saving}>
        <Inventory
          plan={plan}
          observations={observations}
          selection={selection}
          select={setSelection}
          navigate={setFloor}
        />
        <main className="main-view">
          <div className="view-toolbar">
            <nav className="view-tabs" aria-label="Views">
              <button
                className={view === "floor" ? "active" : ""}
                onClick={() => setView("floor")}
              >
                Floor plan
              </button>
              <button
                className={view === "topology" ? "active" : ""}
                onClick={() => setView("topology")}
              >
                Topology & cables
              </button>
            </nav>
            <div className="edit-actions">
              {edit && (
                <>
                  <button
                    disabled={!history.length}
                    onClick={() => {
                      const previous = history.at(-1);
                      if (previous) {
                        setPlan(previous);
                        setHistory(history.slice(0, -1));
                      }
                    }}
                  >
                    Undo
                  </button>
                  <button className="primary" disabled={!dirty} onClick={save}>
                    {saving ? "Saving…" : "Save changes"}
                  </button>
                </>
              )}
              <button
                className={`mode-button ${edit ? "editing" : ""}`}
                onClick={() => setEdit(!edit)}
              >
                {edit ? "✎ Edit mode" : "View mode"}
              </button>
            </div>
          </div>
          <div className="floor-toolbar">
            <nav className="floor-tabs" aria-label="Floors">
              {plan.floors.map((item) => (
                <button
                  key={item.id}
                  className={item.id === floor ? "active" : ""}
                  onClick={() => {
                    setFloor(item.id);
                    setZoom(1);
                  }}
                >
                  {item.name}
                </button>
              ))}
            </nav>
            {view === "floor" && (
              <div className="canvas-controls">
                <label className="checkbox">
                  <input
                    type="checkbox"
                    checked={showCables}
                    onChange={(event) => setShowCables(event.target.checked)}
                  />
                  Cables
                </label>
                <button
                  aria-label="Zoom out"
                  disabled={zoom <= 0.5}
                  onClick={() => setZoom(Math.max(0.5, zoom - 0.25))}
                >
                  −
                </button>
                <button
                  aria-label="Fit floor plan"
                  title="Fit the entire floor plan"
                  aria-pressed={zoom === 1}
                  onClick={() => setZoom(1)}
                >
                  {zoom === 1 ? "Fit" : `${Math.round(zoom * 100)}%`}
                </button>
                <button
                  aria-label="Zoom in"
                  disabled={zoom >= 3}
                  onClick={() => setZoom(Math.min(3, zoom + 0.25))}
                >
                  +
                </button>
              </div>
            )}
          </div>
          {edit && (
            <div className="creation-toolbar">
              <button onClick={() => add("device")}>+ Equipment</button>
              <button onClick={() => add("cabinet")}>+ Cabinet</button>
              <button onClick={() => add("cable")}>+ Cable</button>
              <CableLabelButton plan={plan} change={change} />
              <label className="floor-level">
                Floor level (optional)
                <input
                  aria-label="Floor level (optional)"
                  type="number"
                  min="-1000"
                  max="1000"
                  step="1"
                  placeholder="Unknown"
                  value={
                    plan.floors.find((item) => item.id === floor)?.level ?? ""
                  }
                  onChange={(event) => {
                    const level =
                      event.target.value === ""
                        ? null
                        : Number(event.target.value);
                    if (
                      level !== null &&
                      (!Number.isInteger(level) || Math.abs(level) > 1000)
                    )
                      return;
                    change((current) => ({
                      ...current,
                      floors: current.floors.map((item) =>
                        item.id === floor ? { ...item, level } : item,
                      ),
                    }));
                  }}
                />
              </label>
              <span />
              <button onClick={() => backgroundInput.current?.click()}>
                Replace background
              </button>
              <button onClick={() => importInput.current?.click()}>
                Import plan
              </button>
            </div>
          )}
          {view === "floor" ? (
            <Map
              key={floor}
              plan={plan}
              floor={floor}
              edit={edit}
              selection={selection}
              select={setSelection}
              navigate={setFloor}
              showCables={showCables}
              zoom={zoom}
              change={change}
              move={(kind, id, x, y) =>
                change((plan) =>
                  kind === "cabinet"
                    ? {
                        ...plan,
                        cabinets: plan.cabinets.map((cabinet) =>
                          cabinet.id === id ? { ...cabinet, x, y } : cabinet,
                        ),
                      }
                    : {
                        ...plan,
                        devices: plan.devices.map((device) =>
                          device.id === id ? { ...device, x, y } : device,
                        ),
                      },
                )
              }
            />
          ) : (
            <Topology
              plan={plan}
              selection={selection}
              select={setSelection}
              cableSort={cableSort}
              setCableSort={setCableSort}
            />
          )}
          <footer className="canvas-footer">
            <span>
              {edit
                ? "Edit mode · select equipment to document it; drag placed markers to move them."
                : "Select equipment or a cable to inspect its documentation."}
            </span>
            <span>
              <i className="line copper" />
              Copper <i className="line fiber" />
              Fiber
            </span>
          </footer>
        </main>
        <aside className="details-panel">
          {device ? (
            <DeviceEditor
              key={device.id}
              device={device}
              plan={plan}
              observations={observations}
              observationError={observationError}
              edit={edit}
              floor={floor}
              change={change}
              select={setSelection}
            />
          ) : cabinet ? (
            <CabinetEditor
              cabinet={cabinet}
              plan={plan}
              edit={edit}
              change={change}
              select={setSelection}
            />
          ) : cable ? (
            <CableEditor
              navigate={(id) => {
                setFloor(id);
                setView("floor");
              }}
              cable={cable}
              plan={plan}
              edit={edit}
              change={change}
              select={setSelection}
            />
          ) : (
            <div className="welcome-panel">
              <span className="eyebrow">Make the network yours</span>
              <h2>A place for every connection.</h2>
              <p>
                Document the equipment, sockets, and cables that make your home
                network work.
              </p>
              <div className="welcome-step">
                <b>01</b>
                <span>
                  Switch to <strong>Edit mode</strong> to add or update
                  documentation.
                </span>
              </div>
              <div className="welcome-step">
                <b>02</b>
                <span>
                  Select a cabinet or device, then place it on a floor plan.
                </span>
              </div>
              <div className="welcome-step">
                <b>03</b>
                <span>
                  Add ports, connect cable segments, and save your work.
                </span>
              </div>
              <p className="help">
                The confirmed backbone is included as drafts. Unknown locations
                and ports stay unknown until you document them.
              </p>
            </div>
          )}
        </aside>
      </div>
      <input
        hidden
        ref={importInput}
        type="file"
        accept="application/json,.json"
        onChange={(event) => {
          importPlan(event.target.files?.[0]);
          event.target.value = "";
        }}
      />
      <input
        hidden
        ref={backgroundInput}
        type="file"
        accept="image/png,image/jpeg"
        onChange={(event) => {
          replaceBackground(event.target.files?.[0]);
          event.target.value = "";
        }}
      />
    </div>
  );
}
