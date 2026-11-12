import { createContext, type ComponentChildren } from 'preact';
import { useCallback, useContext, useMemo, useRef, useState } from 'preact/hooks';
import { Toast } from '../components/Toast';

interface ToastApi {
  error: (msg: string) => void;
  info: (msg: string) => void;
}
const Ctx = createContext<ToastApi | null>(null);

export function ToastProvider({ children }: { children: ComponentChildren }) {
  const [toast, setToast] = useState<{ msg: string; level: 'error' | 'info'; n: number } | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const n = useRef(0);
  const show = useCallback((msg: string, level: 'error' | 'info') => {
    setToast({ msg, level, n: ++n.current });
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setToast(null), 6000);
  }, []);
  const api = useMemo<ToastApi>(() => ({ error: (m) => show(m, 'error'), info: (m) => show(m, 'info') }), [show]);
  return (
    <Ctx.Provider value={api}>
      {children}
      <Toast message={toast?.msg ?? ''} level={toast?.level ?? 'error'} visible={toast !== null} />
    </Ctx.Provider>
  );
}

export function useToast(): ToastApi {
  const v = useContext(Ctx);
  if (!v) throw new Error('useToast outside ToastProvider');
  return v;
}
