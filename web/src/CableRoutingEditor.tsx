import { Field } from "./Fields";
import type { Cable, Plan, RouteSection } from "./model";
import type { ChangePlan } from "./DeviceEditor";
import { cableLocation, replaceSections, routeSections } from "./routes";

type Props = {
  plan: Plan;
  cable: Cable;
  edit: boolean;
  change: ChangePlan;
  navigate: (floor: string) => void;
};

export function CableRoutingEditor({
  plan,
  cable,
  edit,
  change,
  navigate,
}: Props) {
  const sections = routeSections(plan, cable);
  const update = (next: RouteSection[]) =>
    change((current) => replaceSections(current, cable.id, next));
  const patch = (index: number, values: Partial<RouteSection>) =>
    update(
      sections.map((section, i) =>
        i === index ? { ...section, ...values } : section,
      ),
    );
  const a = cableLocation(plan, cable.a.device);
  const b = cableLocation(plan, cable.b.device);
  const fromLevel = plan.floors.find((floor) => floor.id === a?.floor)?.level;
  const toLevel = plan.floors.find((floor) => floor.id === b?.floor)?.level;
  const unused = plan.floors.filter(
    (floor) => !sections.some((section) => section.floor === floor.id),
  );
  const suggested = (level: number | null | undefined) =>
    level != null &&
    fromLevel != null &&
    toLevel != null &&
    level > Math.min(fromLevel, toLevel) &&
    level < Math.max(fromLevel, toLevel);
  const mismatched =
    (a && sections[0]?.floor !== a.floor) ||
    (b && sections.at(-1)?.floor !== b.floor);
  return (
    <section className="routing-editor">
      <h3>Cable route · A → B</h3>
      <p className="help">
        Double-click a cable line to add a bend point. Drag its points and
        circular floor markers in Edit mode. Double-click a bend point or press
        Delete while it is focused to remove it. Geometry does not measure cable
        length.
      </p>
      {mismatched && (
        <p role="alert">
          An endpoint has moved to another floor. Reset the route to reconnect
          its drawing; the saved floor sections have been preserved.
        </p>
      )}
      {!sections.length && (
        <p className="help">
          Place an endpoint on a floor before drawing its route.
        </p>
      )}
      <ol>
        {sections.map((section, index) => (
          <li key={section.floor}>
            <button onClick={() => navigate(section.floor)}>
              {plan.floors.find((floor) => floor.id === section.floor)?.name}
            </button>
            <span className="help">
              {index === 0 && a?.floor === section.floor ? " · A" : " · entry"}{" "}
              →{" "}
              {index === sections.length - 1 && b?.floor === section.floor
                ? "B"
                : "exit"}
            </span>
            <label className="checkbox">
              <input
                type="checkbox"
                disabled={!edit}
                checked={section.confirmed}
                onChange={(event) =>
                  patch(index, { confirmed: event.target.checked })
                }
              />
              Route documented on this floor
            </label>
            {edit && (
              <div className="route-actions">
                <button
                  disabled={!section.points?.length}
                  onClick={() => patch(index, { points: [] })}
                >
                  Straighten section
                </button>
                <button
                  disabled={!section.entry && !section.exit}
                  onClick={() =>
                    patch(index, { entry: undefined, exit: undefined })
                  }
                >
                  Reset floor markers
                </button>
                {index > 0 && index < sections.length - 1 && (
                  <>
                    <button
                      aria-label="Move section earlier"
                      disabled={index === 1}
                      onClick={() => {
                        const next = [...sections];
                        [next[index - 1], next[index]] = [
                          next[index],
                          next[index - 1],
                        ];
                        update(next);
                      }}
                    >
                      ↑
                    </button>
                    <button
                      aria-label="Move section later"
                      disabled={index === sections.length - 2}
                      onClick={() => {
                        const next = [...sections];
                        [next[index + 1], next[index]] = [
                          next[index],
                          next[index + 1],
                        ];
                        update(next);
                      }}
                    >
                      ↓
                    </button>
                    <button
                      onClick={() =>
                        update(sections.filter((_, i) => i !== index))
                      }
                    >
                      Remove section
                    </button>
                  </>
                )}
              </div>
            )}
          </li>
        ))}
      </ol>
      {edit && sections.length >= 2 && unused.length > 0 && (
        <Field label="Insert intermediate floor before endpoint B">
          <select
            value=""
            onChange={(event) => {
              if (!event.target.value) return;
              update([
                ...sections.slice(0, -1),
                { floor: event.target.value, confirmed: false },
                sections[sections.length - 1],
              ]);
            }}
          >
            <option value="">Choose a floor / area…</option>
            {unused.map((floor) => (
              <option key={floor.id} value={floor.id}>
                {floor.name}
                {floor.level != null
                  ? ` · level ${floor.level}`
                  : " · level unknown"}
                {suggested(floor.level) ? " · intermediate level" : ""}
              </option>
            ))}
          </select>
        </Field>
      )}
      {edit && !!cable.sections?.length && (
        <button
          onClick={() => {
            if (
              window.confirm(
                "Reset the entire cable route? All bend points, floor marker positions and intermediate sections will be removed. Physical endpoints and cable label are preserved.",
              )
            )
              update([]);
          }}
        >
          Reset entire route…
        </button>
      )}
    </section>
  );
}
