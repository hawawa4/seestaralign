import { DOME_R, domePoint, domeToAltAz } from '../lib/geometry';
import './SkyDome.css';

interface Props {
  altDeg: number;
  azDeg: number;
  /** Greyed out when the starting position is switched off. */
  enabled: boolean;
  /** Site latitude; draws the celestial pole when known. */
  latitude: number | null;
  onPick: (altDeg: number, azDeg: number) => void;
}

const LABELS = [
  { t: 'N', x: 0, y: -DOME_R - 6 },
  { t: 'E', x: DOME_R + 8, y: 3 },
  { t: 'S', x: 0, y: DOME_R + 14 },
  { t: 'W', x: -DOME_R - 8, y: 3 },
];

/** Top-down sky: zenith in the centre, north up. Drag to choose where to point. */
export function SkyDome({ altDeg, azDeg, enabled, latitude, onPick }: Props) {
  const pick = (e: PointerEvent) => {
    const svg = e.currentTarget as SVGSVGElement;
    const r = svg.getBoundingClientRect();
    const x = ((e.clientX - r.left) / r.width) * 240 - 120;
    const y = ((e.clientY - r.top) / r.height) * 240 - 120;
    const { alt, az } = domeToAltAz(x, y);
    onPick(alt, az);
  };
  const [mx, my] = domePoint(altDeg, azDeg);
  const pole = latitude !== null && Math.abs(latitude) > 0.5 ? domePoint(Math.abs(latitude), latitude >= 0 ? 0 : 180) : null;

  return (
    <svg
      class={`dome${enabled ? '' : ' off'}`}
      viewBox="-120 -120 240 240"
      aria-label="Pick a starting position on the sky"
      onPointerDown={(e) => {
        (e.currentTarget as SVGSVGElement).setPointerCapture(e.pointerId);
        pick(e);
      }}
      onPointerMove={(e) => (e.currentTarget as SVGSVGElement).hasPointerCapture(e.pointerId) && pick(e)}
    >
      <circle r={DOME_R} fill="rgba(0,0,0,.35)" stroke="var(--line-2)" />
      <circle r={DOME_R * 0.5} fill="none" stroke="var(--accent)" stroke-width={DOME_R / 3} opacity=".1" />
      {[30, 60].map((a) => (
        <circle key={a} r={(DOME_R * (90 - a)) / 90} fill="none" stroke="var(--line)" stroke-dasharray="2 3" />
      ))}
      <path d={`M0 ${-DOME_R}V${DOME_R}M${-DOME_R} 0H${DOME_R}`} stroke="var(--line)" fill="none" />
      {LABELS.map((l) => (
        <text key={l.t} x={l.x} y={l.y} text-anchor="middle">{l.t}</text>
      ))}
      {[30, 60].map((a) => (
        <text key={a} class="ring-label" x={(DOME_R * (90 - a)) / 90 + 3} y="-3">{a}°</text>
      ))}
      {pole && (
        <g transform={`translate(${pole[0].toFixed(1)} ${pole[1].toFixed(1)})`}>
          <path d="M0-7l2 5L7 0l-5 2L0 7l-2-5L-7 0l5-2z" fill="var(--good)" />
          <text class="pole-label" x="9" y="3">pole</text>
        </g>
      )}
      <g transform={`translate(${mx.toFixed(1)} ${my.toFixed(1)})`}>
        <circle class="pulse" r="7" fill="var(--accent)" />
        <circle r="7" fill="var(--accent)" stroke="var(--bg)" stroke-width="1.5" />
      </g>
    </svg>
  );
}
