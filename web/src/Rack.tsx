import { UnitField } from "./Fields";
import type { Cabinet, Device } from "./model";

function validUnit(value: number | null | undefined): value is number {
  return (
    value != null && Number.isInteger(value) && value >= 1 && value <= 1000
  );
}

export function rackLabel(device: Device): string {
  if (!validUnit(device.rackUnit)) {
    return validUnit(device.heightU)
      ? `${device.heightU} U · unit unassigned`
      : "Unit unassigned";
  }
  if (!validUnit(device.heightU)) return `U${device.rackUnit} · height unknown`;
  const last = device.rackUnit + device.heightU - 1;
  return `${last === device.rackUnit ? `U${last}` : `U${device.rackUnit}–U${last}`} · ${device.heightU} U`;
}

export function RackMountFields({
  device,
  disabled,
  patch,
}: {
  device: Device;
  disabled: boolean;
  patch: (update: Partial<Device>) => void;
}) {
  return (
    <>
      <UnitField
        label="Equipment height (U)"
        value={device.heightU}
        disabled={disabled}
        onChange={(heightU) => patch({ heightU })}
      />
      <UnitField
        label="Starting unit (bottom of equipment)"
        value={device.rackUnit}
        disabled={disabled}
        placeholder="Unassigned"
        onChange={(rackUnit) => patch({ rackUnit })}
      />
      <p className="help">
        U1 is at the bottom of the cabinet. A 2 U device starting at U3 occupies
        U3–U4. Leave dimensions or position blank until known.
      </p>
    </>
  );
}

export function RackElevation({
  cabinet,
  members,
  select,
}: {
  cabinet: Cabinet;
  members: Device[];
  select: (id: string) => void;
}) {
  const height = cabinet.heightU;
  if (!validUnit(height))
    return (
      <p className="help">
        Set the cabinet height to show its rack layout. Equipment can remain
        unassigned.
      </p>
    );

  const mounted = members.filter(
    (device) =>
      validUnit(device.rackUnit) &&
      validUnit(device.heightU) &&
      device.rackUnit + device.heightU - 1 <= height,
  );
  return (
    <div className="rack-scroll" aria-label={`${cabinet.name} rack layout`}>
      <div
        className="rack-elevation"
        style={{ gridTemplateRows: `repeat(${height}, 30px)` }}
      >
        {Array.from({ length: height }, (_, index) => (
          <div className="rack-row" key={index} style={{ gridRow: index + 1 }}>
            <span>U{height - index}</span>
            <span className="rack-empty" />
          </div>
        ))}
        {mounted.map((device) => (
          <button
            key={device.id}
            className="rack-device"
            style={{
              gridColumn: 2,
              gridRow: `${height - device.rackUnit! - device.heightU! + 2} / span ${device.heightU}`,
            }}
            title={`${device.name} · ${rackLabel(device)}`}
            onClick={() => select(device.id)}
          >
            <strong>{device.name}</strong>
            <small>{rackLabel(device)}</small>
          </button>
        ))}
      </div>
    </div>
  );
}
