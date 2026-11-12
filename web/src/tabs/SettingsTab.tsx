import { useEffect, useState } from 'preact/hooks';
import { useSettings } from '../hooks/useSettings';
import type { AppState } from '../types';
import { CaptureSection } from './settings/CaptureSection';
import { ClockSection } from './settings/ClockSection';
import { DisplaySection } from './settings/DisplaySection';
import { SkySection } from './settings/SkySection';
import { SolverSection } from './settings/SolverSection';
import { StartPositionSection } from './settings/StartPositionSection';
import './SettingsTab.css';

/** "✓ Saved" that fades out a moment after each successful save. */
function SavedFlash() {
  const { savedTick } = useSettings();
  const [show, setShow] = useState(false);
  useEffect(() => {
    if (savedTick === 0) return;
    setShow(true);
    const t = setTimeout(() => setShow(false), 1600);
    return () => clearTimeout(t);
  }, [savedTick]);
  return <span class={`saved${show ? ' show' : ''}`}>✓ Saved</span>;
}

export function SettingsTab({ state }: { state: AppState }) {
  return (
    <div class="tab">
      <div class="pagehead">
        <h2>Settings</h2>
        <SavedFlash />
      </div>
      <div class="settings-grid">
        <StartPositionSection site={state.site} />
        <CaptureSection />
        <SkySection site={state.site} />
        <SolverSection astap={state.astap} />
        <ClockSection />
        <DisplaySection />
      </div>
    </div>
  );
}
