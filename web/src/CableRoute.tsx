import { useRef, useState } from "react";
import {
  endpoint,
  type Cable,
  type Plan,
  type RoutePoint,
  type RouteSection,
} from "./model";
import type { PowerFlow } from "./ports";
import { routeSections } from "./routes";

type Located = RoutePoint & { floor: string };
type Target = "entry" | "exit" | number;
type Props = {
  power: PowerFlow[];
  plan: Plan;
  cable: Cable;
  floor: string;
  height: number;
  index: number;
  a: Located | null;
  b: Located | null;
  selected: boolean;
  edit: boolean;
  select: () => void;
  navigate: (floor: string) => void;
  update: (sections: RouteSection[]) => void;
};

export function CableRoute({
  plan,
  power,
  cable,
  floor,
  height,
  index,
  a,
  b,
  selected,
  edit,
  select,
  navigate,
  update,
}: Props) {
  const gesture = useRef<{
    target: Target;
    pointer: number;
    x: number;
    y: number;
    original: RoutePoint;
    moved: boolean;
  } | null>(null);
  const [preview, setPreview] = useState<{
    target: Target;
    point: RoutePoint;
  } | null>(null);
  const sections = routeSections(plan, cable);
  const sectionIndex = sections.findIndex((section) => section.floor === floor);
  if (sectionIndex < 0) return null;
  const section = sections[sectionIndex];
  const anchoredA = sectionIndex === 0 && a?.floor === floor;
  const anchoredB = sectionIndex === sections.length - 1 && b?.floor === floor;
  const localY = anchoredA ? a.y : anchoredB ? b.y : 0.5;
  const fallbackY = Math.max(
    30 / height,
    Math.min(1 - 30 / height, localY + (((index % 3) - 1) * 65) / height),
  );
  const entry = section.entry ?? { x: 0.82, y: fallbackY };
  const exit = section.exit ?? { x: 0.93, y: fallbackY };
  const shown = (target: Target, point: RoutePoint) =>
    preview?.target === target ? preview.point : point;
  const first = anchoredA ? a : shown("entry", entry);
  const last = anchoredB ? b : shown("exit", exit);
  const bends = (section.points ?? []).map((point, i) => shown(i, point));
  const points = [first, ...bends, last];
  const path = points
    .map((point, i) => `${i ? "L" : "M"}${point.x * 1000},${point.y * height}`)
    .join(" ");
  const patch = (change: Partial<RouteSection>) =>
    update(
      sections.map((item, i) =>
        i === sectionIndex ? { ...item, ...change } : item,
      ),
    );
  const setPoint = (target: Target, point: RoutePoint) => {
    if (typeof target === "number")
      patch({ points: bends.map((item, i) => (i === target ? point : item)) });
    else patch({ [target]: point });
  };
  const removePoint = (target: number) =>
    patch({ points: bends.filter((_, i) => i !== target) });
  const position = (
    event: React.MouseEvent<SVGElement> | React.PointerEvent<SVGElement>,
  ) => {
    const matrix = event.currentTarget.ownerSVGElement!.getScreenCTM()!;
    const point = new DOMPoint(event.clientX, event.clientY).matrixTransform(
      matrix.inverse(),
    );
    return {
      x: Math.max(0, Math.min(1, point.x / 1000)),
      y: Math.max(0, Math.min(1, point.y / height)),
    };
  };
  const handle = (target: Target, point: RoutePoint, destination?: string) => {
    const portal = typeof target === "string";
    const remoteEndpoint = target === "entry" ? cable.a : cable.b;
    const label = portal
      ? `To ${plan.floors.find((item) => item.id === destination)?.name ?? "unplaced"} / ${plan.devices.find((item) => item.id === remoteEndpoint.device)?.name ?? "?"}`
      : `Bend point ${target + 1}`;
    return (
      <g
        key={target}
        className={portal ? "portal" : "route-handle"}
        role="button"
        tabIndex={0}
        aria-label={label}
        onClick={(event) => {
          event.stopPropagation();
          select();
          if (!edit && destination) navigate(destination);
        }}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            select();
            if (!edit && destination) navigate(destination);
          }
          if (
            edit &&
            !portal &&
            (event.key === "Delete" || event.key === "Backspace")
          ) {
            event.preventDefault();
            removePoint(target as number);
          }
        }}
        onDoubleClick={(event) => {
          event.stopPropagation();
          if (edit && !portal) removePoint(target as number);
        }}
        onPointerDown={(event) => {
          if (!edit || event.button !== 0 || !event.isPrimary) return;
          event.stopPropagation();
          select();
          event.currentTarget.setPointerCapture(event.pointerId);
          const pointer = position(event);
          gesture.current = {
            target,
            pointer: event.pointerId,
            x: event.clientX,
            y: event.clientY,
            original: { x: point.x - pointer.x, y: point.y - pointer.y },
            moved: false,
          };
        }}
        onPointerMove={(event) => {
          const drag = gesture.current;
          if (!edit || !drag || drag.pointer !== event.pointerId) return;
          if (Math.hypot(event.clientX - drag.x, event.clientY - drag.y) >= 4)
            drag.moved = true;
          if (!drag.moved) return;
          const pointer = position(event);
          setPreview({
            target,
            point: {
              x: Math.max(0, Math.min(1, pointer.x + drag.original.x)),
              y: Math.max(0, Math.min(1, pointer.y + drag.original.y)),
            },
          });
        }}
        onPointerUp={(event) => {
          const drag = gesture.current;
          if (!drag || drag.pointer !== event.pointerId) return;
          if (drag.moved && preview) setPoint(target, preview.point);
          gesture.current = null;
          setPreview(null);
        }}
        onPointerCancel={() => {
          gesture.current = null;
          setPreview(null);
        }}
        onLostPointerCapture={() => {
          gesture.current = null;
          setPreview(null);
        }}
      >
        <circle cx={point.x * 1000} cy={point.y * height} r={portal ? 9 : 7} />
        {portal && (
          <text
            x={point.x * 1000 - 12}
            y={point.y * height - (target === "entry" ? 30 : 15)}
            textAnchor="end"
          >
            {label}
          </text>
        )}
      </g>
    );
  };
  return (
    <g
      className={`cable ${cable.medium} ${selected ? "chosen" : ""}`}
      role="button"
      tabIndex={0}
      aria-label={`${endpoint(plan, cable.a)} to ${endpoint(plan, cable.b)}`}
      onClick={(event) => {
        event.stopPropagation();
        select();
      }}
      onKeyDown={(event) => {
        if (event.key === "Enter") select();
      }}
    >
      <path
        className="cable-hit"
        d={path}
        onDoubleClick={(event) => {
          if (!edit || bends.length >= 500) return;
          event.stopPropagation();
          select();
          const point = position(event);
          // Insert into the nearest drawn segment, retaining A-to-B bend order.
          let closest = 0,
            distance = Infinity;
          for (let i = 0; i < points.length - 1; i++) {
            const start = points[i],
              end = points[i + 1];
            const dx = (end.x - start.x) * 1000,
              dy = (end.y - start.y) * height;
            const px = (point.x - start.x) * 1000,
              py = (point.y - start.y) * height;
            const t = Math.max(
              0,
              Math.min(1, (px * dx + py * dy) / (dx * dx + dy * dy || 1)),
            );
            const nextDistance = Math.hypot(px - t * dx, py - t * dy);
            if (nextDistance < distance) {
              closest = i;
              distance = nextDistance;
            }
          }
          patch({
            points: [
              ...bends.slice(0, closest),
              point,
              ...bends.slice(closest),
            ],
          });
        }}
      />
      <path className="cable-line" d={path} style={{ pointerEvents: "none" }} />
      {power.map((flow) => {
        const ordered = flow.fromA ? points : [...points].reverse();
        const vertices = ordered.flatMap((point, index) =>
          index === 0
            ? [point]
            : [
                {
                  x: (ordered[index - 1].x + point.x) / 2,
                  y: (ordered[index - 1].y + point.y) / 2,
                },
                point,
              ],
        );
        const route = vertices
          .map(
            (point, index) =>
              `${index ? "L" : "M"}${point.x * 1000},${point.y * height}`,
          )
          .join(" ");
        return (
          <g
            className="poe-flow"
            key={`${flow.source.source}/${flow.source.port}`}
            aria-label="Reported PoE-out direction"
          >
            <path d={route} markerMid="url(#poe-arrow)" />
            <text
              x={(first.x + last.x) * 500}
              y={((first.y + last.y) * height) / 2 + 16}
            >
              ⚡ PoE-out{power.length > 1 ? " · multiple reported sources" : ""}
            </text>
          </g>
        );
      })}
      <text
        className="cable-label"
        x={(first.x + last.x) * 500}
        y={((first.y + last.y) * height) / 2 - 8}
        style={{ pointerEvents: "none" }}
      >
        {cable.medium}
        {cable.draft ? " · draft" : ""}
        {!section.confirmed ? " · route undocumented" : ""}
      </text>
      {!anchoredA &&
        handle("entry", first, sections[sectionIndex - 1]?.floor ?? a?.floor)}
      {!anchoredB &&
        handle("exit", last, sections[sectionIndex + 1]?.floor ?? b?.floor)}
      {selected && edit && bends.map((point, i) => handle(i, point))}
    </g>
  );
}
