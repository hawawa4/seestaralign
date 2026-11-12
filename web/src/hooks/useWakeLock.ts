import { useEffect, useState } from 'preact/hooks';

/**
 * Keeps the screen on while `active`. Needs a secure context (https or
 * localhost) and a browser that supports it; otherwise it quietly does nothing.
 * Returns whether the lock is currently held.
 */
export function useWakeLock(active: boolean): boolean {
  const [held, setHeld] = useState(false);
  useEffect(() => {
    if (!active || !('wakeLock' in navigator)) return;
    let lock: WakeLockSentinel | null = null;
    let cancelled = false;
    const acquire = async () => {
      try {
        const l = await navigator.wakeLock.request('screen');
        if (cancelled) return void l.release();
        lock = l;
        setHeld(true);
        l.addEventListener('release', () => setHeld(false));
      } catch {
        /* denied: fine */
      }
    };
    const onVisible = () => {
      if (!document.hidden && !lock?.released) return;
      if (!document.hidden) void acquire();
    };
    void acquire();
    document.addEventListener('visibilitychange', onVisible);
    return () => {
      cancelled = true;
      document.removeEventListener('visibilitychange', onVisible);
      void lock?.release();
      setHeld(false);
    };
  }, [active]);
  return held;
}
