import './Switch.css';

interface Props {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
  hint?: string;
}

export function Switch({ checked, onChange, label, hint }: Props) {
  return (
    <label class="switch">
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.currentTarget.checked)} />
      <span class="tr" />
      <span class="sw-t">
        {label}
        {hint && <small>{hint}</small>}
      </span>
    </label>
  );
}
