import type { HistoryEntry, Phase, PolarError } from '../types';
import { fmtRing } from '../lib/format';
import { outerRing } from '../lib/geometry';
import './Bullseye.css';

const C = 110; // centre
const R = 96; // outer ring radius

interface Props {
  error: PolarError;
  initial: PolarError | null;
  history: HistoryEntry[] | null;
  phase: Phase;
}

/**
 * Where the mount axis is relative to the pole, as seen facing the pole. It
 * zooms to the current error so the last arcminutes stay readable.
 */
export function Bullseye({ error, initial, history, phase }: Props) {
  const outer = outerRing(Math.hypot(error.alt_deg, error.az_deg) * 60);
  const scale = R / outer;
  const eastSign = error.northern ? 1 : -1; // facing the pole: north -> east on the right, south -> on the left

  const pos = (altDeg: number, azDeg: number): [number, number] => {
    let x = eastSign * azDeg * 60 * scale;
    let y = -altDeg * 60 * scale;
    const r = Math.hypot(x, y);
    const lim = R + 6;
    if (r > lim) {
      x *= lim / r; // pin off-scale points to the edge
      y *= lim / r;
    }
    return [C + x, C + y];
  };

  const [dx, dy] = pos(error.alt_deg, error.az_deg);
  const trail = (history ?? []).slice(-14).map((h) => pos(h.alt, h.az));
  const start = initial && phase === 'adjusting' ? pos(initial.alt_deg, initial.az_deg) : null;
  const east = error.northern ? { e: C + R - 9, w: 8 } : { e: 8, w: C + R - 9 };

  return (
    <svg class="bull" viewBox="0 0 220 220" role="img" aria-label="Mount axis relative to the pole">
      <circle cx={C} cy={C} r={R} fill="rgba(0,0,0,.3)" stroke="var(--line-2)" />
      {[2 / 3, 1 / 3].map((f) => (
        <circle key={f} cx={C} cy={C} r={R * f} fill="none" stroke="var(--line)" stroke-dasharray="2 3" />
      ))}
      <line x1={C - R} y1={C} x2={C + R} y2={C} stroke="var(--line)" />
      <line x1={C} y1={C - R} x2={C} y2={C + R} stroke="var(--line)" />
      <circle cx={C} cy={C} r={Math.max(scale, 3)} fill="color-mix(in srgb, var(--good) 14%, transparent)" stroke="var(--good)" stroke-width="1.5" />
      <text x={C + 4} y={C - R + 11}>{fmtRing(outer)}</text>
      <text x={C + 4} y={C - R / 3 + 11}>{fmtRing(outer / 3)}</text>
      <text x={east.e} y={C - 5}>E</text>
      <text x={east.w} y={C - 5}>W</text>
      <text x={C - 6} y={12}>up</text>

      {start && <circle cx={start[0]} cy={start[1]} r="4.5" fill="none" stroke="var(--faint)" stroke-dasharray="2 2" />}
      {trail.length > 1 && (
        <g>
          <polyline fill="none" stroke="var(--dim)" stroke-width="1.2" opacity=".6" points={trail.map(([x, y]) => `${x.toFixed(1)},${y.toFixed(1)}`).join(' ')} />
          {trail.slice(0, -1).map(([x, y], i) => (
            <circle key={i} cx={x.toFixed(1)} cy={y.toFixed(1)} r="2.2" fill="var(--dim)" opacity={((i + 1) / trail.length) * 0.7} />
          ))}
        </g>
      )}
      <g class="bull-dot" style={{ transform: `translate(${dx.toFixed(1)}px, ${dy.toFixed(1)}px)` }}>
        <circle class="pulse" r="6.5" fill="var(--accent)" />
        <circle r="6.5" fill="var(--accent)" stroke="var(--bg)" stroke-width="1.5" />
      </g>
    </svg>
  );
}
