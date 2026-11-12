import { Accordion } from '../../components/Accordion';
import { Fields, NumberField } from '../../components/Fields';
import { Switch } from '../../components/Switch';
import { useSettings } from '../../hooks/useSettings';
import type { Site } from '../../types';

export function SkySection({ site }: { site: Site | null }) {
  const { values: v, set } = useSettings();
  const where = site ? `${site.lat.toFixed(2)}°, ${site.lon.toFixed(2)}° (${site.source})` : 'location unknown';
  return (
    <Accordion icon="sun" title="Location & sky" summary={`${where} · refraction ${v.refraction ? 'on' : 'off'}`}>
      <Fields>
        <NumberField label="Latitude override" unit="°" value={v.lat} nullable placeholder="from mount" onChange={(n) => set('lat', n)} />
        <NumberField label="Longitude override" unit="°" value={v.lon} nullable placeholder="from mount" onChange={(n) => set('lon', n)} />
        <div class="full">
          <Switch checked={v.refraction} onChange={(on) => set('refraction', on)} label="Correct for refraction" hint="Align to the apparent pole, as seen through the atmosphere." />
        </div>
        <NumberField label="Pressure" unit="hPa" value={v.pressure_hpa} onChange={(n) => n !== null && set('pressure_hpa', n)} />
        <NumberField label="Temperature" unit="°C" value={v.temp_c} onChange={(n) => n !== null && set('temp_c', n)} />
      </Fields>
    </Accordion>
  );
}
