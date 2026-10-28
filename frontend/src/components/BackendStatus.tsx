import { useEffect, useState } from 'react';
import { GetAppInfo } from '../../wailsjs/go/main/App.js';
import { hasDesktopBridge } from '../lib/desktop';

type BridgeState =
  | { kind: 'connecting' }
  | { kind: 'browser' }
  | { kind: 'connected'; name: string; version: string }
  | { kind: 'error'; message: string };

export function BackendStatus() {
  const [bridge, setBridge] = useState<BridgeState>(() => ({
    kind: hasDesktopBridge() ? 'connecting' : 'browser',
  }));

  useEffect(() => {
    if (!hasDesktopBridge()) return;

    let mounted = true;

    async function loadAppInfo() {
      try {
        const info = await GetAppInfo();
        if (mounted) {
          setBridge({ kind: 'connected', name: info.name, version: info.version });
        }
      } catch (error: unknown) {
        if (mounted) {
          const message = error instanceof Error ? error.message : String(error);
          setBridge({ kind: 'error', message: message || 'Application information could not be loaded.' });
        }
      }
    }

    void loadAppInfo();
    return () => {
      mounted = false;
    };
  }, []);

  return (
    <section className={`backend-status backend-status--${bridge.kind}`} aria-label="Application environment" aria-live="polite" aria-atomic="true">
      <div className="status-line"><span className="status-dot" />
        <strong>
          {bridge.kind === 'browser' && 'Browser preview'}
          {bridge.kind === 'connecting' && 'Connecting to desktop…'}
          {bridge.kind === 'connected' && 'Desktop bridge connected'}
          {bridge.kind === 'error' && 'Desktop bridge error'}
        </strong>
      </div>
      <p>
        {bridge.kind === 'browser' && 'Desktop bridge unavailable in this browser.'}
        {bridge.kind === 'connecting' && 'Reading application information.'}
        {bridge.kind === 'connected' && `${bridge.name} · v${bridge.version}`}
        {bridge.kind === 'error' && bridge.message}
      </p>
    </section>
  );
}
