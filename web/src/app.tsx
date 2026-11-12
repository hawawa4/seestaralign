import { useCallback, useEffect, useRef, useState } from 'preact/hooks';
import { Header } from './components/Header';
import { isTab, TabBar, type Tab } from './components/TabBar';
import { SettingsProvider } from './hooks/useSettings';
import { ThemeProvider, useTheme } from './hooks/useTheme';
import { ToastProvider } from './hooks/useToast';
import { useServerState } from './hooks/useServerState';
import { useWakeLock } from './hooks/useWakeLock';
import { countUnseen } from './lib/logs';
import { storage } from './lib/storage';
import { AlignTab } from './tabs/AlignTab';
import { ConnectTab } from './tabs/ConnectTab';
import { LogTab } from './tabs/LogTab';
import { SettingsTab } from './tabs/SettingsTab';
import type { AppState } from './types';

function DimOverlay() {
  const { dim } = useTheme();
  return <div class="dim" style={{ opacity: dim / 100 }} />;
}

function Content({ state }: { state: AppState }) {
  const [tab, setTabState] = useState<Tab>(() => {
    const h = location.hash.slice(1);
    return isTab(h) ? h : state.phase === 'disconnected' ? 'connect' : 'align';
  });
  const go = useCallback((t: Tab) => {
    setTabState(t);
    history.replaceState(null, '', `#${t}`);
    window.scrollTo(0, 0);
  }, []);
  useEffect(() => {
    const onHash = () => {
      const h = location.hash.slice(1);
      if (isTab(h)) setTabState(h);
    };
    window.addEventListener('hashchange', onHash);
    return () => window.removeEventListener('hashchange', onHash);
  }, []);

  // Once a connection attempt succeeds, move on from the Connect tab.
  const prevPhase = useRef(state.phase);
  useEffect(() => {
    if (prevPhase.current === 'connecting' && state.phase === 'ready' && tab === 'connect') go('align');
    prevPhase.current = state.phase;
  }, [state.phase, tab, go]);

  // Badge on the Log tab for warnings/errors you haven't looked at yet.
  const [seen, setSeen] = useState(() => storage.get('logSeen', ''));
  const latest = state.log?.length ? state.log[state.log.length - 1]!.t : '';
  useEffect(() => {
    if (tab === 'log' && latest) {
      setSeen(latest);
      storage.set('logSeen', latest);
    }
  }, [tab, latest]);

  const running = state.phase === 'measuring' || state.phase === 'adjusting';
  const awake = useWakeLock(running);

  return (
    <SettingsProvider server={state.settings}>
      <TabBar tab={tab} onSelect={go} logBadge={tab === 'log' ? 0 : countUnseen(state.log, seen)} />
      <main class="wrap">
        {tab === 'align' && <AlignTab state={state} awake={awake} onNavigate={go} />}
        {tab === 'connect' && <ConnectTab state={state} />}
        {tab === 'settings' && <SettingsTab state={state} />}
        {tab === 'log' && <LogTab state={state} />}
      </main>
    </SettingsProvider>
  );
}

function Shell() {
  const { state, online } = useServerState();
  return (
    <>
      <Header phase={state?.phase ?? null} online={online} />
      {state ? (
        <Content state={state} />
      ) : (
        <main class="wrap">
          <div class="tab muted">Connecting to the server…</div>
        </main>
      )}
      <DimOverlay />
    </>
  );
}

export function App() {
  return (
    <ThemeProvider>
      <ToastProvider>
        <Shell />
      </ToastProvider>
    </ThemeProvider>
  );
}
