import './Segmented.css';

interface Props<T extends string> {
  options: readonly { value: T; label: string }[];
  value: T;
  onChange: (v: T) => void;
  /** Stretch to the full width of the container. */
  block?: boolean;
}

export function Segmented<T extends string>({ options, value, onChange, block }: Props<T>) {
  return (
    <div class={`seg${block ? ' block' : ''}`} role="group">
      {options.map((o) => (
        <button type="button" key={o.value} class={o.value === value ? 'on' : ''} aria-pressed={o.value === value} onClick={() => onChange(o.value)}>
          {o.label}
        </button>
      ))}
    </div>
  );
}
