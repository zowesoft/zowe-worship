import { useCallback, useEffect, useState } from 'react';
import { App } from '../../../bindings/zowe-worship';
import type { MediaView } from '../../../bindings/zowe-worship/models';
import type { Aspect, Presentation } from '../../../bindings/zowe-worship/internal/media/models';
import { FittedImage } from '../../components/FittedImage';
import { Icon } from '../../components/Icon';

function ratioOf(aspects: Aspect[], value: string): number {
  return aspects.find((aspect) => aspect.value === value)?.ratio ?? 0;
}

function describe(aspect: string, fit: string): string {
  if (aspect === 'screen') return 'filling the screen';
  if (aspect === 'auto') return fit === 'cover' ? 'own shape, filled' : 'own shape, shown whole';
  return fit === 'cover' ? `${aspect}, filled` : `${aspect}, shown whole`;
}

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

// Kept as one string so the class scanner reads it whole: a class glued to `${`
// is not picked up, which silently dropped the disabled dimming.
const chipBase = 'px-[13px] py-[7px] border border-border rounded-full bg-transparent text-secondary text-[11.5px] whitespace-nowrap hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-[0.45]';

export function MediaEditorWindow({ mediaID }: { mediaID: string }) {
  const [asset, setAsset] = useState<MediaView | null>(null);
  const [problem, setProblem] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const list = await App.ListMedia();
      const found = (list ?? []).find((item) => item.id === mediaID) ?? null;
      setAsset(found);
      if (!found) setProblem('This asset is no longer in the library.');
    } catch (error: unknown) {
      setProblem(messageOf(error));
    }
  }, [mediaID]);

  useEffect(() => {
    void load();
  }, [load]);

  if (asset === null) {
    return (
      <main className="h-full grid place-items-center pt-3.5 px-4 pb-4 overflow-hidden text-secondary text-[12px]">
        <p>{problem ?? 'Loading…'}</p>
      </main>
    );
  }

  // A still picture is edited by framing it; a clip is dressed with a picture.
  if (asset.kind === 'image') {
    return <FramingEditor asset={asset} onAsset={setAsset} />;
  }
  return <BackgroundEditor asset={asset} onAsset={setAsset} />;
}

type EditorProps = {
  asset: MediaView;
  onAsset: (asset: MediaView) => void;
};

