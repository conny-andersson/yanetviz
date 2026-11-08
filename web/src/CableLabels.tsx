import { useEffect, useRef, useState } from "react";
import { api, endpoint, type Plan } from "./model";
import type { ChangePlan } from "./DeviceEditor";

type Preview = { id: string; label?: string; error?: string };

type Props = { plan: Plan; change: ChangePlan; cableId?: string };

export function CableLabelButton(props: Props) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button onClick={() => setOpen(true)}>
        Auto-label {props.cableId ? "cable" : "cables"}…
      </button>
      {open && <CableLabelDialog {...props} close={() => setOpen(false)} />}
    </>
  );
}

function CableLabelDialog({
  plan,
  change,
  cableId,
  close,
}: Props & { close: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [previews, setPreviews] = useState<Preview[] | null>(null);
  const [error, setError] = useState("");
  const [replace, setReplace] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const rows = (previews ?? []).filter((row) => !cableId || row.id === cableId);
  const cables = new Map(plan.cables.map((cable) => [cable.id, cable]));

  useEffect(() => {
    dialog.current?.showModal();
  }, []);

  useEffect(() => {
    let active = true;
    api<Preview[]>("/api/cable-labels/preview", "POST", plan)
      .then((result) => {
        if (!active) return;
        setPreviews(result);
        setSelected(
          new Set(
            result
              .filter(
                (row) =>
                  (!cableId || row.id === cableId) &&
                  row.label &&
                  !plan.cables
                    .find((cable) => cable.id === row.id)
                    ?.label.trim(),
              )
              .map((row) => row.id),
          ),
        );
      })
      .catch((error) => {
        if (active) setError((error as Error).message);
      });
    return () => {
      active = false;
    };
  }, [plan, cableId]);

  function apply() {
    const labels = new Map(
      rows
        .filter(
          (row) =>
            selected.has(row.id) &&
            row.label &&
            (replace || !cables.get(row.id)?.label.trim()),
        )
        .map((row) => [row.id, row.label!]),
    );
    change((plan) => ({
      ...plan,
      cables: plan.cables.map((cable) =>
        labels.has(cable.id)
          ? { ...cable, label: labels.get(cable.id)! }
          : cable,
      ),
    }));
    close();
  }

  return (
    <dialog
      className="cable-label-dialog"
      ref={dialog}
      aria-labelledby="cable-label-title"
      onCancel={(event) => {
        event.preventDefault();
        close();
      }}
    >
      <div className="section-heading">
        <h2 id="cable-label-title">Auto-label cables</h2>
        <button aria-label="Close label preview" onClick={close}>
          ×
        </button>
      </div>
      <p className="help">
        Review labels before applying. Endpoints keep their A-to-B order. Labels
        change only when you apply them, then Save changes.
      </p>
      {error && (
        <p role="alert" className="error-text">
          {error}
        </p>
      )}
      {!previews && !error && <p role="status">Preparing label preview…</p>}
      {previews && (
        <>
          <label className="checkbox">
            <input
              type="checkbox"
              checked={replace}
              onChange={(event) => {
                const checked = event.target.checked;
                setReplace(checked);
                setSelected(
                  new Set(
                    rows
                      .filter(
                        (row) =>
                          row.label &&
                          (checked || !cables.get(row.id)?.label.trim()),
                      )
                      .map((row) => row.id),
                  ),
                );
              }}
            />
            Allow replacement of existing labels
          </label>
          <div className="label-preview-list">
            {rows.map((row) => {
              const cable = cables.get(row.id)!;
              const preserved = !!cable.label.trim() && !replace;
              return (
                <div key={row.id} className="label-preview-row">
                  <input
                    type="checkbox"
                    aria-label={`Label cable ${endpoint(plan, cable.a)} to ${endpoint(plan, cable.b)}`}
                    checked={selected.has(row.id)}
                    disabled={!row.label || preserved}
                    onChange={(event) =>
                      setSelected((previous) => {
                        const next = new Set(previous);
                        if (event.target.checked) next.add(row.id);
                        else next.delete(row.id);
                        return next;
                      })
                    }
                  />
                  <div>
                    <p className="label-endpoints">
                      {endpoint(plan, cable.a)} → {endpoint(plan, cable.b)}
                      {cable.draft ? " · draft" : ""}
                    </p>
                    {cable.label && (
                      <p className="help">
                        Current: {cable.label}
                        {preserved ? " · kept" : ""}
                      </p>
                    )}
                    {row.label && <code>{row.label}</code>}
                    {row.error && <p className="error-text">{row.error}</p>}
                  </div>
                </div>
              );
            })}
          </div>
          {!rows.length && <p className="help">No cables documented yet.</p>}
        </>
      )}
      <div className="dialog-actions">
        <button onClick={close}>Cancel</button>
        <button
          className="primary"
          disabled={!selected.size || !previews}
          onClick={apply}
        >
          Apply {selected.size} label{selected.size === 1 ? "" : "s"}
        </button>
      </div>
    </dialog>
  );
}
