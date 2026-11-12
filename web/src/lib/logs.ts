import type { LogEntry } from '../types';

export const isProblem = (l: LogEntry) => l.level === 'warn' || l.level === 'error';

/** Warnings and errors newer than the last time the Log tab was open. */
export function countUnseen(log: LogEntry[] | null, seen: string): number {
  return (log ?? []).filter((l) => isProblem(l) && l.t > seen).length;
}

export function logAsText(log: LogEntry[] | null): string {
  return (log ?? []).map((l) => `${new Date(l.t).toLocaleTimeString()} [${l.level}] ${l.msg}`).join('\n');
}
