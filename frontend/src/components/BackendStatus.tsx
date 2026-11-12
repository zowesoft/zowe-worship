import { useEffect, useState } from 'react';
import { App } from '../../bindings/zopresent';
import { useStudio } from '../stores/StudioProvider';

type BridgeState =
  | { kind: 'connecting' }
  | { kind: 'browser' }
  | { kind: 'connected'; name: string; version: string }
  | { kind: 'error'; message: string };

export function BackendStatus() {
  const studio = useStudio();
  const [info, setInfo] = useState<{ name: string; version: string } | null>(null);
  const [failure, setFailure] = useState<string | null>(null);

  useEffect(() => {
    if (!studio.bridgeAvailable) return;

    let mounted = true;

    App.GetAppInfo()
      .then((value) => {
        if (mounted) setInfo({ name: value.name, version: value.version });
      })
      .catch((error: unknown) => {
        if (!mounted) return;
        const message = error instanceof Error ? error.message : String(error);
        setFailure(message || 'Application information could not be loaded.');
      });

    return () => {
      mounted = false;
    };
  }, [studio.bridgeAvailable]);

  const bridge: BridgeState = !studio.ready
    ? { kind: 'connecting' }
    : !studio.bridgeAvailable
      ? { kind: 'browser' }
      : failure !== null
        ? { kind: 'error', message: failure }
        : info !== null
          ? { kind: 'connected', name: info.name, version: info.version }
          : { kind: 'connecting' };

  return (
    <section className="py-3.5 px-2 border-t border-border" aria-label="Application environment" aria-live="polite" aria-atomic="true">
      <div className={`flex gap-[7px] items-center ${bridge.kind === 'error' ? ' text-error' : ''}`}><span className={`inline-block w-1.5 h-1.5 rounded-full shrink-0 ${bridge.kind === 'connected' ? 'bg-teal' : bridge.kind === 'error' ? 'bg-error' : bridge.kind === 'connecting' ? 'bg-amber' : 'bg-secondary'}`} />
        <strong className="text-[10.5px] font-medium">
          {bridge.kind === 'browser' && 'Browser preview'}
          {bridge.kind === 'connecting' && 'Connecting to desktop…'}
          {bridge.kind === 'connected' && 'Desktop bridge connected'}
          {bridge.kind === 'error' && 'Desktop bridge error'}
        </strong>
      </div>
      <p className="mt-[5px] pl-[13px] text-secondary text-[10px] leading-[1.6] wrap-anywhere">
        {bridge.kind === 'browser' && (studio.error ?? 'Desktop bridge unavailable in this browser.')}
        {bridge.kind === 'connecting' && 'Reading application information.'}
        {bridge.kind === 'connected' && `${bridge.name} · v${bridge.version}`}
        {bridge.kind === 'error' && bridge.message}
      </p>
    </section>
  );
}
