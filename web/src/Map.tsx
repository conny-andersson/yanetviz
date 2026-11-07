import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { endpoint, location, roles, type Plan, type Selection } from "./model";

type Props = {
  plan: Plan;
  floor: string;
  edit: boolean;
  selection: Selection;
  select: (s: Selection) => void;
  move: (kind: "device" | "cabinet", id: string, x: number, y: number) => void;
  navigate: (id: string) => void;
  showCables: boolean;
  zoom: number;
};

type DragGesture = {
  pointerId: number;
  kind: "device" | "cabinet";
  id: string;
  clientX: number;
  clientY: number;
  pointerX: number;
  pointerY: number;
  originalX: number;
  originalY: number;
  dragging: boolean;
};

const dragThreshold = 4; // Screen pixels, independent of canvas zoom.
const clampPosition = (value: number) =>
  Math.max(0.025, Math.min(0.975, value));

export function Map({
  plan: p,
  floor,
  edit,
  selection,
  select,
  move,
  navigate,
  showCables,
  zoom,
}: Props) {
  const viewport = useRef<HTMLDivElement>(null);
  const svg = useRef<SVGSVGElement>(null);
  const gesture = useRef<DragGesture | null>(null);
  const [available, setAvailable] = useState({ width: 0, height: 0 });
  const [height, setHeight] = useState(800);
  const [drag, setDrag] = useState<{
    kind: "device" | "cabinet";
    id: string;
    x: number;
    y: number;
  } | null>(null);
  const bg = p.floors.find((f) => f.id === floor)?.background;

  useLayoutEffect(() => {
    const element = viewport.current;
    if (!element) return;

    // Observe the canvas content box, excluding its padding and toolbars.
    // This also refits the plan when the window or editor layout changes.
    const observer = new ResizeObserver(([entry]) => {
      setAvailable({
        width: entry.contentRect.width,
        height: entry.contentRect.height,
      });
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    let active = true;
    setHeight(800);
    if (bg) {
      const img = new Image();
      img.onload = () => {
        if (active && img.naturalWidth)
          setHeight((1000 * img.naturalHeight) / img.naturalWidth);
      };
      img.src = bg;
    }
    return () => {
      active = false;
    };
  }, [bg]);

  // Fit both dimensions while retaining the background's original aspect ratio.
  // Zoom is relative to that fitted size; the border occupies two extra pixels.
  const fitScale = Math.max(
    0,
    Math.min((available.width - 2) / 1000, (available.height - 2) / height),
  );
  const canvasWidth = 1000 * fitScale * zoom + 2;
  const canvasHeight = height * fitScale * zoom + 2;

  const coord = (e: React.PointerEvent) => {
    const matrix = svg.current?.getScreenCTM();
    if (!matrix) return { x: 0.5, y: 0.5 };
    const pt = new DOMPoint(e.clientX, e.clientY).matrixTransform(
      matrix.inverse(),
    );
    return {
      x: pt.x / 1000,
      y: pt.y / height,
    };
  };
  const start = (
    e: React.PointerEvent<SVGGElement>,
    kind: "device" | "cabinet",
    id: string,
    x: number,
    y: number,
  ) => {
    if (e.button !== 0 || !e.isPrimary) return;
    e.stopPropagation();
    select({ kind, id });
    if (edit) {
      e.currentTarget.setPointerCapture(e.pointerId);
      const pointer = coord(e);
      gesture.current = {
        pointerId: e.pointerId,
        kind,
        id,
        clientX: e.clientX,
        clientY: e.clientY,
        pointerX: pointer.x,
        pointerY: pointer.y,
        originalX: x,
        originalY: y,
        dragging: false,
      };
    }
  };

  const dragPosition = (e: React.PointerEvent<SVGGElement>) => {
    const current = gesture.current;
    if (!edit || !current || current.pointerId !== e.pointerId) return null;
    if (!current.dragging) {
      const distance = Math.hypot(
        e.clientX - current.clientX,
        e.clientY - current.clientY,
      );
      if (distance < dragThreshold) return null;
      current.dragging = true;
    }

    // Move by the pointer delta, retaining where the marker was grabbed. Clicking
    // an edge or label must not snap the marker's center underneath the pointer.
    const pointer = coord(e);
    return {
      kind: current.kind,
      id: current.id,
      x: clampPosition(current.originalX + pointer.x - current.pointerX),
      y: clampPosition(current.originalY + pointer.y - current.pointerY),
    };
  };

  const cancelDrag = (e: React.PointerEvent<SVGGElement>) => {
    if (gesture.current?.pointerId !== e.pointerId) return;
    gesture.current = null;
    setDrag(null);
  };

  const finishDrag = (e: React.PointerEvent<SVGGElement>) => {
    const current = gesture.current;
    if (!current || current.pointerId !== e.pointerId) return;
    const position = dragPosition(e);
    cancelDrag(e);
    if (
      position &&
      (Math.abs(position.x - current.originalX) > 0.000001 ||
        Math.abs(position.y - current.originalY) > 0.000001)
    ) {
      move(position.kind, position.id, position.x, position.y);
    }
  };
  const point = (kind: string, id: string, x: number, y: number) =>
    drag?.kind === kind && drag.id === id ? drag : { x, y };
  const devicePoint = (id: string) => {
    const d = p.devices.find((d) => d.id === id);
    if (!d) return null;
    const loc = location(p, d);
    if (!loc) return null;
    const v = point(
      d.cabinet ? "cabinet" : "device",
      d.cabinet || id,
      loc.x,
      loc.y,
    );
    return { ...loc, ...v };
  };
  const marker = (
    kind: "device" | "cabinet",
    id: string,
    name: string,
    subtitle: string,
    x: number,
    y: number,
    icon: string,
  ) => {
    const pt = point(kind, id, x, y);
    const selected = selection?.kind === kind && selection.id === id;
    return (
      <g
        key={id}
        className={`marker ${selected ? "selected" : ""} ${edit ? "draggable" : ""}`}
        transform={`translate(${pt.x * 1000} ${pt.y * height})`}
        tabIndex={0}
        role="button"
        aria-label={name}
        onKeyDown={(e) => {
          if (e.key === "Enter") select({ kind, id });
        }}
        onPointerDown={(e) => start(e, kind, id, x, y)}
        onPointerMove={(e) => {
          const position = dragPosition(e);
          if (position) setDrag(position);
        }}
        onPointerUp={finishDrag}
        onPointerCancel={cancelDrag}
        onLostPointerCapture={cancelDrag}
      >
        <rect x="-20" y="-20" width="40" height="40" rx="10" />
        <text className="marker-icon" textAnchor="middle" y="6">
          {icon}
        </text>
        <text className="marker-name" y="37" textAnchor="middle">
          {name}
        </text>
        <text className="marker-subtitle" y="51" textAnchor="middle">
          {subtitle}
        </text>
      </g>
    );
  };
  return (
    <div className="map-scroll" ref={viewport}>
      <div
        className="map-stage"
        style={{
          width: Math.max(available.width, canvasWidth),
          height: Math.max(available.height, canvasHeight),
        }}
      >
        <svg
          ref={svg}
          className="floor-map"
          viewBox={`0 0 1000 ${height}`}
          style={{ width: canvasWidth, height: canvasHeight }}
          aria-label="Network floor plan"
          onClick={(e) => {
            if (e.target === svg.current) select(null);
          }}
        >
          <defs>
            <pattern
              id="grid"
              width="25"
              height="25"
              patternUnits="userSpaceOnUse"
            >
              <circle cx="1" cy="1" r=".7" fill="#c2c9cb" />
            </pattern>
          </defs>
          <rect width="1000" height={height} fill="url(#grid)" />
          {bg && (
            <image
              href={bg}
              width="1000"
              height={height}
              preserveAspectRatio="none"
              className="floor-background"
            />
          )}
          {showCables &&
            p.cables.map((c, i) => {
              const a = devicePoint(c.a.device),
                b = devicePoint(c.b.device);
              if (a?.floor !== floor && b?.floor !== floor) return null;
              const local = a?.floor === floor ? a : b!;
              const remote = a?.floor === floor ? b : a;
              const both = remote?.floor === floor;
              const x1 = local.x * 1000,
                y1 = local.y * height;
              const x2 = both ? remote!.x * 1000 : 930,
                y2 = both
                  ? remote!.y * height
                  : Math.max(
                      30,
                      Math.min(height - 30, y1 + ((i % 3) - 1) * 65),
                    );
              const selected =
                selection?.kind === "cable" && selection.id === c.id;
              const remoteEndpoint = a?.floor === floor ? c.b : c.a;
              return (
                <g
                  key={c.id}
                  className={`cable ${c.medium} ${selected ? "chosen" : ""}`}
                  role="button"
                  tabIndex={0}
                  aria-label={`${endpoint(p, c.a)} to ${endpoint(p, c.b)}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    select({ kind: "cable", id: c.id });
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") select({ kind: "cable", id: c.id });
                  }}
                >
                  <path className="cable-hit" d={`M${x1},${y1} L${x2},${y2}`} />
                  <path
                    className="cable-line"
                    d={`M${x1},${y1} L${x2},${y2}`}
                  />
                  <text
                    className="cable-label"
                    x={(x1 + x2) / 2}
                    y={(y1 + y2) / 2 - 8}
                  >
                    {c.medium}
                    {c.draft ? " · draft" : ""}
                  </text>
                  {!both && (
                    <g
                      className="portal"
                      onClick={(e) => {
                        e.stopPropagation();
                        if (remote) navigate(remote.floor);
                        select({ kind: "device", id: remoteEndpoint.device });
                      }}
                    >
                      <circle cx={x2} cy={y2} r="9" />
                      <text x={x2 - 12} y={y2 - 15} textAnchor="end">
                        To{" "}
                        {remote
                          ? p.floors.find((f) => f.id === remote.floor)?.name
                          : "unplaced"}{" "}
                        /{" "}
                        {
                          p.devices.find((d) => d.id === remoteEndpoint.device)
                            ?.name
                        }
                      </text>
                    </g>
                  )}
                </g>
              );
            })}
          {p.cabinets
            .filter((c) => c.floor === floor && c.placed)
            .map((c) =>
              marker(
                "cabinet",
                c.id,
                c.name,
                `${p.devices.filter((d) => d.cabinet === c.id).length} devices`,
                c.x,
                c.y,
                "▤",
              ),
            )}
          {p.devices
            .filter((d) => !d.cabinet && location(p, d)?.floor === floor)
            .map((d) =>
              marker(
                "device",
                d.id,
                d.name,
                roles[d.role],
                d.x,
                d.y,
                d.role === "access_point"
                  ? "◉"
                  : d.role === "client"
                    ? "▣"
                    : "⇄",
              ),
            )}
        </svg>
      </div>
    </div>
  );
}
