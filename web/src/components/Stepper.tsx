import type { Step } from '../lib/steps';
import './Stepper.css';

export function Stepper({ steps }: { steps: Step[] }) {
  return (
    <div class="stepper">
      {steps.map((s, i) => (
        <div class={`st ${s.status === 'todo' ? '' : s.status}`} key={s.label}>
          <b>{s.status === 'done' ? '✓' : i + 1}</b>
          {s.label}
        </div>
      ))}
    </div>
  );
}
