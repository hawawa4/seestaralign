import { Accordion } from '../../components/Accordion';
import { Switch } from '../../components/Switch';
import { useSettings } from '../../hooks/useSettings';

export function ClockSection() {
  const { values: v, set } = useSettings();
  return (
    <Accordion icon="clock" title="Seestar clock" summary={v.sync_clock ? 'Synced on connect' : 'Not synced'}>
      <Switch
        checked={v.sync_clock}
        onChange={(on) => set('sync_clock', on)}
        label="Sync the clock on connect"
        hint="The Seestar has no battery-backed clock, and 1 s of error is 15″ of sky rotation."
      />
    </Accordion>
  );
}
