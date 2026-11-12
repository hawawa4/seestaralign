/** "27′ 00″"-style angle (always positive; the sign is shown by an arrow). */
export function fmtAngle(deg: number | null | undefined): string {
  if (deg == null || Number.isNaN(deg)) return '–';
  const a = Math.abs(deg);
  const seconds = Math.round(a * 3600);
  if (seconds < 60) return `${seconds}″`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}′ ${String(seconds % 60).padStart(2, '0')}″`;
  const minutes = Math.round(a * 60); // whole arcminutes; rounding carries into the degrees
  return `${Math.floor(minutes / 60)}° ${String(minutes % 60).padStart(2, '0')}′`;
}

export type Grade = 'good' | 'okc' | 'badc';

/** Colour class for an error: under 1′ good, under 5′ ok, else bad. */
export function grade(deg: number): Grade {
  const arcmin = Math.abs(deg) * 60;
  return arcmin < 1 ? 'good' : arcmin < 5 ? 'okc' : 'badc';
}

/** Arcminutes threshold below which an axis counts as "on target". */
export const ON_TARGET_ARCMIN = 0.25;
export const onTarget = (deg: number) => Math.abs(deg) * 60 < ON_TARGET_ARCMIN;

/** One decimal place, no trailing zero. */
export const num1 = (v: number) => String(Math.round(v * 10) / 10);

/** Label for a bullseye ring radius given in arcminutes. */
export function fmtRing(arcmin: number): string {
  if (arcmin >= 60) return `${+(arcmin / 60).toFixed(1)}°`;
  if (arcmin >= 1) return `${+arcmin.toFixed(1)}′`;
  return `${Math.round(arcmin * 60)}″`;
}
