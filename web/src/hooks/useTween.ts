import { useEffect, useRef, useState } from 'preact/hooks';

const reducedMotion = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

/** Eases from the previous value to `target`, so readouts glide instead of jump. */
export function useTween(target: number, ms = 500): number {
  const [value, setValue] = useState(target);
  const current = useRef(target);
  useEffect(() => {
    if (ms === 0 || reducedMotion()) {
      current.current = target;
      setValue(target);
      return;
    }
    const from = current.current;
    const t0 = performance.now();
    let raf = 0;
    const step = (now: number) => {
      const k = Math.min(1, (now - t0) / ms);
      current.current = from + (target - from) * (1 - Math.pow(1 - k, 3));
      setValue(current.current);
      if (k < 1) raf = requestAnimationFrame(step);
    };
    raf = requestAnimationFrame(step);
    return () => cancelAnimationFrame(raf);
  }, [target, ms]);
  return value;
}
