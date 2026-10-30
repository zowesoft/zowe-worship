import { useEffect, useState } from 'react';
import { Events } from '@wailsio/runtime';
import { App } from '../../../bindings/zowe-worship';
import type { PresentationView } from '../../../bindings/zowe-worship/models';
import './audience.css';

export function AudienceDisplay() {
  const [state, setState] = useState<PresentationView | null>(null);

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

  const live = state?.live ?? null;

  if (!live) {
    return <main className="audience audience--blank" aria-label="Audience display" />;
  }

  return (
    <main className="audience" aria-label="Audience display">
      <p className="audience-lines">
        {(live.lines ?? []).map((line, index) => <span key={index}>{line}</span>)}
      </p>
      <p className="audience-label">{live.label}</p>
    </main>
  );
}
