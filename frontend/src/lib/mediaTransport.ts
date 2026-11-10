import { useEffect, useRef } from 'react';
import type { RefObject } from 'react';
import { Events } from '@wailsio/runtime';
import { App } from '../../bindings/zowe-worship';

// useTransportPlayer wires one media element to the studio transport: it takes
// commands and reports its clock back. The audience display uses it while it is
// the player; the studio uses it for the live pane when no display is open.
export function useTransportPlayer(mediaRef: RefObject<HTMLMediaElement | null>, source: string | undefined) {
  // The volume the studio last set; a replaced element starts back at full, so
  // the stored value is applied again on the next clip.
  const volumeRef = useRef<number | null>(null);

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
  }, [source]);
}
