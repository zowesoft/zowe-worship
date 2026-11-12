import { createContext, useCallback, useContext, useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { Events } from '@wailsio/runtime';
import { App } from '../../bindings/zopresent';
import type * as models from '../../bindings/zopresent/models';
import { DEFAULT_SHORTCUTS } from '../lib/shortcuts';

type ShortcutsValue = {
  shortcuts: models.ShortcutsView;
  refresh: () => Promise<void>;
};

const ShortcutsContext = createContext<ShortcutsValue | null>(null);

export function ShortcutsProvider({ children }: { children: ReactNode }) {
  const [shortcuts, setShortcuts] = useState<models.ShortcutsView>(DEFAULT_SHORTCUTS);

  const refresh = useCallback(async () => {
    try {
      setShortcuts(await App.GetShortcuts());
    } catch {
      // Without the bridge the defaults stand.
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Shortcuts are edited in the Settings window, a separate webview, so a saved
  // map arrives as an event; re-read it or the studio keeps firing the old keys.
  useEffect(() => {
    const cancel = Events.On('shortcuts:config', () => {
      void refresh();
    });
    return () => {
      cancel();
    };
  }, [refresh]);

  return (
    <ShortcutsContext.Provider value={{ shortcuts, refresh }}>
      {children}
    </ShortcutsContext.Provider>
  );
}

export function useShortcuts(): ShortcutsValue {
  const context = useContext(ShortcutsContext);
  if (!context) throw new Error('useShortcuts must be used inside ShortcutsProvider.');
  return context;
}
