import { Icon, type IconName } from './Icon';
import './TabBar.css';

export const TABS = ['align', 'connect', 'settings', 'log'] as const;
export type Tab = (typeof TABS)[number];
export const isTab = (s: string): s is Tab => (TABS as readonly string[]).includes(s);

const META: Record<Tab, { label: string; icon: IconName }> = {
  align: { label: 'Align', icon: 'align' },
  connect: { label: 'Connect', icon: 'connect' },
  settings: { label: 'Settings', icon: 'settings' },
  log: { label: 'Log', icon: 'log' },
};

interface Props {
  tab: Tab;
  onSelect: (t: Tab) => void;
  /** Unseen warnings/errors, shown on the Log tab. */
  logBadge: number;
}

export function TabBar({ tab, onSelect, logBadge }: Props) {
  return (
    <nav class="tabs" aria-label="Sections">
      {TABS.map((t) => (
        <button key={t} aria-selected={t === tab} onClick={() => onSelect(t)}>
          <Icon name={META[t].icon} />
          {META[t].label}
          {t === 'log' && logBadge > 0 && <span class="dot">{logBadge > 9 ? '9+' : logBadge}</span>}
        </button>
      ))}
    </nav>
  );
}
