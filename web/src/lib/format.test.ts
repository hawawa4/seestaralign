import { describe, expect, it } from 'vitest';
import { fmtAngle, fmtRing, grade, num1, onTarget } from './format';

describe('fmtAngle', () => {
  it('uses arcseconds below an arcminute', () => {
    expect(fmtAngle(0.004)).toBe('14″');
  });
  it('uses minutes and seconds below a degree', () => {
    expect(fmtAngle(0.45)).toBe('27′ 00″');
    expect(fmtAngle(-0.45)).toBe('27′ 00″'); // sign is shown by an arrow instead
  });
  it('uses degrees and minutes above a degree', () => {
    expect(fmtAngle(1.5333)).toBe('1° 32′');
  });
  it('carries rounding up instead of printing 60', () => {
    expect(fmtAngle(0.99999)).toBe('1° 00′');
    expect(fmtAngle(59.7 / 3600)).toBe('1′ 00″');
    expect(fmtAngle(1 + 59.6 / 60)).toBe('2° 00′');
  });
  it('handles missing values', () => {
    expect(fmtAngle(null)).toBe('–');
    expect(fmtAngle(NaN)).toBe('–');
  });
});

describe('grade / onTarget', () => {
  it('grades by arcminutes', () => {
    expect(grade(0.5 / 60)).toBe('good');
    expect(grade(3 / 60)).toBe('okc');
    expect(grade(10 / 60)).toBe('badc');
  });
  it('treats under 0.25′ as on target', () => {
    expect(onTarget(0.2 / 60)).toBe(true);
    expect(onTarget(-0.3 / 60)).toBe(false);
  });
});

describe('misc formatting', () => {
  it('rounds to one decimal without trailing zeros', () => {
    expect(num1(50)).toBe('50');
    expect(num1(45.04)).toBe('45');
    expect(num1(12.36)).toBe('12.4');
  });
  it('labels rings', () => {
    expect(fmtRing(120)).toBe('2°');
    expect(fmtRing(10 / 3)).toBe('3.3′');
    expect(fmtRing(0.5)).toBe('30″');
  });
});
