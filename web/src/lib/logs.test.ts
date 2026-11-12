import { describe, expect, it } from 'vitest';
import { countUnseen } from './logs';
import type { LogEntry } from '../types';

const e = (t: string, level: LogEntry['level']): LogEntry => ({ t, level, msg: 'x' });

describe('countUnseen', () => {
  const log = [e('2026-01-01T10:00:00Z', 'warn'), e('2026-01-01T10:01:00Z', 'info'), e('2026-01-01T10:02:00Z', 'error')];
  it('counts only warnings and errors newer than the last view', () => {
    expect(countUnseen(log, '')).toBe(2);
    expect(countUnseen(log, '2026-01-01T10:00:00Z')).toBe(1);
    expect(countUnseen(log, '2026-01-01T10:02:00Z')).toBe(0);
  });
  it('copes with no log', () => {
    expect(countUnseen(null, '')).toBe(0);
  });
});
