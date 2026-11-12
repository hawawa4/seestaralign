import type { HistoryEntry } from '../types';

const W = 300;
const H = 90;
const PAD = 6;

/** Total error over time, with a dashed line at the 1′ goal. */
export function Sparkline({ history }: { history: HistoryEntry[] | null }) {
  const h = history ?? [];
  if (h.length < 2) {
    return (
      <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" aria-label="Error history">
        <text x="0" y="50">History appears while adjusting</text>
      </svg>
    );
  }
  const max = Math.max(...h.map((p) => p.total)) * 60 || 1;
  const y = (arcmin: number) => H - PAD - (arcmin / max) * (H - 2 * PAD);
  const pts = h.map((p, i) => `${(i / (h.length - 1)) * W},${y(p.total * 60)}`).join(' ');
  return (
    <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" aria-label="Error history">
      <defs>
        <linearGradient id="spark-fill" x1="0" x2="0" y1="0" y2="1">
          <stop offset="0" stop-color="var(--accent)" stop-opacity=".35" />
          <stop offset="1" stop-color="var(--accent)" stop-opacity="0" />
        </linearGradient>
      </defs>
      {max > 1 && <line x1="0" x2={W} y1={y(1)} y2={y(1)} stroke="var(--good)" stroke-dasharray="4 4" vector-effect="non-scaling-stroke" />}
      <polygon points={`0,${H} ${pts} ${W},${H}`} fill="url(#spark-fill)" />
      <polyline points={pts} fill="none" stroke="var(--accent)" stroke-width="2" vector-effect="non-scaling-stroke" stroke-linejoin="round" />
    </svg>
  );
}
