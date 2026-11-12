import { fmtAngle } from '../lib/format';
import { useTween } from '../hooks/useTween';

/** An angle that glides to each new value. */
export function AnimatedAngle({ deg, class: cls, id }: { deg: number; class?: string; id?: string }) {
  const v = useTween(deg);
  return (
    <div class={cls} id={id}>
      {fmtAngle(v)}
    </div>
  );
}
