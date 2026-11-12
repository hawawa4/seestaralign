import { Accordion } from '../../components/Accordion';
import { Fields, NumberField } from '../../components/Fields';
import { Segmented } from '../../components/Segmented';
import { SkyDome } from '../../components/SkyDome';
import { Switch } from '../../components/Switch';
import { useSettings } from '../../hooks/useSettings';
import type { Site, StartMode } from '../../types';

const MODES = [
  { value: 'altaz', label: 'Alt / Az' },
  { value: 'radec', label: 'RA / Dec' },
] as const;

export function StartPositionSection({ site }: { site: Site | null }) {
  const { values: v, set } = useSettings();
  const summary = !v.start_enabled
    ? 'Off – aim by hand'
    : v.start_mode === 'radec'
      ? `RA ${v.start_ra_hours} h · Dec ${v.start_dec_deg}°`
      : `Alt ${v.start_alt_deg}° · Az ${v.start_az_deg}°`;

  return (
    <Accordion icon="start" title="Starting position" summary={summary} defaultOpen>
      <Switch checked={v.start_enabled} onChange={(on) => set('start_enabled', on)} label="Slew here first" hint="The first step of every run, saved for next time." />
      <div class={`mt-16${v.start_enabled ? '' : ' dimmed'}`}>
        <Segmented block options={MODES} value={v.start_mode} onChange={(m: StartMode) => set('start_mode', m)} />
        {v.start_mode === 'altaz' ? (
          <div class="dome-wrap">
            <SkyDome
              altDeg={v.start_alt_deg}
              azDeg={v.start_az_deg}
              enabled={v.start_enabled}
              latitude={site?.lat ?? null}
              onPick={(alt, az) => {
                set('start_alt_deg', alt);
                set('start_az_deg', az);
              }}
            />
            <div>
              <Fields columns={2}>
                <NumberField label="Altitude" unit="°" value={v.start_alt_deg} onChange={(n) => n !== null && set('start_alt_deg', n)} />
                <NumberField label="Azimuth" unit="°" value={v.start_az_deg} onChange={(n) => n !== null && set('start_az_deg', n)} />
              </Fields>
              <div class="hint mt-10">
                Drag on the dome or type values. Top-down view: north is up, east is right, the centre is the zenith. The shaded ring is 30–60° up, a good spot near the meridian, away from the pole and zenith.
              </div>
            </div>
          </div>
        ) : (
          <div class="mt-16">
            <Fields>
              <NumberField label="Right ascension" unit="h" value={v.start_ra_hours} onChange={(n) => n !== null && set('start_ra_hours', n)} />
              <NumberField label="Declination" unit="°" value={v.start_dec_deg} onChange={(n) => n !== null && set('start_dec_deg', n)} />
            </Fields>
            <div class="hint">In the frame the mount reports (JNow), not catalogue J2000.</div>
          </div>
        )}
      </div>
    </Accordion>
  );
}
