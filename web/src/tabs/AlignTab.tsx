import { api } from '../api';
import { Preflight } from '../components/Preflight';
import { ResultCard } from '../components/ResultCard';
import { StageCard } from '../components/StageCard';
import { FrameCard } from '../components/FrameCard';
import type { Tab } from '../components/TabBar';
import { Warnings } from '../components/Warnings';
import { useToast } from '../hooks/useToast';
import type { AppState } from '../types';
import './AlignTab.css';

export function AlignTab({ state, awake, onNavigate }: { state: AppState; awake: boolean; onNavigate: (t: Tab) => void }) {
  const toast = useToast();
  const { phase, error, frame } = state;
  const running = phase === 'measuring' || phase === 'adjusting';
  const connected = phase !== 'disconnected' && phase !== 'connecting';
  const nPoints = state.points?.length ?? 0;

  const run = async (fn: () => Promise<unknown>) => {
    try {
      await fn();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    }
  };
  const start = () => (connected ? run(api.start) : onNavigate('connect'));

  return (
    <div class="tab align">
      <div>
        <Warnings items={state.warnings} />
        {(running || nPoints > 0 || error) && <StageCard state={state} awake={awake} />}
        {error && <ResultCard state={state} error={error} />}
        {!running && <Preflight state={state} onNavigate={onNavigate} />}
      </div>
      <div>{frame && <FrameCard frame={frame} />}</div>
      <div class="actions">
        {running ? (
          <button class="btn big ghost" onClick={() => run(api.stop)}>Stop</button>
        ) : (
          <button class="btn big" disabled={phase === 'connecting'} onClick={start}>
            {!connected ? 'Connect to your Seestar' : error ? 'Restart alignment' : 'Start alignment'}
          </button>
        )}
      </div>
    </div>
  );
}
