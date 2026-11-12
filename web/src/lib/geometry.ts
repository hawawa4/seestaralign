import { clamp } from './util';

/** Radius of the sky dome in its own SVG units. */
export const DOME_R = 100;

/** Alt/az (degrees) to dome coordinates: zenith in the centre, north up, east right. */
export function domePoint(altDeg: number, azDeg: number, R = DOME_R): [number, number] {
  const r = (R * (90 - clamp(altDeg, 0, 90))) / 90;
  const a = (azDeg * Math.PI) / 180;
  return [r * Math.sin(a), -r * Math.cos(a)];
}

/** Inverse of domePoint, rounded to 0.1°. Points outside the dome clamp to the horizon. */
export function domeToAltAz(x: number, y: number, R = DOME_R): { alt: number; az: number } {
  const alt = clamp(90 * (1 - Math.min(Math.hypot(x, y), R) / R), 0, 90);
  let az = (Math.atan2(x, -y) * 180) / Math.PI;
  if (az < 0) az += 360;
  const round = (v: number) => Math.round(v * 10) / 10;
  return { alt: round(alt), az: round(az) % 360 };
}

/** Bullseye ring radii, in arcminutes. */
export const RINGS = [1, 2, 5, 10, 20, 30, 60, 120, 300, 600];

/** Smallest ring that still contains the error, so the last arcminutes stay visible. */
export function outerRing(errArcmin: number): number {
  const need = Math.max(errArcmin, 0.5) * 1.15;
  return RINGS.find((r) => r >= need) ?? RINGS[RINGS.length - 1]!;
}
