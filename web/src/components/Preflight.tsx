import type { ComponentChildren } from 'preact';
import type { AppState } from '../types';
import { num1 } from '../lib/format';
import type { Tab } from './TabBar';
import './Preflight.css';

type Status = 'ok' | 'bad' | 'off';

function Row({ status, title, detail, action }: { status: Status; title: string; detail: string; action?: ComponentChildren }) {
  return (
    <div class={`pf ${status}`}>
      <span class="ic">{status === 'ok' ? '✓' : status === 'bad' ? '!' : '–'}</span>
      <div class="tx">
        {title}
        <small>{detail}</small>
      </div>
      {action}
    </div>
  );
}

/** What has to be true before Start works, and what the run will do first. */
export function Preflight({ state, onNavigate }: { state: AppState; onNavigate: (t: Tab) => void }) {
  const { settings: set, site, model, astap, rig } = state;
  const connected = state.phase !== 'disconnected' && state.phase !== 'connecting';
  const go = (tab: Tab, label: string) => (
    <button class="btn sm ghost" onClick={() => onNavigate(tab)}>
      {label}
    </button>
  );
  const where = set.start_mode === 'radec' ? `RA ${num1(set.start_ra_hours)} h · Dec ${num1(set.start_dec_deg)}°` : `Alt ${num1(set.start_alt_deg)}° · Az ${num1(set.start_az_deg)}°`;

  return (
    <div class="card">
      <h2>Before you start</h2>
      {connected ? (
        <Row status="ok" title="Seestar connected" detail={rig?.telescope.name || rig?.base_url || ''} />
      ) : (
        <Row status="bad" title="Seestar not connected" detail="Connect over Alpaca first." action={go('connect', 'Connect')} />
      )}
      {site ? (
        <Row status="ok" title="Location known" detail={`${site.lat.toFixed(3)}°, ${site.lon.toFixed(3)}° · from ${site.source}`} />
      ) : (
        <Row status={connected ? 'bad' : 'off'} title="Location unknown" detail="Enter latitude/longitude in Settings." action={go('settings', 'Settings')} />
      )}
      {model ? (
        <Row status="ok" title={`Model: ${model.name}`} detail={state.model_source} />
      ) : (
        <Row status={connected ? 'bad' : 'off'} title="Model not set" detail="Pick your Seestar model on the Connect tab." action={go('connect', 'Choose')} />
      )}
      {astap.path ? (
        <Row status="ok" title="Plate solver ready" detail={astap.path} />
      ) : (
        <Row status="bad" title="ASTAP not found" detail="Install astap_cli, or set its path in Settings." action={go('settings', 'Settings')} />
      )}
      {set.start_enabled ? (
        <Row status="ok" title="Will slew to the starting position" detail={where} action={go('settings', 'Edit')} />
      ) : (
        <Row status="off" title="Starting position: off" detail="Aim by hand near the meridian, 30–60° up, away from the pole and zenith." action={go('settings', 'Set')} />
      )}
    </div>
  );
}
