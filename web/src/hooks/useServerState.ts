import { useEffect, useState } from 'preact/hooks';
import type { AppState } from '../types';

/** The server's whole state, pushed over SSE on every change. */
export function useServerState(): { state: AppState | null; online: boolean } {
  const [state, setState] = useState<AppState | null>(null);
  const [online, setOnline] = useState(true);
  useEffect(() => {
    const es = new EventSource('/api/events');
    es.onmessage = (m) => {
      try {
        setState(JSON.parse(m.data) as AppState);
        setOnline(true);
      } catch (e) {
        console.error('bad state message', e);
      }
    };
    es.onerror = () => setOnline(false); // EventSource reconnects by itself
    return () => es.close();
  }, []);
  return { state, online };
}