// FramingEditor is the shape-and-edges editor a still picture gets.
function FramingEditor({ asset, onAsset }: EditorProps) {
  const [aspects, setAspects] = useState<Aspect[]>([]);
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void App.MediaAspects().then((shapes) => setAspects(shapes ?? []));
  }, []);

  async function choose(next: Presentation) {
    if (busy) return;
    setBusy(true);
    setProblem(null);
    try {
      const slide = await App.SaveMediaPresentation(asset.id, next);
      // Reframing the slide the room is looking at should change the room.
      const state = await App.GetPresentationState();
      if (state.live?.source === asset.url) await App.ProjectMedia(asset.id);
      onAsset({ ...asset, aspect: slide.aspect, fit: slide.fit });
    } catch (error: unknown) {
      setProblem(messageOf(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="h-full grid grid-rows-[auto_minmax(0,1fr)_auto] gap-3.5 pt-3.5 px-4 pb-4 overflow-hidden" aria-label={`Edit ${asset.title}`}>
      <header className="flex items-end justify-between gap-3.5 [&>div:first-child]:min-w-0 [&>div:first-child]:flex-1 [&_h2]:truncate [&_h2]:text-[17px] [&_h2]:font-semibold [&_h2]:mt-[3px]">
        <div>
          <p className="text-[9.5px] uppercase tracking-[1.3px] font-semibold text-secondary">Edit image</p>
          <h2 title={asset.title}>{asset.title}</h2>
        </div>
        <span className="shrink-0 px-2.5 py-1 border border-border rounded-full bg-elevated text-secondary text-[10px]">{describe(asset.aspect, asset.fit)}</span>
      </header>

      <section className="min-h-0 grid grid-rows-[minmax(0,1fr)_auto] gap-[7px]" aria-label="Output preview">
        {/* Flex, not grid place-items-center: a centred grid row hugs its content,
            so FittedImage's percentage max-height has nothing to resolve against
            and the picture lays out at full size and is clipped by the pane. */}
        <div className="min-h-0 flex items-center justify-center overflow-hidden border border-[#30445F] rounded-lg [background:repeating-conic-gradient(#0C1524_0%_25%,#0A1220_0%_50%)_50%_/_22px_22px]">
          <FittedImage
            src={asset.url}
            alt=""
            aspect={asset.aspect}
            ratio={ratioOf(aspects, asset.aspect)}
            fit={asset.fit === 'cover' ? 'cover' : 'contain'}
          />
        </div>
        <p className="text-[#6F809B] text-[10.5px]">How the audience screen will show it</p>
      </section>

      <section className="grid gap-3">
        <div className="[&_h3]:mb-[7px] [&_h3]:text-secondary [&_h3]:text-[9.5px] [&_h3]:tracking-[1.1px] [&_h3]:uppercase">
          <h3>Shape</h3>
          <div className="flex gap-1.5 flex-wrap">
            {aspects.map((aspect) => (
              <button
                key={aspect.value}
                type="button"
                className={`${chipBase}${asset.aspect === aspect.value ? ' border-blue! bg-selected! text-text!' : ''}`}
                aria-pressed={asset.aspect === aspect.value}
                disabled={busy}
                onClick={() => void choose({ aspect: aspect.value, fit: asset.fit })}
              >
                {aspect.label}
              </button>
            ))}
          </div>
        </div>

        <div className="[&_h3]:mb-[7px] [&_h3]:text-secondary [&_h3]:text-[9.5px] [&_h3]:tracking-[1.1px] [&_h3]:uppercase">
          <h3>Edges</h3>
          <div className="flex gap-1.5 flex-wrap">
            <button
              type="button"
              className={`${chipBase}${asset.fit !== 'cover' ? ' border-blue! bg-selected! text-text!' : ''}`}
              aria-pressed={asset.fit !== 'cover'}
              disabled={busy || asset.aspect === 'screen'}
              title="Show the whole picture, adding bars where needed"
              onClick={() => void choose({ aspect: asset.aspect, fit: 'contain' })}
            >
              Show whole picture
            </button>
            <button
              type="button"
              className={`${chipBase}${asset.fit === 'cover' ? ' border-blue! bg-selected! text-text!' : ''}`}
              aria-pressed={asset.fit === 'cover'}
              disabled={busy || asset.aspect === 'screen'}
              title="Fill the shape and crop whatever hangs over"
              onClick={() => void choose({ aspect: asset.aspect, fit: 'cover' })}
            >
              Fill and crop
            </button>
          </div>
        </div>

        <button
          type="button"
          className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950] justify-self-start"
          disabled={busy || (asset.aspect === 'auto' && asset.fit !== 'cover')}
          onClick={() => void choose({ aspect: 'auto', fit: 'contain' })}
        >
          <Icon name="close" size={13} />Back to the picture's own shape
        </button>
      </section>

      {problem && <p className="text-error text-[11.5px]" role="alert">{problem}</p>}
    </main>
  );
}

// BackgroundEditor dresses an audio or video clip with a still picture, so a
// song can fill the screen with artwork while it plays.
function BackgroundEditor({ asset, onAsset }: EditorProps) {
  const [pictures, setPictures] = useState<MediaView[]>([]);
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void App.ListMedia()
      .then((list) => setPictures((list ?? []).filter((item) => item.kind === 'image' && !item.missing)))
      .catch((error: unknown) => setProblem(messageOf(error)));
  }, []);

  async function choose(backgroundID: string) {
    if (busy) return;
    setBusy(true);
    setProblem(null);
    try {
      const slide = await App.SetMediaBackground(asset.id, backgroundID);
      // A live clip should change its dress the moment it is changed here.
      const state = await App.GetPresentationState();
      if (state.live?.itemId === asset.id) await App.ProjectMedia(asset.id);
      onAsset({ ...asset, backgroundUrl: slide.backgroundUrl ?? '' });
    } catch (error: unknown) {
      setProblem(messageOf(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="h-full grid grid-rows-[auto_minmax(0,1fr)_auto] gap-3.5 pt-3.5 px-4 pb-4 overflow-hidden" aria-label={`Edit ${asset.title}`}>
      <header className="min-w-0 [&_h2]:truncate [&_h2]:text-[17px] [&_h2]:font-semibold [&_h2]:mt-[3px]">
        <p className="text-[9.5px] uppercase tracking-[1.3px] font-semibold text-secondary">Edit {asset.kind}</p>
        <h2 title={asset.title}>{asset.title}</h2>
      </header>

      <section className="min-h-0 grid grid-rows-[auto_minmax(0,1fr)] gap-3" aria-label="Background">
        <div className="relative grid content-center justify-items-center items-center gap-2.5 aspect-[16/9] overflow-hidden rounded-lg border border-[#30445F] bg-[#0B1424]">
          {asset.backgroundUrl && <img src={asset.backgroundUrl} alt="" className="absolute inset-0 w-full h-full object-cover" />}
          {asset.backgroundUrl && <div className="absolute inset-0 bg-[rgba(4,9,18,0.45)]" />}
          <span className={`relative grid place-items-center w-11 h-11 border rounded-[11px] ${asset.backgroundUrl ? 'border-[#5E749A] bg-[rgba(11,20,36,0.72)]' : 'border-[#30466A] bg-selected'} text-[#8CABFF]`}><Icon name="music" size={22} /></span>
          <p className={`relative text-[13px] font-semibold text-center px-3 ${asset.backgroundUrl ? '[text-shadow:0_1px_8px_rgba(0,0,0,0.7)]' : 'text-secondary'}`}>{asset.title}</p>
        </div>
        <p className="text-[#6F809B] text-[10.5px]">
          {asset.backgroundUrl ? 'The picture behind the clip on the audience screen.' : 'No background yet — the clip plays on a plain dark frame.'}
        </p>

        <div className="min-h-0 overflow-y-auto pr-0.5">
          {pictures.length === 0 ? (
            <p className="py-4 text-secondary text-[11.5px] text-center">No pictures in the library yet. Add one under Media first.</p>
          ) : (
            <ul className="list-none grid grid-cols-[repeat(auto-fill,minmax(118px,1fr))] gap-2 m-0 p-0">
              {pictures.map((picture) => (
                <li key={picture.id}>
                  <button
                    type="button"
                    className={`block w-full aspect-[16/9] overflow-hidden rounded-md border bg-[#0B1424] disabled:opacity-60 ${asset.backgroundUrl === picture.url ? 'border-blue!' : 'border-border hover:not-disabled:border-[#5E749A]'}`}
                    title={`Use ${picture.title} as the background`}
                    disabled={busy}
                    onClick={() => void choose(picture.id)}
                  >
                    {picture.hasThumbnail
                      ? <img src={picture.thumbnailUrl} alt="" className="w-full h-full object-cover" />
                      : <span className="grid place-items-center w-full h-full text-secondary"><Icon name="media" size={18} /></span>}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>

      <section className="flex items-center justify-between gap-3">
        <button
          type="button"
          className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]"
          disabled={busy || !asset.backgroundUrl}
          onClick={() => void choose('')}
        >
          <Icon name="close" size={13} />No background
        </button>
        {problem && <p className="text-error text-[11.5px]" role="alert">{problem}</p>}
      </section>
    </main>
  );
}
