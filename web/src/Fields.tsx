import { cloneElement, useId, type ReactElement } from "react";

export function Field({
  label,
  children,
}: {
  label: string;
  children: ReactElement<{ id?: string }>;
}) {
  const id = useId();
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {cloneElement(children, { id })}
    </div>
  );
}

export function TextField({
  label,
  value,
  onChange,
  disabled = false,
  multiline = false,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  multiline?: boolean;
}) {
  return (
    <Field label={label}>
      {multiline ? (
        <textarea
          value={value}
          disabled={disabled}
          onChange={(event) => onChange(event.target.value)}
          rows={3}
        />
      ) : (
        <input
          value={value}
          disabled={disabled}
          onChange={(event) => onChange(event.target.value)}
        />
      )}
    </Field>
  );
}

export function SelectField({
  label,
  value,
  onChange,
  options,
  disabled = false,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  disabled?: boolean;
}) {
  return (
    <Field label={label}>
      <select
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </Field>
  );
}
