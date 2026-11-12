import { Accordion } from '../../components/Accordion';
import { Segmented } from '../../components/Segmented';
import { useTheme, type Theme } from '../../hooks/useTheme';

const THEMES = [
  { value: 'night', label: 'Night (red)' },
  { value: 'dark', label: 'Dark' },
] as const;

/** Browser-local appearance; not sent to the server. */
export function DisplaySection() {
  const { theme, setTheme, dim, setDim } = useTheme();
  return (
    <Accordion icon="moon" title="Display" summary={`${theme === 'dark' ? 'Dark' : 'Night'} · ${dim}% dimmed`} defaultOpen>
      <label>Theme</label>
      <Segmented block options={THEMES} value={theme} onChange={(t: Theme) => setTheme(t)} />
      <label class="label-row mt-16">
        <span>Dim the screen</span>
        <b>{dim}%</b>
      </label>
      <input type="range" min="0" max="85" step="1" value={dim} onInput={(e) => setDim(Number(e.currentTarget.value))} />
      <div class="hint">Stored in this browser only.</div>
    </Accordion>
  );
}
