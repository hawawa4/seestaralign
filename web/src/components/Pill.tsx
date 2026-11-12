import './Pill.css';

export type PillState = 'off' | 'on' | 'live' | 'busy';

export function Pill({ state, label, id }: { state: PillState; label: string; id?: string }) {
  return (
    <span class={`pill ${state}`} id={id}>
      <i />
      <span>{label}</span>
    </span>
  );
}
