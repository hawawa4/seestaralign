// Empties internal/web/dist but keeps the .gitkeep that lets `go build`
// work before the frontend has ever been built.
import { readdirSync, rmSync, mkdirSync, writeFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';

const dist = new URL('../../internal/web/dist', import.meta.url).pathname;
mkdirSync(dist, { recursive: true });
for (const f of readdirSync(dist)) if (f !== '.gitkeep') rmSync(join(dist, f), { recursive: true, force: true });
if (!existsSync(join(dist, '.gitkeep'))) writeFileSync(join(dist, '.gitkeep'), '');
