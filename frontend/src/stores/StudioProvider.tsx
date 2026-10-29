import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { App } from '../../bindings/zowe-worship';
import * as models from '../../bindings/zowe-worship/models';

export type ScriptureInput = {
  translationId: string;
  bookId: number;
  chapter: number;
  verseStart: number;
  verseEnd?: number;
};

export type SongSectionInput = {
  songId: string;
  sectionId: string;
};

export type NewSongInput = {
  title: string;
  author: string;
  copyright: string;
  sections: { label: string; text: string }[];
};

type StudioValue = {
  ready: boolean;
  bridgeAvailable: boolean;
  error: string | null;
  order: models.ServiceOrderView | null;
  services: models.ServiceView[];
  presentation: models.PresentationView | null;
  songs: models.SongsView | null;
  refreshSongs: (query?: string) => Promise<void>;
  createService: (name: string) => Promise<models.ServiceOrderView>;
  useService: (serviceID: string) => Promise<models.ServiceOrderView>;
  renameService: (serviceID: string, name: string) => Promise<models.ServiceOrderView>;
  addScripture: (input: ScriptureInput) => Promise<models.ServiceOrderView>;
  removeItem: (itemID: string) => Promise<models.ServiceOrderView>;
  reorderItems: (itemIDs: string[]) => Promise<models.ServiceOrderView>;
  previewItem: (itemID: string) => Promise<models.PresentationView>;
  previewScripture: (input: ScriptureInput) => Promise<models.PresentationView>;
  projectScripture: (input: ScriptureInput) => Promise<models.PresentationView>;
  createSong: (input: NewSongInput) => Promise<models.SongsView>;
  deleteSong: (songID: string) => Promise<models.SongsView>;
  previewSongSection: (input: SongSectionInput) => Promise<models.PresentationView>;
  projectSongSection: (input: SongSectionInput) => Promise<models.PresentationView>;
  addSongSectionToService: (input: SongSectionInput) => Promise<models.ServiceOrderView>;
  sendPreviewLive: () => Promise<models.PresentationView>;
  clearLive: () => Promise<models.PresentationView>;
  nextItem: () => Promise<models.PresentationView>;
  previousItem: () => Promise<models.PresentationView>;
};

const StudioContext = createContext<StudioValue | null>(null);

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

function toRequest(input: ScriptureInput): models.ScriptureRequest {
  return {
    translationId: input.translationId,
    bookId: input.bookId,
    chapter: input.chapter,
    verseStart: input.verseStart,
    verseEnd: input.verseEnd ?? input.verseStart,
  };
}

function toSectionRequest(input: SongSectionInput): models.SongSectionRequest {
  return { songId: input.songId, sectionId: input.sectionId };
}

function toSongRequest(input: NewSongInput): models.NewSongRequest {
  return {
    title: input.title,
    author: input.author,
    copyright: input.copyright,
    sections: input.sections.map((section) => ({ label: section.label, text: section.text })),
  };
}

export function StudioProvider({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false);
  const [bridgeAvailable, setBridgeAvailable] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [order, setOrder] = useState<models.ServiceOrderView | null>(null);
  const [services, setServices] = useState<models.ServiceView[]>([]);
  const [presentationState, setPresentationState] = useState<models.PresentationView | null>(null);
  const [songs, setSongs] = useState<models.SongsView | null>(null);

  // v3 has no `window.go` to sniff, so the bridge is whatever the first load
  // proves: opening in a plain browser rejects every call and the studio stays
  // in its read-only state.
  useEffect(() => {
    let active = true;

    Promise.all([App.GetServiceOrder(), App.GetPresentationState(), App.ListSongs(''), App.ListServices()])
      .then(([initialOrder, initialPresentation, initialSongs, initialServices]) => {
        if (!active) return;
        setBridgeAvailable(true);
        setOrder(initialOrder);
        setPresentationState(initialPresentation);
        setSongs(initialSongs);
        setServices(initialServices ?? []);
      })
      .catch((cause: unknown) => {
        if (!active) return;
        setError(`${errorMessage(cause)} — open the Zowe Worship desktop app to run a service.`);
      })
      .finally(() => {
        if (active) setReady(true);
      });

    return () => {
      active = false;
    };
  }, []);

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

  // Every service mutation can add or rename a row in the switcher, so the
  // list is reread alongside the schedule instead of patched in the client.
  const orderWithList = useCallback(async (action: () => Promise<models.ServiceOrderView>) => {
    const value = await action();
    setServices((await App.ListServices()) ?? []);
    return value;
  }, []);

  const value = useMemo<StudioValue>(() => ({
    ready,
    bridgeAvailable,
    error,
    order,
    services,
    presentation: presentationState,
    songs,
    refreshSongs: async (query = '') => {
      await run(() => App.ListSongs(query), setSongs);
    },
    createService: (name) => run(() => orderWithList(() => App.CreateService(name)), setOrder),
    useService: (serviceID) => run(() => orderWithList(() => App.UseService(serviceID)), setOrder),
    renameService: (serviceID, name) => run(() => orderWithList(() => App.RenameService(serviceID, name)), setOrder),
    addScripture: (input) => run(() => App.AddScriptureToService(toRequest(input)), setOrder),
    removeItem: (itemID) => run(() => App.RemoveServiceItem(itemID), setOrder),
    reorderItems: (itemIDs) => run(() => App.ReorderServiceItems(itemIDs), setOrder),
    previewItem: (itemID) => run(() => App.PreviewServiceItem(itemID), setPresentationState),
    previewScripture: (input) => run(() => App.PreviewScripture(toRequest(input)), setPresentationState),
    projectScripture: (input) => run(() => App.ProjectScripture(toRequest(input)), setPresentationState),
    createSong: (input) => run(() => App.CreateSong(toSongRequest(input)), setSongs),
    deleteSong: (songID) => run(() => App.DeleteSong(songID), setSongs),
    previewSongSection: (input) => run(() => App.PreviewSongSection(toSectionRequest(input)), setPresentationState),
    projectSongSection: (input) => run(() => App.ProjectSongSection(toSectionRequest(input)), setPresentationState),
    addSongSectionToService: (input) => run(() => App.AddSongSectionToService(toSectionRequest(input)), setOrder),
    sendPreviewLive: () => run(App.SendPreviewLive, setPresentationState),
    clearLive: () => run(App.ClearLive, setPresentationState),
    nextItem: () => run(App.NextServiceItem, setPresentationState),
    previousItem: () => run(App.PreviousServiceItem, setPresentationState),
  }), [bridgeAvailable, error, order, orderWithList, presentationState, ready, run, services, songs]);

  return <StudioContext.Provider value={value}>{children}</StudioContext.Provider>;
}

export function useStudio(): StudioValue {
  const context = useContext(StudioContext);
  if (!context) {
    throw new Error('useStudio must be used inside StudioProvider.');
  }
  return context;
}
