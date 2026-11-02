import { useCallback, useEffect, useState } from 'react';
import { App } from '../../../bindings/zowe-worship';
import type { MediaView } from '../../../bindings/zowe-worship/models';
import type { Aspect, Presentation } from '../../../bindings/zowe-worship/internal/media/models';
import { FittedImage } from '../../components/FittedImage';
import { Icon } from '../../components/Icon';
import './editor.css';

function ratioOf(aspects: Aspect[], value: string): number {
  return aspects.find((aspect) => aspect.value === value)?.ratio ?? 0;
}

function describe(aspect: string, fit: string): string {
  if (aspect === 'screen') return 'filling the screen';
  if (aspect === 'auto') return fit === 'cover' ? 'own shape, filled' : 'own shape, shown whole';
  return fit === 'cover' ? `${aspect}, filled` : `${aspect}, shown whole`;
}

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
      <main className="editor editor--empty">
        <p>{problem ?? 'Loading…'}</p>
      </main>
    );
  }

  return (
    <main className="editor" aria-label={`Edit ${asset.title}`}>
      <header className="editor-header">
        <div>
          <p className="eyebrow">Edit image</p>
          <h2 title={asset.title}>{asset.title}</h2>
        </div>
        <span className="editor-state">{describe(asset.aspect, asset.fit)}</span>
      </header>

      <section className="editor-preview" aria-label="Output preview">
        <div className="editor-output">
          <FittedImage
            src={asset.url}
            alt=""
            aspect={asset.aspect}
            ratio={ratioOf(aspects, asset.aspect)}
            fit={asset.fit === 'cover' ? 'cover' : 'contain'}
          />
        </div>
        <p className="editor-caption">How the audience screen will show it</p>
      </section>

      <section className="editor-controls">
        <div className="editor-group">
          <h3>Shape</h3>
          <div className="editor-chips">
            {aspects.map((aspect) => (
              <button
                key={aspect.value}
                type="button"
                className={`editor-chip${asset.aspect === aspect.value ? ' editor-chip--active' : ''}`}
                aria-pressed={asset.aspect === aspect.value}
                disabled={busy}
                onClick={() => void choose({ aspect: aspect.value, fit: asset.fit })}
              >
                {aspect.label}
              </button>
            ))}
          </div>
        </div>

        <div className="editor-group">
          <h3>Edges</h3>
          <div className="editor-chips">
            <button
              type="button"
              className={`editor-chip${asset.fit !== 'cover' ? ' editor-chip--active' : ''}`}
              aria-pressed={asset.fit !== 'cover'}
              disabled={busy || asset.aspect === 'screen'}
              title="Show the whole picture, adding bars where needed"
              onClick={() => void choose({ aspect: asset.aspect, fit: 'contain' })}
            >
              Show whole picture
            </button>
            <button
              type="button"
              className={`editor-chip${asset.fit === 'cover' ? ' editor-chip--active' : ''}`}
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
          className="button button--small editor-reset"
          disabled={busy || (asset.aspect === 'auto' && asset.fit !== 'cover')}
          onClick={() => void choose({ aspect: 'auto', fit: 'contain' })}
        >
          <Icon name="close" size={13} />Back to the picture's own shape
        </button>
      </section>

      {problem && <p className="editor-error" role="alert">{problem}</p>}
    </main>
  );
}
