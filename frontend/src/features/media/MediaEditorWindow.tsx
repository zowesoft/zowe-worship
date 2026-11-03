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

// Kept as one string so the class scanner reads it whole: a class glued to `${`
// is not picked up, which silently dropped the disabled dimming.
const chipBase = 'px-[13px] py-[7px] border border-border rounded-full bg-transparent text-secondary text-[11.5px] whitespace-nowrap hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-[0.45]';

export function MediaEditorWindow({ mediaID }: { mediaID: string }) {
  const [aspects, setAspects] = useState<Aspect[]>([]);
  const [asset, setAsset] = useState<MediaView | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setProblem(null);
    try {
      const [list, shapes] = await Promise.all([App.ListMedia(), App.MediaAspects()]);
      setAspects(shapes ?? []);
      const found = (list ?? []).find((item) => item.id === mediaID) ?? null;
      setAsset(found);
      if (!found) setProblem('This image is no longer in the library.');
    } catch (error: unknown) {
      setProblem(error instanceof Error ? error.message : String(error));
    }
  }, [mediaID]);

  useEffect(() => {
    void load();
  }, [load]);

  async function choose(next: Presentation) {
    if (!asset || busy) return;
    setBusy(true);
    setProblem(null);
    try {
      const slide = await App.SaveMediaPresentation(asset.id, next);
      // Reframing the slide the room is looking at should change the room.
      const state = await App.GetPresentationState();
      if (state.live?.source === asset.url) await App.ProjectMedia(asset.id);
      setAsset({ ...asset, aspect: slide.aspect, fit: slide.fit });
    } catch (error: unknown) {
      setProblem(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  if (asset === null) {
    return (
      <main className="h-full grid grid-rows-[auto_minmax(0,1fr)_auto] gap-3.5 pt-3.5 px-4 pb-4 overflow-hidden place-items-center text-secondary text-[12px]">
        <p>{problem ?? 'Loading…'}</p>
      </main>
    );
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
        <div className="min-h-0 grid place-items-center overflow-hidden border border-[#30445F] rounded-lg [background:repeating-conic-gradient(#0C1524_0%_25%,#0A1220_0%_50%)_50%_/_22px_22px]">
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
