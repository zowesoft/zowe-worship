import { useEffect, useState } from 'react';
import { Events } from '@wailsio/runtime';
import { FittedImage } from '../../components/FittedImage';
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
    if (!next.image) {
      setShown(next);
      return;
    }

    // Preload the file before replacing what is already on screen, so an image
    // deleted after it was scheduled cannot swap the room's slide for a broken
    // one. On failure the previous slide simply stays.
    const probe = new window.Image();
    probe.onload = () => setShown(next);
    probe.onerror = () => undefined;
    probe.src = next.image;
  }, [state]);

  if (!shown) {
    return <main className="audience audience--blank" aria-label="Audience display" />;
  }

  if (shown.image) {
    return (
      <main className="audience audience--image" aria-label="Audience display">
        <FittedImage
          src={shown.image}
          alt=""
          ratio={shown.ratio}
          fit={shown.fit === 'cover' ? 'cover' : 'contain'}
        />
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
