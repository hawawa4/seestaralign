import type { AppState } from '../types';

export interface Step {
  label: string;
  status: 'todo' | 'now' | 'done';
}

/**
 * The progress steps shown while aligning: an optional slew to the starting
 * position, three measuring points, then the live adjustment.
 */
export function computeSteps(
  st: Pick<AppState, 'phase' | 'slewing' | 'points'> & { settings: Pick<AppState['settings'], 'start_enabled'> },
): Step[] {
  const slew = st.settings.start_enabled;
  const off = slew ? 1 : 0;
  const nPts = st.points?.length ?? 0;
  const running = st.phase === 'measuring' || st.phase === 'adjusting';

  let now = -1;
  if (st.phase === 'adjusting') now = off + 3;
  else if (st.phase === 'measuring') now = slew && st.slewing ? 0 : off + Math.min(nPts, 2);
  const progress = running ? now : nPts > 0 ? off + nPts : 0;

  const labels = [...(slew ? ['Slew'] : []), 'Point 1', 'Point 2', 'Point 3', 'Adjust'];
  return labels.map((label, i) => ({
    label,
    status: running && i === now ? 'now' : i < progress ? 'done' : 'todo',
  }));
}
