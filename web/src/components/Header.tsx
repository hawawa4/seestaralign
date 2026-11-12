import type { Phase } from '../types';
import { useTheme } from '../hooks/useTheme';
import { Logo } from './Icon';
import { Pill, type PillState } from './Pill';
import './Header.css';

const LABELS: Record<Phase, string> = {
  disconnected: 'offline',
  connecting: 'connecting…',
  ready: 'connected',
  measuring: 'measuring',
  adjusting: 'adjusting',
};

export function pillState(phase: Phase): PillState {
  if (phase === 'measuring' || phase === 'adjusting') return 'live';
  if (phase === 'ready') return 'on';
  if (phase === 'connecting') return 'busy';
  return 'off';
}

export function Header({ phase, online }: { phase: Phase | null; online: boolean }) {
  const { theme, setTheme } = useTheme();
  return (
    <header>
      <div class="wrap">
        <Logo />
        <h1>Seestar Polar Align</h1>
        {!online ? <Pill state="off" label="server offline" /> : phase && <Pill state={pillState(phase)} label={LABELS[phase]} />}
        <button class="iconbtn" title="Switch between night (red) and dark theme" aria-label="Toggle theme" onClick={() => setTheme(theme === 'dark' ? 'night' : 'dark')}>
          {theme === 'dark' ? '☀' : '☾'}
        </button>
      </div>
    </header>
  );
}

export { LABELS as PHASE_LABELS };
