import { Accordion } from '../../components/Accordion';
import { Fields, NumberField } from '../../components/Fields';
import { Segmented } from '../../components/Segmented';
import { useSettings } from '../../hooks/useSettings';
import type { Direction } from '../../types';

const DIRECTIONS = [
  { value: 'east', label: 'East' },
  { value: 'west', label: 'West' },
] as const;

export function CaptureSection() {
  const { values: v, set } = useSettings();
  return (
    <Accordion icon="camera" title="Capture & movement" summary={`${v.exposure_s} s exposures · ${v.move_deg}° moves ${v.direction}`} defaultOpen>
      <Fields>
        <NumberField label="Exposure" unit="s" value={v.exposure_s} onChange={(n) => n !== null && set('exposure_s', n)} />
        <NumberField label="Gain" value={v.gain} integer nullable placeholder="leave as is" onChange={(n) => set('gain', n)} />
        <div class="full">
          <label class="label-row">
            <span>Move per step</span>
            <b>{v.move_deg}°</b>
          </label>
          <input type="range" min="5" max="60" step="1" value={v.move_deg} onInput={(e) => set('move_deg', Number(e.currentTarget.value))} />
          <div class="hint">Bigger is more accurate. Needs {v.move_deg * 2}° of free sky to the {v.direction} (two moves of {v.move_deg}°).</div>
        </div>
        <div>
          <label>Direction</label>
          <Segmented block options={DIRECTIONS} value={v.direction} onChange={(d: Direction) => set('direction', d)} />
        </div>
        <NumberField label="Move rate" unit="°/s" value={v.move_rate} onChange={(n) => n !== null && set('move_rate', n)} />
        <NumberField label="Settle time" unit="s" value={v.settle_s} onChange={(n) => n !== null && set('settle_s', n)} />
        <NumberField label="Search radius" unit="°" value={v.search_radius_deg} onChange={(n) => n !== null && set('search_radius_deg', n)} />
      </Fields>
    </Accordion>
  );
}
