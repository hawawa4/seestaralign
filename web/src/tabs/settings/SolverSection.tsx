import { Accordion } from '../../components/Accordion';
import { Fields, TextField } from '../../components/Fields';
import { useSettings } from '../../hooks/useSettings';
import type { AppState } from '../../types';

export function SolverSection({ astap }: { astap: AppState['astap'] }) {
  const { values: v, set } = useSettings();
  const dbs = astap.databases ?? [];
  const summary = astap.path ? `ASTAP found${dbs.length ? ` · ${dbs.join(', ')}` : ' · no star database'}` : 'ASTAP not found';
  const info = astap.path ? `Using ${astap.path}${dbs.length ? ` · databases: ${dbs.join(', ')}` : ' · no star database detected'}` : 'ASTAP not found – install astap_cli';
  return (
    <Accordion icon="sparkle" title="Plate solving" summary={summary}>
      <Fields>
        <TextField full label="ASTAP binary" value={v.astap_path} placeholder="auto (astap_cli)" hint={info} onChange={(s) => set('astap_path', s)} />
        <TextField label="Star database" value={v.astap_database} placeholder="auto (e.g. d50)" onChange={(s) => set('astap_database', s)} />
        <TextField label="Database folder" value={v.astap_database_dir} placeholder="auto" onChange={(s) => set('astap_database_dir', s)} />
      </Fields>
    </Accordion>
  );
}
