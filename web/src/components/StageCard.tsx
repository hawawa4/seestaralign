import type { AppState } from '../types';
import { computeSteps } from '../lib/steps';
import { Stepper } from './Stepper';
import './StageCard.css';

/** Progress through the run and what the app is doing right now. */
export function StageCard({ state, awake }: { state: AppState; awake: boolean }) {
  const running = state.phase === 'measuring' || state.phase === 'adjusting';
  const message = state.step || (running ? 'Working…' : state.error ? 'Stopped. Last result shown below.' : 'Stopped.');
  return (
    <div class="card">
      <Stepper steps={computeSteps(state)} />
      <div class="stage-msg">
        {running && <span class="sp" />}
        <span>{message}</span>
        {awake && <span class="awake">screen stays on</span>}
      </div>
      {running && <div class="bar" />}
    </div>
  );
}
