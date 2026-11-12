import type { AppState, PolarError } from '../types';
import { fmtAngle, grade, onTarget } from '../lib/format';
import { AnimatedAngle } from './AnimatedAngle';
import { Bullseye } from './Bullseye';
import { Sparkline } from './Sparkline';
import { Warnings } from './Warnings';
import './ResultCard.css';

function AxisCard({ title, deg, glyph, motion, instruction }: { title: string; deg: number; glyph: string; motion: string; instruction: string }) {
  const ok = onTarget(deg);
  return (
    <div class="axis">
      <div class="t">{title}</div>
      <div class={`arrow ${ok ? 'ok' : `go ${motion}`}`}>{ok ? '✓' : glyph}</div>
      <AnimatedAngle deg={deg} class={`v ${grade(deg)}`} />
      <div class="d">{ok ? 'on target' : instruction}</div>
    </div>
  );
}

/** The live answer: total error, which way to turn each knob, and how it's trending. */
export function ResultCard({ state, error: e }: { state: AppState; error: PolarError }) {
  const aligned = Math.abs(e.total_deg) * 60 < 1;
  const a = state.adjust;
  const init = state.initial;
  return (
    <div class={`card${aligned ? ' aligned' : ''}`}>
      <div class="hero">
        <AnimatedAngle deg={e.total_deg} class={`num ${grade(e.total_deg)}`} />
        <div class="lbl">{state.phase === 'adjusting' ? 'total polar error · live' : 'total polar error'}</div>
        {aligned && <div class="banner">Within 1′ – nicely aligned</div>}
      </div>
      <div class="axes">
        <AxisCard
          title="Altitude"
          deg={e.alt_deg}
          glyph={e.alt_direction === 'raise' ? '▲' : '▼'}
          motion={e.alt_direction === 'raise' ? 'up' : 'down'}
          instruction={e.alt_direction === 'raise' ? 'Raise the axis' : 'Lower the axis'}
        />
        <AxisCard
          title="Azimuth"
          deg={e.az_deg}
          glyph={e.az_hand === 'left' ? '◀' : '▶'}
          motion={e.az_hand}
          instruction={`Move ${e.az_direction} (${e.az_hand})`}
        />
      </div>
      <div class="viz">
        <Bullseye error={e} initial={init} history={state.history} phase={state.phase} />
        <div>
          <div class="eyebrow">Error over time</div>
          <div class="mt-6">
            <Sparkline history={state.history} />
          </div>
          <div class="kvs">
            {init && <span>{`Initial ${fmtAngle(init.total_deg)} (alt ${fmtAngle(init.alt_deg)}, az ${fmtAngle(init.az_deg)})`}</span>}
            {a && a.frames > 0 && <span>{`${a.frames} frame${a.frames === 1 ? '' : 's'} · turned alt ${fmtAngle(a.beta_deg)}, az ${fmtAngle(a.alpha_deg)}`}</span>}
          </div>
        </div>
      </div>
      {a?.warnings && a.warnings.length > 0 && (
        <div class="mt-12">
          <Warnings items={a.warnings} />
        </div>
      )}
    </div>
  );
}
