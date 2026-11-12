import { useState } from 'preact/hooks';
import { api } from '../api';
import { Fields, NumberField, SelectField, TextField } from '../components/Fields';
import { pillState, PHASE_LABELS } from '../components/Header';
import { Pill } from '../components/Pill';
import { Warnings } from '../components/Warnings';
import { useSettings } from '../hooks/useSettings';
import { useToast } from '../hooks/useToast';
import type { AppState, FoundDevice } from '../types';
import './ConnectTab.css';

const ALIGNMENT_MODES: Record<number, string> = { 0: 'Alt-Az', 1: 'Polar (EQ)', 2: 'German polar' };

export function ConnectTab({ state }: { state: AppState }) {
  const { values, set, flush } = useSettings();
  const toast = useToast();
  const [found, setFound] = useState<FoundDevice[] | 'scanning' | null>(null);
  const [busy, setBusy] = useState(false);

  const { phase, rig, site } = state;
  const connected = phase !== 'disconnected' && phase !== 'connecting';
  const guard = async (fn: () => Promise<unknown>) => {
    try {
      await fn();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    }
  };

  const toggleConnection = async () => {
    setBusy(true);
    await guard(async () => {
      await flush(); // make sure the address typed a moment ago is what gets used
      await (connected ? api.disconnect() : api.connect());
    });
    setBusy(false);
  };

  const scan = async () => {
    setFound('scanning');
    try {
      setFound(await api.discover());
    } catch (e) {
      setFound(null);
      toast.error(e instanceof Error ? e.message : String(e));
    }
  };

  const modelOptions = [
    { value: '', label: state.model && !values.model ? `Auto (${state.model.name}, ${state.model_source})` : 'Auto' },
    ...state.models.map((m) => ({ value: m.key, label: m.name })),
  ];

  return (
    <div class="tab">
      <div class="pagehead"><h2>Connect</h2></div>
      <div class="grid2">
        <div>
          <div class="card">
            <div class="conn-state">
              <Pill state={pillState(phase)} label={connected ? 'connected' : PHASE_LABELS[phase]} />
              <span class="muted small">
                {connected && rig ? rig.base_url : phase === 'connecting' ? 'Looking for the Seestar…' : 'Enable Alpaca in the Seestar app first.'}
              </span>
            </div>
            <div class="row">
              <div class="grow">
                <TextField label="Seestar address" value={values.host} placeholder="seestar.local" disabled={connected} onChange={(v) => set('host', v)} />
              </div>
              <div class="port">
                <NumberField label="Port" value={values.port} placeholder="auto" integer zeroEmpty disabled={connected} onChange={(v) => set('port', v ?? 0)} />
              </div>
            </div>
            <div class="row between mt-14">
              <button class="btn ghost sm" disabled={connected || found === 'scanning'} onClick={scan}>Scan network</button>
              <button class={`btn${connected ? ' ghost' : ''}`} disabled={phase === 'connecting' || busy} onClick={toggleConnection}>
                {phase === 'connecting' ? 'Connecting…' : connected ? 'Disconnect' : 'Connect'}
              </button>
            </div>
            <div class="hosts">
              {found === 'scanning' && <span class="small muted mt-10">Scanning…</span>}
              {Array.isArray(found) && found.length === 0 && (
                <span class="small muted mt-10">No Alpaca devices answered. Is Alpaca enabled in the Seestar app?</span>
              )}
              {Array.isArray(found) &&
                found.map((f) => (
                  <button
                    class="chip"
                    key={`${f.ip}:${f.port}`}
                    onClick={() => {
                      set('host', f.ip);
                      set('port', f.port);
                    }}
                  >
                    {f.ip}:{f.port}
                  </button>
                ))}
            </div>
          </div>
          <Warnings items={state.warnings} />
        </div>

        <div>
          {rig ? (
            <div class="card">
              <h2>Device</h2>
              <dl class="kv">
                <dt>Camera</dt>
                <dd>
                  {rig.camera.name || `#${rig.camera.number}`} {rig.sensor_w > 0 && <span class="muted">{rig.sensor_w}×{rig.sensor_h}</span>}
                </dd>
                <dt>Mount</dt>
                <dd>
                  {rig.telescope.name || `#${rig.telescope.number}`} {rig.alignment_mode >= 0 && <span class="muted">{ALIGNMENT_MODES[rig.alignment_mode] ?? ''}</span>}
                </dd>
                <dt>Site</dt>
                <dd>
                  {site ? (
                    <>
                      {site.lat.toFixed(4)}°, {site.lon.toFixed(4)}° <span class="muted">({site.source})</span>
                    </>
                  ) : (
                    <span class="badc">unknown – set it in Settings</span>
                  )}
                </dd>
              </dl>
              <div class="mt-16">
                <Fields>
                  <SelectField
                    full
                    label="Model"
                    value={values.model}
                    options={modelOptions}
                    onChange={(v) => set('model', v)}
                    hint="The S50/S30 and the Pro models can't be told apart from the sensor size. If a solve matches another model's scale, the app switches automatically."
                  />
                </Fields>
              </div>
            </div>
          ) : (
            <div class="card">
              <h2>Not connected</h2>
              <p class="sub mt-6">
                Type the address (or IP) of your Seestar and press Connect, or scan the network for Alpaca devices. Port <b>auto</b> tries discovery first, then 32323.
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
