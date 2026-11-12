import { useEffect, useRef, useState } from 'preact/hooks';
import { Segmented } from '../components/Segmented';
import { useToast } from '../hooks/useToast';
import { isProblem, logAsText } from '../lib/logs';
import type { AppState } from '../types';
import './LogTab.css';

type Filter = 'all' | 'problems';
const FILTERS = [
  { value: 'all', label: 'All' },
  { value: 'problems', label: 'Warnings & errors' },
] as const;

async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    // clipboard API needs https/localhost; fall back to a temporary textarea
    const ta = document.createElement('textarea');
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    try {
      return document.execCommand('copy');
    } finally {
      ta.remove();
    }
  }
}

export function LogTab({ state }: { state: AppState }) {
  const toast = useToast();
  const [filter, setFilter] = useState<Filter>('all');
  const all = state.log ?? [];
  const list = filter === 'problems' ? all.filter(isProblem) : all;

  // follow new lines only while scrolled to the bottom
  const box = useRef<HTMLDivElement>(null);
  const stick = useRef(true);
  useEffect(() => {
    const el = box.current;
    if (el && stick.current) el.scrollTop = el.scrollHeight;
  }, [list.length]);

  return (
    <div class="tab">
      <div class="pagehead"><h2>Log</h2></div>
      <div class="card">
        <div class="toolbar">
          <Segmented options={FILTERS} value={filter} onChange={setFilter} />
          <span class="sp" />
          <button class="btn ghost sm" onClick={async () => ((await copyText(logAsText(state.log))) ? toast.info('Log copied') : toast.error("Couldn't copy"))}>Copy</button>
        </div>
        <div
          class="log"
          ref={box}
          onScroll={(e) => {
            const el = e.currentTarget;
            stick.current = el.scrollTop + el.clientHeight >= el.scrollHeight - 4;
          }}
        >
          {list.length === 0 && <span class="muted">Nothing here yet.</span>}
          {list.map((l, i) => (
            <div class={`${l.level}-l`} key={`${l.t}-${i}`}>
              <time>{new Date(l.t).toLocaleTimeString()}</time>
              {l.msg}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
