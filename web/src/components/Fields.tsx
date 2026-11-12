import type { ComponentChildren } from 'preact';
import { useEffect, useId, useRef, useState } from 'preact/hooks';
import './Fields.css';

export function Fields({ children, columns }: { children: ComponentChildren; columns?: number }) {
  return (
    <div class="fields" style={columns ? { gridTemplateColumns: `repeat(${columns}, 1fr)` } : undefined}>
      {children}
    </div>
  );
}

interface FieldShellProps {
  label: string;
  hint?: string;
  full?: boolean;
  htmlFor: string;
  children: ComponentChildren;
}
function FieldShell({ label, hint, full, htmlFor, children }: FieldShellProps) {
  return (
    <div class={full ? 'full' : undefined}>
      <label for={htmlFor}>{label}</label>
      {children}
      {hint && <div class="hint">{hint}</div>}
    </div>
  );
}

interface TextProps {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  hint?: string;
  full?: boolean;
  disabled?: boolean;
}
export function TextField({ label, value, onChange, placeholder, hint, full, disabled }: TextProps) {
  const id = useId();
  return (
    <FieldShell label={label} hint={hint} full={full} htmlFor={id}>
      <input id={id} value={value} placeholder={placeholder} disabled={disabled} spellcheck={false} autocomplete="off" onInput={(e) => onChange(e.currentTarget.value)} />
    </FieldShell>
  );
}

interface NumberProps {
  label: string;
  value: number | null;
  /** Called with a parsed number; `null` only if `nullable` and the box was emptied. */
  onChange: (v: number | null) => void;
  unit?: string;
  placeholder?: string;
  hint?: string;
  full?: boolean;
  /** Empty box means null. */
  nullable?: boolean;
  /** Empty box means 0, and 0 is shown as empty. */
  zeroEmpty?: boolean;
  integer?: boolean;
  disabled?: boolean;
}

/**
 * A number box that keeps what you typed ("1." stays "1.") while still sending
 * every valid number up as you go. It shows the saved value again once unfocused.
 */
export function NumberField({ label, value, onChange, unit, placeholder, hint, full, nullable, zeroEmpty, integer, disabled }: NumberProps) {
  const id = useId();
  const show = (v: number | null) => (v == null || (zeroEmpty && v === 0) ? '' : String(v));
  const [text, setText] = useState(show(value));
  const focused = useRef(false);
  useEffect(() => {
    if (!focused.current) setText(show(value));
  }, [value]); // eslint-disable-line react-hooks/exhaustive-deps

  const input = (raw: string) => {
    setText(raw);
    const t = raw.trim();
    if (t === '') {
      if (nullable) onChange(null);
      else if (zeroEmpty) onChange(0);
      return;
    }
    const n = integer ? parseInt(t, 10) : Number(t);
    if (Number.isFinite(n)) onChange(n);
  };

  return (
    <FieldShell label={label} hint={hint} full={full} htmlFor={id}>
      <div class={unit ? 'inp' : undefined}>
        <input
          id={id}
          value={text}
          inputMode={integer ? 'numeric' : 'decimal'}
          placeholder={placeholder}
          disabled={disabled}
          autocomplete="off"
          onFocus={() => (focused.current = true)}
          onBlur={() => {
            focused.current = false;
            setText(show(value));
          }}
          onInput={(e) => input(e.currentTarget.value)}
        />
        {unit && <span>{unit}</span>}
      </div>
    </FieldShell>
  );
}

interface SelectProps {
  label: string;
  value: string;
  options: readonly { value: string; label: string }[];
  onChange: (v: string) => void;
  hint?: string;
  full?: boolean;
}
export function SelectField({ label, value, options, onChange, hint, full }: SelectProps) {
  const id = useId();
  return (
    <FieldShell label={label} hint={hint} full={full} htmlFor={id}>
      <select id={id} value={value} onChange={(e) => onChange(e.currentTarget.value)}>
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </FieldShell>
  );
}
