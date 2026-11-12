import { describe, expect, it } from 'vitest';
import { DOME_R, domePoint, domeToAltAz, outerRing } from './geometry';

describe('sky dome mapping', () => {
  it('puts the zenith in the centre and the horizon on the rim', () => {
    const [zx, zy] = domePoint(90, 123);
    expect(zx).toBeCloseTo(0);
    expect(zy).toBeCloseTo(0);
    const [x, y] = domePoint(0, 0);
    expect(x).toBeCloseTo(0);
    expect(y).toBeCloseTo(-DOME_R); // north is up
  });
  it('puts east on the right', () => {
    const [x, y] = domePoint(0, 90);
    expect(x).toBeCloseTo(DOME_R);
    expect(y).toBeCloseTo(0);
  });
  it('round-trips alt/az', () => {
    for (const [alt, az] of [[50, 180], [35, 110], [12.3, 271.4], [80, 5]] as const) {
      const [x, y] = domePoint(alt, az);
      expect(domeToAltAz(x, y)).toEqual({ alt, az });
    }
  });
  it('clamps outside the dome to the horizon', () => {
    expect(domeToAltAz(DOME_R * 3, 0).alt).toBe(0);
  });
  it('never returns an azimuth of 360', () => {
    expect(domeToAltAz(-0.001, -50).az).toBeLessThan(360);
  });
});

describe('outerRing', () => {
  it('zooms to the smallest ring that fits the error', () => {
    expect(outerRing(0)).toBe(1);
    expect(outerRing(4)).toBe(5);
    expect(outerRing(41)).toBe(60);
  });
  it('caps at the biggest ring', () => {
    expect(outerRing(100000)).toBe(600);
  });
});
