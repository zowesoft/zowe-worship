import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { main } from '../../wailsjs/go/models';
import {
  AddScriptureToService,
  AddSongSectionToService,
  ClearLive,
  CreateService,
  CreateSong,
  DeleteSong,
  GetPresentationState,
  GetServiceOrder,
  ListServices,
  ListSongs,
  NextServiceItem,
  PreviewScripture,
  PreviewServiceItem,
  PreviewSongSection,
  PreviousServiceItem,
  ProjectScripture,
  ProjectSongSection,
  RemoveServiceItem,
  RenameService,
  ReorderServiceItems,
  SendPreviewLive,
  UseService,
} from '../../wailsjs/go/main/App.js';
import { hasDesktopBridge } from '../lib/desktop';

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
  order: main.ServiceOrderView | null;
  services: main.ServiceView[];
  presentation: main.PresentationView | null;
  songs: main.SongsView | null;
  refreshSongs: (query?: string) => Promise<void>;
  createService: (name: string) => Promise<main.ServiceOrderView>;
  useService: (serviceID: string) => Promise<main.ServiceOrderView>;
  renameService: (serviceID: string, name: string) => Promise<main.ServiceOrderView>;
  addScripture: (input: ScriptureInput) => Promise<main.ServiceOrderView>;
  removeItem: (itemID: string) => Promise<main.ServiceOrderView>;
  reorderItems: (itemIDs: string[]) => Promise<main.ServiceOrderView>;
  previewItem: (itemID: string) => Promise<main.PresentationView>;
  previewScripture: (input: ScriptureInput) => Promise<main.PresentationView>;
  projectScripture: (input: ScriptureInput) => Promise<main.PresentationView>;
  createSong: (input: NewSongInput) => Promise<main.SongsView>;
  deleteSong: (songID: string) => Promise<main.SongsView>;
  previewSongSection: (input: SongSectionInput) => Promise<main.PresentationView>;
  projectSongSection: (input: SongSectionInput) => Promise<main.PresentationView>;
  addSongSectionToService: (input: SongSectionInput) => Promise<main.ServiceOrderView>;
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

function toSectionRequest(input: SongSectionInput): main.SongSectionRequest {
  return new main.SongSectionRequest({ songId: input.songId, sectionId: input.sectionId });
}

function toSongRequest(input: NewSongInput): main.NewSongRequest {
  return new main.NewSongRequest({
    title: input.title,
    author: input.author,
    copyright: input.copyright,
    sections: input.sections.map(
      (section) => new main.NewSongSectionRequest({ label: section.label, text: section.text }),
    ),
  });
}

export function StudioProvider({ children }: { children: ReactNode }) {
  const bridgeAvailable = hasDesktopBridge();
  const [ready, setReady] = useState(false);
  const [error, setError] = useState<string | null>(
    bridgeAvailable ? null : 'Open the Zowe Worship desktop app to run a service.',
  );
  const [order, setOrder] = useState<main.ServiceOrderView | null>(null);
  const [services, setServices] = useState<main.ServiceView[]>([]);
  const [presentationState, setPresentationState] = useState<main.PresentationView | null>(null);
  const [songs, setSongs] = useState<main.SongsView | null>(null);

  useEffect(() => {
    if (!bridgeAvailable) return;

    let active = true;
    Promise.all([GetServiceOrder(), GetPresentationState(), ListSongs(''), ListServices()])
      .then(([initialOrder, initialPresentation, initialSongs, initialServices]) => {
        if (!active) return;
        setOrder(initialOrder);
        setPresentationState(initialPresentation);
        setSongs(initialSongs);
        setServices(initialServices);
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

  // Every service mutation can add or rename a row in the switcher, so the
  // list is reread alongside the schedule instead of patched in the client.
  const orderWithList = useCallback(async (action: () => Promise<main.ServiceOrderView>) => {
    const value = await action();
    setServices(await ListServices());
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
      await run(() => ListSongs(query), setSongs);
    },
    createService: (name) => run(() => orderWithList(() => CreateService(name)), setOrder),
    useService: (serviceID) => run(() => orderWithList(() => UseService(serviceID)), setOrder),
    renameService: (serviceID, name) => run(() => orderWithList(() => RenameService(serviceID, name)), setOrder),
    addScripture: (input) => run(() => AddScriptureToService(toRequest(input)), setOrder),
    removeItem: (itemID) => run(() => RemoveServiceItem(itemID), setOrder),
    reorderItems: (itemIDs) => run(() => ReorderServiceItems(itemIDs), setOrder),
    previewItem: (itemID) => run(() => PreviewServiceItem(itemID), setPresentationState),
    previewScripture: (input) => run(() => PreviewScripture(toRequest(input)), setPresentationState),
    projectScripture: (input) => run(() => ProjectScripture(toRequest(input)), setPresentationState),
    createSong: (input) => run(() => CreateSong(toSongRequest(input)), setSongs),
    deleteSong: (songID) => run(() => DeleteSong(songID), setSongs),
    previewSongSection: (input) => run(() => PreviewSongSection(toSectionRequest(input)), setPresentationState),
    projectSongSection: (input) => run(() => ProjectSongSection(toSectionRequest(input)), setPresentationState),
    addSongSectionToService: (input) => run(() => AddSongSectionToService(toSectionRequest(input)), setOrder),
    sendPreviewLive: () => run(SendPreviewLive, setPresentationState),
    clearLive: () => run(ClearLive, setPresentationState),
    nextItem: () => run(NextServiceItem, setPresentationState),
    previousItem: () => run(PreviousServiceItem, setPresentationState),
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
