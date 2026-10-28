import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { main } from '../../wailsjs/go/models';
import {
  AddScriptureToService,
  ClearLive,
  CreateService,
  GetPresentationState,
  GetServiceOrder,
  NextServiceItem,
  PreviewScripture,
  PreviewServiceItem,
  PreviousServiceItem,
  ProjectScripture,
  RemoveServiceItem,
  SendPreviewLive,
} from '../../wailsjs/go/main/App.js';
import { hasDesktopBridge } from '../lib/desktop';

export type ScriptureInput = {
  translationId: string;
  bookId: number;
  chapter: number;
  verseStart: number;
  verseEnd?: number;
};

type StudioValue = {
  ready: boolean;
  bridgeAvailable: boolean;
  error: string | null;
  order: main.ServiceOrderView | null;
  presentation: main.PresentationView | null;
  createService: (name: string) => Promise<main.ServiceOrderView>;
  addScripture: (input: ScriptureInput) => Promise<main.ServiceOrderView>;
  removeItem: (itemID: string) => Promise<main.ServiceOrderView>;
  previewItem: (itemID: string) => Promise<main.PresentationView>;
  previewScripture: (input: ScriptureInput) => Promise<main.PresentationView>;
  projectScripture: (input: ScriptureInput) => Promise<main.PresentationView>;
  sendPreviewLive: () => Promise<main.PresentationView>;
  clearLive: () => Promise<main.PresentationView>;
  nextItem: () => Promise<main.PresentationView>;
  previousItem: () => Promise<main.PresentationView>;
};

const StudioContext = createContext<StudioValue | null>(null);

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

function toRequest(input: ScriptureInput): main.ScriptureRequest {
  return new main.ScriptureRequest({
    translationId: input.translationId,
    bookId: input.bookId,
    chapter: input.chapter,
    verseStart: input.verseStart,
    verseEnd: input.verseEnd ?? input.verseStart,
  });
}

export function StudioProvider({ children }: { children: ReactNode }) {
  const bridgeAvailable = hasDesktopBridge();
  const [ready, setReady] = useState(false);
  const [error, setError] = useState<string | null>(
    bridgeAvailable ? null : 'Open the Zowe Worship desktop app to run a service.',
  );
  const [order, setOrder] = useState<main.ServiceOrderView | null>(null);
  const [presentationState, setPresentationState] = useState<main.PresentationView | null>(null);

  useEffect(() => {
    if (!bridgeAvailable) return;

    let active = true;
    Promise.all([GetServiceOrder(), GetPresentationState()])
      .then(([initialOrder, initialPresentation]) => {
        if (!active) return;
        setOrder(initialOrder);
        setPresentationState(initialPresentation);
      })
      .catch((cause: unknown) => {
        if (active) setError(errorMessage(cause));
      })
      .finally(() => {
        if (active) setReady(true);
      });

    return () => {
      active = false;
    };
  }, [bridgeAvailable]);

  const run = useCallback(async <T,>(action: () => Promise<T>, apply: (value: T) => void): Promise<T> => {
    try {
      const value = await action();
      apply(value);
      setError(null);
      return value;
    } catch (cause: unknown) {
      const message = errorMessage(cause);
      setError(message);
      throw new Error(message);
    }
  }, []);

  const value = useMemo<StudioValue>(() => ({
    ready,
    bridgeAvailable,
    error,
    order,
    presentation: presentationState,
    createService: (name) => run(() => CreateService(name), setOrder),
    addScripture: (input) => run(() => AddScriptureToService(toRequest(input)), setOrder),
    removeItem: (itemID) => run(() => RemoveServiceItem(itemID), setOrder),
    previewItem: (itemID) => run(() => PreviewServiceItem(itemID), setPresentationState),
    previewScripture: (input) => run(() => PreviewScripture(toRequest(input)), setPresentationState),
    projectScripture: (input) => run(() => ProjectScripture(toRequest(input)), setPresentationState),
    sendPreviewLive: () => run(SendPreviewLive, setPresentationState),
    clearLive: () => run(ClearLive, setPresentationState),
    nextItem: () => run(NextServiceItem, setPresentationState),
    previousItem: () => run(PreviousServiceItem, setPresentationState),
  }), [bridgeAvailable, error, order, presentationState, ready, run]);

  return <StudioContext.Provider value={value}>{children}</StudioContext.Provider>;
}

export function useStudio(): StudioValue {
  const context = useContext(StudioContext);
  if (!context) {
    throw new Error('useStudio must be used inside StudioProvider.');
  }
  return context;
}
