import { describe, expect, it } from 'vitest';
import { computeSteps } from './steps';
import type { Phase, Point } from '../types';

const pts = (n: number) => Array.from({ length: n }, (_, i) => ({ n: i + 1 }) as Point);
const make = (phase: Phase, o: { slew?: boolean; slewing?: boolean; points?: number } = {}) =>
  computeSteps({
    phase,
    slewing: o.slewing ?? false,
    points: o.points ? pts(o.points) : null,
    settings: { start_enabled: o.slew ?? false },
  });
const statuses = (s: ReturnType<typeof make>) => s.map((x) => x.status).join(' ');

describe('computeSteps', () => {
  it('omits the slew step when no starting position is set', () => {
    expect(make('ready').map((s) => s.label)).toEqual(['Point 1', 'Point 2', 'Point 3', 'Adjust']);
  });
  it('starts with the slew step when it is enabled', () => {
    expect(make('ready', { slew: true })[0]?.label).toBe('Slew');
  });
  it('shows the slew as the current step while slewing', () => {
    expect(statuses(make('measuring', { slew: true, slewing: true }))).toBe('now todo todo todo todo');
  });
  it('moves on to point 1 once the slew is done', () => {
    expect(statuses(make('measuring', { slew: true }))).toBe('done now todo todo todo');
  });
  it('tracks the measured points', () => {
    expect(statuses(make('measuring', { points: 2 }))).toBe('done done now todo');
    expect(statuses(make('measuring', { points: 3 }))).toBe('done done now todo'); // never past point 3 while measuring
  });
  it('is on Adjust once adjusting', () => {
    expect(statuses(make('adjusting', { slew: true, points: 3 }))).toBe('done done done done now');
  });
  it('keeps the finished steps visible after a stop', () => {
    expect(statuses(make('ready', { points: 3 }))).toBe('done done done todo');
  });
});
