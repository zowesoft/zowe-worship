import { useEffect, useRef, useState } from 'react';
import { Events } from '@wailsio/runtime';
import { SlideMedia } from '../../components/SlideMedia';
import { ThemedSlide } from '../../components/ThemedSlide';
import { useTransportPlayer } from '../../lib/mediaTransport';
import { App } from '../../../bindings/zowe-worship';
import type { PresentationView } from '../../../bindings/zowe-worship/models';
import type { Content } from '../../../bindings/zowe-worship/internal/presentation/models';

export function AudienceDisplay() {
  const [state, setState] = useState<PresentationView | null>(null);
  const [shown, setShown] = useState<Content | null>(null);
  const mediaRef = useRef<HTMLMediaElement>(null);

  useTransportPlayer(mediaRef, shown?.source);

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

  // The studio sends transport commands here, because this window holds the only
  // element that is actually playing.
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
    return <main className="h-full grid content-center justify-items-center gap-[clamp(24px,5vh,60px)] py-[clamp(24px,6vh,80px)] px-[clamp(28px,8vw,180px)] bg-[#000] text-text overflow-hidden" aria-label="Audience display" />;
  }

  if (shown.kind === 'image' || shown.kind === 'video' || shown.kind === 'audio') {
    const frame = shown.kind === 'image'
      ? 'h-full grid content-center justify-items-center gap-[clamp(24px,5vh,60px)] bg-app text-text overflow-hidden p-0'
      : shown.kind === 'video'
        ? 'h-full grid content-stretch justify-items-center gap-[clamp(24px,5vh,60px)] bg-app text-text overflow-hidden p-0'
        : 'h-full grid content-center justify-items-center gap-[clamp(24px,5vh,60px)] py-[clamp(24px,6vh,80px)] px-[clamp(28px,8vw,180px)] bg-app text-text overflow-hidden';
    return (
      <main className={frame} aria-label="Audience display">
        <SlideMedia content={shown} player mediaRef={mediaRef} onEnded={() => void App.MediaEnded()} />
      </main>
    );
  }

  return (
    <main className="h-full grid content-stretch justify-items-center gap-[clamp(24px,5vh,60px)] bg-app text-text overflow-hidden p-0" aria-label="Audience display">
      <ThemedSlide
        theme={shown.theme}
        lines={shown.lines ?? []}
        reference={shown.label}
        urlFor={(mediaID) => `/media/file/${mediaID}`}
        className="w-full h-full"
      />
    </main>
  );
}
