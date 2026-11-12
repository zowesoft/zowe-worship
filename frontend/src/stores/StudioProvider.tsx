import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { Events } from '@wailsio/runtime';
import { App } from '../../bindings/zowe-worship';
import * as models from '../../bindings/zowe-worship/models';
import type { Aspect } from '../../bindings/zowe-worship/internal/media/models';

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

// MediaAction is the transport vocabulary the studio and the audience player
// share. Seek takes a fraction of the clip; volume is a percentage.
export type MediaAction = 'play' | 'pause' | 'restart' | 'stop' | 'seek' | 'volume';

type StudioValue = {
  ready: boolean;
  bridgeAvailable: boolean;
  error: string | null;
  order: models.ServiceOrderView | null;
  services: models.ServiceView[];
  presentation: models.PresentationView | null;
  display: models.DisplayView | null;
  screens: models.ScreenOption[];
  songs: models.SongsView | null;
  media: models.MediaView[];
  mediaAspects: Aspect[];
  importResult: models.MediaImportResult | null;
  refreshMedia: () => Promise<void>;
  importMedia: (paths: string[]) => Promise<models.MediaImportResult>;
  previewMedia: (mediaID: string) => Promise<models.PresentationView>;
  projectMedia: (mediaID: string) => Promise<models.PresentationView>;
  addMediaToService: (mediaID: string) => Promise<models.ServiceOrderView>;
  deleteMedia: (id: string) => Promise<void>;
  relinkMedia: (mediaID: string, path: string) => Promise<void>;
  openDisplay: (screenID: string) => Promise<models.DisplayView>;
  closeDisplay: () => Promise<models.DisplayView>;
  // Reconnect puts the remembered output back after the window died or the
  // monitor blipped; refreshScreens rereads the picker so a replugged
  // display shows up without restarting the studio.
  reconnectDisplay: () => Promise<models.DisplayView>;
  refreshScreens: () => Promise<void>;
  openSettings: () => Promise<models.EditorView>;
  openThemes: () => Promise<models.EditorView>;
  mediaControl: (action: MediaAction, value?: number) => Promise<void>;
  refreshSongs: (query?: string) => Promise<void>;
  createService: (name: string) => Promise<models.ServiceOrderView>;
  duplicateService: (serviceID: string, newName: string) => Promise<models.ServiceOrderView>;
  useService: (serviceID: string) => Promise<models.ServiceOrderView>;
  renameService: (serviceID: string, name: string) => Promise<models.ServiceOrderView>;
  addScripture: (input: ScriptureInput) => Promise<models.ServiceOrderView>;
  addCustomItem: (req: models.CustomItemRequest) => Promise<models.ServiceOrderView>;
  updateItemMeta: (itemID: string, notes: string, duration: number) => Promise<models.ServiceOrderView>;
  removeItem: (itemID: string) => Promise<models.ServiceOrderView>;
  reorderItems: (itemIDs: string[]) => Promise<models.ServiceOrderView>;
  setItemTheme: (itemID: string, themeID: string) => Promise<models.ServiceOrderView>;
  previewItem: (itemID: string) => Promise<models.PresentationView>;
  previewScripture: (input: ScriptureInput) => Promise<models.PresentationView>;
  projectScripture: (input: ScriptureInput) => Promise<models.PresentationView>;
  approveDetection: (id: string) => Promise<models.PresentationView>;
  projectDetection: (id: string) => Promise<models.PresentationView>;
  deleteSong: (songID: string) => Promise<models.SongsView>;
  duplicateSong: (songID: string) => Promise<models.SongsView>;
  previewSongSection: (input: SongSectionInput) => Promise<models.PresentationView>;
  projectSongSection: (input: SongSectionInput) => Promise<models.PresentationView>;
  addSongSectionToService: (input: SongSectionInput) => Promise<models.ServiceOrderView>;
  addSongArrangementToService: (songID: string, arrangementID: string) => Promise<models.ServiceOrderView>;
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

export function StudioProvider({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false);
  const [bridgeAvailable, setBridgeAvailable] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [order, setOrder] = useState<models.ServiceOrderView | null>(null);
  const [services, setServices] = useState<models.ServiceView[]>([]);
  const [presentationState, setPresentationState] = useState<models.PresentationView | null>(null);
  const [display, setDisplay] = useState<models.DisplayView | null>(null);
  const [screens, setScreens] = useState<models.ScreenOption[]>([]);
  const [songs, setSongs] = useState<models.SongsView | null>(null);
  const [media, setMedia] = useState<models.MediaView[]>([]);
  const [mediaAspects, setMediaAspects] = useState<Aspect[]>([]);
  const [importResult, setImportResult] = useState<models.MediaImportResult | null>(null);

  // v3 has no `window.go` to sniff, so the first load proves the bridge: in a
  // plain browser every call rejects and the studio stays read-only.
  useEffect(() => {
    let active = true;

    Promise.all([
      App.GetServiceOrder(),
      App.GetPresentationState(),
      App.ListSongs(''),
      App.ListServices(),
      App.ListScreens(),
      App.GetAudienceDisplay(),
      App.ListMedia(),
      App.MediaAspects(),
    ])
      .then(([
        initialOrder, initialPresentation, initialSongs, initialServices,
        initialScreens, initialDisplay, initialMedia, initialAspects,
      ]) => {
        if (!active) return;
        setBridgeAvailable(true);
        setOrder(initialOrder);
        setPresentationState(initialPresentation);
        setSongs(initialSongs);
        setServices(initialServices ?? []);
        setScreens(initialScreens ?? []);
        setDisplay(initialDisplay);
        setMedia(initialMedia ?? []);
        setMediaAspects(initialAspects ?? []);
      })
      .catch((cause: unknown) => {
        if (!active) return;
        setError(`${errorMessage(cause)} — open the Zowe Worship desktop app to run a service.`);
      })
      .finally(() => {
        if (active) setReady(true);
        // The splash holds the wall until the first data lands, then gets out
        // of the way on the clean paint.
        document.getElementById('splash')?.remove();
      });

    return () => {
      active = false;
    };
  }, []);

  // Anything that changes the slide pair republishes it, including a theme
  // edited in its own window, so the studio never shows a stale preview.
  useEffect(() => Events.On('presentation', (event) => {
    setPresentationState(event.data as models.PresentationView);
  }), []);

  // The display pushes its own state, so the selector and the live chip
  // follow a projector window that was closed on the far end without any
  // polling - the window handle is gone, the operator's choice is not.
  useEffect(() => Events.On('display', (event) => {
    setDisplay(event.data as models.DisplayView);
  }), []);

  // Records the failure for the error area and rethrows, so a caller that
  // swallows it still leaves the message visible.
  const run = useCallback(async <T,>(action: () => Promise<T>, apply?: (value: T) => void): Promise<T> => {
    try {
      const value = await action();
      apply?.(value);
      setError(null);
      return value;
    } catch (cause: unknown) {
      const message = errorMessage(cause);
      setError(message);
      throw new Error(message);
    }
  }, []);

  // A mutation can add or rename a row in the switcher, so the list is reread
  // rather than patched in the client.
  const orderWithList = useCallback(async (action: () => Promise<models.ServiceOrderView>) => {
    const value = await action();
    setServices((await App.ListServices()) ?? []);
    return value;
  }, []);

  // Reread the grid after every change so a rejected import still shows what
  // the library actually holds.
  const mediaAction = useCallback(async <T,>(action: () => Promise<T>): Promise<T> => {
    const value = await action();
    setMedia((await App.ListMedia()) ?? []);
    return value;
  }, []);

  const importMedia = useCallback(async (paths: string[]) => {
    const result = await run(() => mediaAction(() => App.ImportMedia(paths)));
    setImportResult(result);
    return result;
  }, [mediaAction, run]);

  const value = useMemo<StudioValue>(() => ({
    ready,
    bridgeAvailable,
    error,
    order,
    services,
    presentation: presentationState,
    display,
    screens,
    songs,
    media,
    mediaAspects,
    importResult,
    refreshMedia: async () => {
      await run(() => mediaAction(App.ListMedia));
    },
    importMedia,
    previewMedia: (mediaID) => run(() => App.PreviewMedia(mediaID), setPresentationState),
    projectMedia: (mediaID) => run(() => App.ProjectMedia(mediaID), setPresentationState),
    addMediaToService: (mediaID) => run(() => App.AddMediaToService(mediaID), setOrder),
    deleteMedia: async (id) => {
      await run(() => App.DeleteMedia(id), (list) => setMedia(list ?? []));
    },
    relinkMedia: async (mediaID, path) => {
      await run(() => App.RelinkMedia(mediaID, path), (list) => setMedia(list ?? []));
    },
    openDisplay: (screenID) => run(() => App.OpenAudienceDisplay(screenID), setDisplay),
    closeDisplay: () => run(() => App.CloseAudienceDisplay(), setDisplay),
    reconnectDisplay: () => run(() => App.ReconnectAudienceDisplay(), setDisplay),
    refreshScreens: async () => {
      await run(() => App.ListScreens(), (list) => setScreens(list ?? []));
    },
    openSettings: () => run(() => App.OpenSettings()),
    openThemes: () => run(() => App.OpenThemeWindow()),
    mediaControl: (action, value) => run(() => App.MediaControl({ action, value: value ?? 0 })),
    refreshSongs: async (query = '') => {
      await run(() => App.ListSongs(query), setSongs);
    },
    createService: (name) => run(() => orderWithList(() => App.CreateService(name)), setOrder),
    duplicateService: (serviceID, newName) => run(() => orderWithList(() => App.DuplicateService(serviceID, newName)), setOrder),
    useService: (serviceID) => run(() => orderWithList(() => App.UseService(serviceID)), setOrder),
    renameService: (serviceID, name) => run(() => orderWithList(() => App.RenameService(serviceID, name)), setOrder),
    addScripture: (input) => run(() => App.AddScriptureToService(toRequest(input)), setOrder),
    addCustomItem: (req) => run(() => App.AddCustomItemToService(req), setOrder),
    updateItemMeta: (itemID, notes, duration) => run(() => App.UpdateServiceItemMeta(itemID, notes, duration), setOrder),
    removeItem: (itemID) => run(() => App.RemoveServiceItem(itemID), setOrder),
    reorderItems: (itemIDs) => run(() => App.ReorderServiceItems(itemIDs), setOrder),
    setItemTheme: (itemID, themeID) => run(() => App.SetItemTheme(itemID, themeID), setOrder),
    previewItem: (itemID) => run(() => App.PreviewServiceItem(itemID), setPresentationState),
    previewScripture: (input) => run(() => App.PreviewScripture(toRequest(input)), setPresentationState),
    projectScripture: (input) => run(() => App.ProjectScripture(toRequest(input)), setPresentationState),
    approveDetection: (id) => run(() => App.ApproveDetection(id), setPresentationState),
    projectDetection: (id) => run(() => App.ProjectDetection(id), setPresentationState),
    deleteSong: (songID) => run(() => App.DeleteSong(songID), setSongs),
    duplicateSong: (songID) => run(() => App.DuplicateSong(songID), setSongs),
    previewSongSection: (input) => run(() => App.PreviewSongSection(toSectionRequest(input)), setPresentationState),
    projectSongSection: (input) => run(() => App.ProjectSongSection(toSectionRequest(input)), setPresentationState),
    addSongSectionToService: (input) => run(() => App.AddSongSectionToService(toSectionRequest(input)), setOrder),
    addSongArrangementToService: (songID, arrangementID) => run(
      () => App.AddSongArrangementToService({ songId: songID, arrangementId: arrangementID }),
      setOrder,
    ),
    sendPreviewLive: () => run(App.SendPreviewLive, setPresentationState),
    clearLive: () => run(App.ClearLive, setPresentationState),
    nextItem: () => run(App.NextServiceItem, setPresentationState),
    previousItem: () => run(App.PreviousServiceItem, setPresentationState),
  }), [bridgeAvailable, display, error, importMedia, importResult, media, mediaAction, mediaAspects, order, orderWithList, presentationState, ready, run, screens, services, songs]);

  return <StudioContext.Provider value={value}>{children}</StudioContext.Provider>;
}

export function useStudio(): StudioValue {
  const context = useContext(StudioContext);
  if (!context) {
    throw new Error('useStudio must be used inside StudioProvider.');
  }
  return context;
}
