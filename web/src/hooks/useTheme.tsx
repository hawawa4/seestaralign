import { createContext, type ComponentChildren } from 'preact';
import { useContext, useEffect, useMemo, useState } from 'preact/hooks';
import { starfield } from '../lib/stars';
import { storage } from '../lib/storage';

export type Theme = 'night' | 'dark';

interface ThemeApi {
  theme: Theme;
  setTheme: (t: Theme) => void;
  /** 0–85: how much to black out the screen, on top of the theme. */
  dim: number;
  setDim: (n: number) => void;
}
const Ctx = createContext<ThemeApi | null>(null);

export function ThemeProvider({ children }: { children: ComponentChildren }) {
  const [theme, setThemeState] = useState<Theme>(storage.get('theme', 'night') === 'dark' ? 'dark' : 'night');
  const [dim, setDimState] = useState(() => Math.max(0, Math.min(85, Number(storage.get('dim', '0')) || 0)));

  useEffect(() => {
    const root = document.documentElement;
    if (theme === 'dark') root.dataset.theme = 'dark';
    else delete root.dataset.theme;
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', theme === 'dark' ? '#080b10' : '#000000');
    const star = getComputedStyle(root).getPropertyValue('--star').trim() || '#ff6a55';
    document.body.style.setProperty('--stars', starfield(star));
  }, [theme]);

  const api = useMemo<ThemeApi>(
    () => ({
      theme,
      dim,
      setTheme: (t) => {
        storage.set('theme', t);
        setThemeState(t);
      },
      setDim: (n) => {
        storage.set('dim', String(n));
        setDimState(n);
      },
    }),
    [theme, dim],
  );
  return <Ctx.Provider value={api}>{children}</Ctx.Provider>;
}

export function useTheme(): ThemeApi {
  const v = useContext(Ctx);
  if (!v) throw new Error('useTheme outside ThemeProvider');
  return v;
}
