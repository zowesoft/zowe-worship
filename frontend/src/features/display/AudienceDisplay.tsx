import { useEffect, useState } from 'react';
import { Events } from '@wailsio/runtime';
import { SlideMedia } from '../../components/SlideMedia';
import { App } from '../../../bindings/zowe-worship';
import type { PresentationView } from '../../../bindings/zowe-worship/models';
import type { Content } from '../../../bindings/zowe-worship/internal/presentation/models';
import './audience.css';

export function AudienceDisplay() {
  const [state, setState] = useState<PresentationView | null>(null);
  const [shown, setShown] = useState<Content | null>(null);

  useEffect(() => {
    let active = true;

    App.GetPresentationState()
      .then((value) => {
        if (active) setState(value);
      })
      .catch(() => {
        // A display that cannot reach the studio stays black.
      });

    const cancel = Events.On('presentation', (event) => {
      setState(event.data as PresentationView);
    });

    return () => {
      active = false;
      cancel();
    };
  }, []);

  useEffect(() => {
    const next = state?.live ?? null;
    if (!next) {
      setShown(null);
      return;
    }
    // Only a picture can be proved to load before it replaces what the room is
    // already seeing. A clip that will not decode shows its own error, and the
    // operator sends something else.
    if (next.kind !== 'image') {
      setShown(next);
      return;
    }

    const probe = new window.Image();
    probe.onload = () => setShown(next);
    probe.onerror = () => undefined;
    probe.src = next.source ?? '';
  }, [state]);

  if (!shown) {
    return <main className="audience audience--blank" aria-label="Audience display" />;
  }

  if (shown.kind === 'image' || shown.kind === 'video' || shown.kind === 'audio') {
    const frame = shown.kind === 'image' ? 'audience audience--image'
      : shown.kind === 'video' ? 'audience audience--media'
        : 'audience';
    return (
      <main className={frame} aria-label="Audience display">
        <SlideMedia content={shown} player />
      </main>
    );
  }

  return (
    <main className="audience" aria-label="Audience display">
      <p className="audience-lines">
        {(shown.lines ?? []).map((line, index) => <span key={index}>{line}</span>)}
      </p>
      <p className="audience-label">{shown.label}</p>
    </main>
  );
}
