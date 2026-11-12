import { createContext, type ComponentChildren } from 'preact';
import { useCallback, useContext, useEffect, useMemo, useRef, useState } from 'preact/hooks';
import { api } from '../api';
import type { Settings } from '../types';
import { useToast } from './useToast';

interface SettingsApi {
  /** Saved settings overlaid with edits that haven't been confirmed by the server yet. */
  values: Settings;
  set: <K extends keyof Settings>(key: K, value: Settings[K]) => void;
  /** Send pending edits now (e.g. before connecting). */
  flush: () => Promise<void>;
  /** Changes every time a save succeeds. */
  savedTick: number;
}

const Ctx = createContext<SettingsApi | null>(null);

const SAVE_DELAY_MS = 350;
const CONFIRM_TIMEOUT_MS = 1500;

export function SettingsProvider({ server, children }: { server: Settings; children: ComponentChildren }) {
  const toast = useToast();
  const [edits, setEdits] = useState<Partial<Settings>>({}); // shown to the user
  const unsent = useRef<Partial<Settings>>({}); // not yet PUT
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const [savedTick, setSavedTick] = useState(0);

  const values = useMemo(() => ({ ...server, ...edits }), [server, edits]);
  const valuesRef = useRef(values);
  valuesRef.current = values;

  // Drop edits as soon as the server reports the same value back.
  useEffect(() => {
    setEdits((e) => {
      const next = { ...e };
      let changed = false;
      for (const k of Object.keys(e) as (keyof Settings)[]) {
        if (!(k in unsent.current) && server[k] === e[k]) {
          delete next[k];
          changed = true;
        }
      }
      return changed ? next : e;
    });
  }, [server]);

  const forget = useCallback((patch: Partial<Settings>) => {
    setEdits((e) => {
      const next = { ...e };
      for (const k of Object.keys(patch) as (keyof Settings)[]) if (next[k] === patch[k]) delete next[k];
      return next;
    });
  }, []);

  const flush = useCallback(async () => {
    clearTimeout(timer.current);
    const patch = unsent.current;
    if (Object.keys(patch).length === 0) return;
    unsent.current = {};
    try {
      await api.saveSettings(patch);
      setSavedTick((n) => n + 1);
      setTimeout(() => forget(patch), CONFIRM_TIMEOUT_MS); // normally the server echo gets there first
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
      forget(patch); // fall back to what the server has
    }
  }, [forget, toast]);

  const set = useCallback<SettingsApi['set']>(
    (key, value) => {
      const change: Partial<Settings> = { [key]: value };
      if (key === 'lat' || key === 'lon') {
        // the server wants latitude and longitude together
        change.lat = key === 'lat' ? (value as number | null) : valuesRef.current.lat;
        change.lon = key === 'lon' ? (value as number | null) : valuesRef.current.lon;
      }
      unsent.current = { ...unsent.current, ...change };
      setEdits((e) => ({ ...e, ...change }));
      clearTimeout(timer.current);
      timer.current = setTimeout(() => void flush(), SAVE_DELAY_MS);
    },
    [flush],
  );

  const ctx = useMemo(() => ({ values, set, flush, savedTick }), [values, set, flush, savedTick]);
  return <Ctx.Provider value={ctx}>{children}</Ctx.Provider>;
}

export function useSettings(): SettingsApi {
  const v = useContext(Ctx);
  if (!v) throw new Error('useSettings outside SettingsProvider');
  return v;
}
