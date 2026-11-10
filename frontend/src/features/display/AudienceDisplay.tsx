import { useEffect, useRef, useState } from 'react';
import { Events } from '@wailsio/runtime';
import { SlideMedia } from '../../components/SlideMedia';
import { ThemedSlide } from '../../components/ThemedSlide';
import { App } from '../../../bindings/zowe-worship';
import type { PresentationView } from '../../../bindings/zowe-worship/models';
import type { Content } from '../../../bindings/zowe-worship/internal/presentation/models';

export function AudienceDisplay() {
  const [state, setState] = useState<PresentationView | null>(null);
  const [shown, setShown] = useState<Content | null>(null);
  const mediaRef = useRef<HTMLMediaElement>(null);
  // The volume the studio last set; a replaced element starts back at full, so
  // the stored value is applied again on the next clip.
  const volumeRef = useRef<number | null>(null);

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
  useEffect(() => Events.On('media-control', (event) => {
    const element = mediaRef.current;
    if (!element) return;
    const command = event.data as { action: string; value: number };

    switch (command.action) {
      case 'pause':
        element.pause();
        return;
      case 'stop':
        element.pause();
        element.currentTime = 0;
        return;
      case 'restart':
        element.currentTime = 0;
        element.play().catch(() => {
          // A clip that refuses to start stays put; the operator sees it on screen.
        });
        return;
      case 'seek':
        element.currentTime = Math.min(command.value, 1) * (element.duration || 0);
        return;
      case 'volume':
        volumeRef.current = Math.min(Math.max(command.value, 0), 100);
        element.volume = volumeRef.current / 100;
        return;
      default: // 'play'
        element.play().catch(() => undefined);
    }
  }), []);

  // The clock goes back to the studio, twice a second, so its transport bar
  // tracks the clip the room is watching.
  useEffect(() => {
    const element = mediaRef.current;
    if (!element) return;

    const report = () => {
      void App.MediaProgress(
        Math.round(element.currentTime * 1000),
        Number.isFinite(element.duration) ? Math.round(element.duration * 1000) : 0,
      );
    };
    element.addEventListener('timeupdate', report);
    element.addEventListener('loadedmetadata', report);
    // A new clip or a reattached player needs the last commanded volume.
    if (volumeRef.current !== null) element.volume = volumeRef.current / 100;
    return () => {
      element.removeEventListener('timeupdate', report);
      element.removeEventListener('loadedmetadata', report);
    };
  }, [shown?.source]);

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
