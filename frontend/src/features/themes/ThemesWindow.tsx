import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { Window } from '@wailsio/runtime';
import { App } from '../../../bindings/zowe-worship';
import type { MediaView } from '../../../bindings/zowe-worship/models';
import type { FontFamily, Theme } from '../../../bindings/zowe-worship/internal/themes/models';
import { BackgroundKind, Kind } from '../../../bindings/zowe-worship/internal/themes/models';
import { Icon } from '../../components/Icon';
import './themes-window.css';

const kinds: { value: Kind; label: string }[] = [
  { value: Kind.KindScripture, label: 'Scripture' },
  { value: Kind.KindLyrics, label: 'Lyrics' },
  { value: Kind.KindText, label: 'Text' },
];

const backgrounds: { value: BackgroundKind; label: string }[] = [
  { value: BackgroundKind.BackgroundSolid, label: 'Colour' },
  { value: BackgroundKind.BackgroundGradient, label: 'Gradient' },
  { value: BackgroundKind.BackgroundImage, label: 'Picture' },
];

const alignments = ['left', 'center', 'right'];
const references = [
  { value: 'none', label: 'Hidden' },
  { value: 'top', label: 'Top' },
  { value: 'bottom', label: 'Bottom' },
];

// The words a theme is judged on. The long form is what the overflow warning
// measures, because a theme that fits one verse is no use for a whole chapter.
const samples: Record<string, { lines: string[]; reference: string }> = {
  scripture: {
    lines: [
      'The Lord is my shepherd;',
      'I shall not want.',
      'He maketh me to lie down in green pastures:',
      'he leadeth me beside the still waters.',
      'He restoreth my soul',
      'he leadeth me in the paths of righteousness',
      'for his name’s sake.',
    ],
    reference: 'Psalm 23:1–3',
  },
  lyrics: {
    lines: [
      'Amazing grace! How sweet the sound',
      'That saved a wretch like me!',
      'I once was lost, but now am found;',
      'Was blind, but now I see.',
    ],
    reference: 'Amazing Grace · Verse 1',
  },
  text: {
    lines: [
      'No evening service this Wednesday.',
      'The hall is closed for the wedding',
      'and the coffee morning moves to Saturday.',
    ],
    reference: 'Notice',
  },
};

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

function Chips({ label, value, options, onPick }: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onPick: (value: string) => void;
}) {
  return (
    <div className="theme-field">
      <span className="theme-field-label">{label}</span>
      <div className="theme-chips">
        {options.map((option) => (
          <button
            key={option.value}
            type="button"
            className={`theme-chip${option.value === value ? ' theme-chip--on' : ''}`}
            onClick={() => onPick(option.value)}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}

function Slider({ label, value, min, max, step, unit, onPick }: {
  label: string;
  value: number;
  min: number;
  max: number;
  step: number;
  unit: string;
  onPick: (value: number) => void;
}) {
  return (
    <label className="theme-field">
      <span className="theme-field-label">
        {label}
        <em>{value}{unit}</em>
      </span>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(event) => onPick(Number(event.target.value))}
      />
    </label>
  );
}

function Colour({ label, value, onPick }: {
  label: string;
  value: string;
  onPick: (value: string) => void;
}) {
  return (
    <label className="theme-field theme-field--colour">
      <span className="theme-field-label">{label}</span>
      <input type="color" value={value} onChange={(event) => onPick(event.target.value)} />
    </label>
  );
}

export function ThemesWindow() {
  const [themes, setThemes] = useState<Theme[]>([]);
  const [fonts, setFonts] = useState<FontFamily[]>([]);
  const [images, setImages] = useState<MediaView[]>([]);
  const [draft, setDraft] = useState<Theme | null>(null);
  const [isNew, setIsNew] = useState(false);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [overflow, setOverflow] = useState(false);
  const linesRef = useRef<HTMLParagraphElement>(null);

  useEffect(() => {
    Promise.all([App.ListThemes(), App.ThemeFonts(), App.ListMedia(), App.NewTheme(Kind.KindScripture)])
      .then(([stored, families, media, starter]) => {
        setThemes(stored ?? []);
        setFonts(families ?? []);
        setImages((media ?? []).filter((item) => item.kind === 'image' && !item.missing));
        setIsNew(true);
        setDraft(starter);
      })
      .catch((error: unknown) => setProblem(errorMessage(error)));
  }, []);

  const sample = samples[draft?.kind ?? 'scripture'] ?? samples.scripture;

  // A theme is only readable if the longest passage it will be given still fits.
  // The sample body is the bounded box, so the text inside it can be measured
  // against the room the slide actually has.
  useLayoutEffect(() => {
    const element = linesRef.current;
    const box = element?.parentElement;
    if (!element || !box) return;
    setOverflow(element.offsetHeight > box.clientHeight + 1);
  }, [draft, sample]);

  function patch(change: Partial<Theme>) {
    setDraft((current) => (current ? { ...current, ...change } : current));
  }
  function patchStyle(change: Partial<Theme['style']>) {
    setDraft((current) => (current ? { ...current, style: { ...current.style, ...change } } : current));
  }
  function patchBackground(change: Partial<Theme['background']>) {
    setDraft((current) => (current ? { ...current, background: { ...current.background, ...change } } : current));
  }

  async function save() {
    if (!draft) return;
    setBusy(true);
    setProblem(null);
    try {
      const stored = isNew
        ? await App.CreateTheme(draft)
        : await App.UpdateTheme(draft);
      const list = stored ?? [];
      setThemes(list);
      const saved = list.find((theme) => theme.name === draft.name && theme.kind === draft.kind);
      if (saved) setDraft(saved);
      setIsNew(false);
      setNotice(isNew ? 'Theme created.' : 'Theme saved.');
    } catch (error: unknown) {
      setProblem(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  function pick(theme: Theme) {
    setProblem(null);
    setNotice(null);
    setIsNew(false);
    setDraft(theme);
  }

  async function startNew(kind: Kind) {
    setProblem(null);
    setNotice(null);
    setIsNew(true);
    setDraft(await App.NewTheme(kind));
  }

  // Duplicate, delete and "make default" all come back as the whole list, so one
  // helper refreshes the rows and keeps the editor pointed at a theme that still
  // exists.
  async function run(action: () => Promise<Theme[] | null>, message: string) {
    setBusy(true);
    setProblem(null);
    setNotice(null);
    try {
      const list = (await action()) ?? [];
      setThemes(list);

      const current = list.find((theme) => theme.id === draft?.id);
      if (current) {
        setDraft(current);
      } else {
        setDraft(await App.NewTheme(Kind.KindScripture));
        setIsNew(true);
      }
      setNotice(message);
    } catch (error: unknown) {
      setProblem(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  const byKind = kinds.map((kind) => ({
    ...kind,
    items: themes.filter((theme) => theme.kind === kind.value),
  }));

  const stageStyle = {
    '--ff': draft?.style.fontFamily ?? 'serif',
    '--fs': `${draft?.style.fontSize ?? 4}cqh`,
    '--lh': String(draft?.style.lineHeight ?? 1.4),
    '--mg': String(draft?.style.margin ?? 8),
    '--fg': draft?.style.color ?? '#FFFFFF',
    '--ta': draft?.style.align ?? 'center',
    '--bg': backgroundCss(draft, images),
    '--weight': draft?.style.bold ? '700' : '400',
    '--slant': draft?.style.italic ? 'italic' : 'normal',
    '--lift': draft?.style.shadow ? '0 0.4cqh 1.2cqh rgba(0,0,0,.65)' : 'none',
  } as React.CSSProperties;

  return (
    <main className="themes-window" aria-label="Themes">
      <header className="themes-header">
        <div>
          <p className="eyebrow">Presentation</p>
          <h2>Themes</h2>
        </div>
        <button type="button" className="icon-button" onClick={() => Window.Close()} aria-label="Close themes" title="Close">
          <Icon name="close" size={16} />
        </button>
      </header>

      <div className="themes-body">
        <aside className="themes-list" aria-label="Saved themes">
          {byKind.map((group) => (
            <section key={group.value} className="themes-group">
              <div className="themes-group-head">
                <h3>{group.label}</h3>
                <button type="button" className="icon-button icon-button--small" onClick={() => void startNew(group.value)} aria-label={`New ${group.label} theme`} title={`New ${group.label} theme`}>
                  <Icon name="plus" size={13} />
                </button>
              </div>
              {group.items.length === 0 ? (
                <p className="themes-none">None yet.</p>
              ) : (
                <ul>
                  {group.items.map((theme) => (
                    <li key={theme.id}>
                      <button
                        type="button"
                        className={`theme-row${draft?.id === theme.id && !isNew ? ' theme-row--on' : ''}`}
                        onClick={() => pick(theme)}
                      >
                        <span className="theme-row-dot" style={{ background: theme.background.color || theme.style.color }} />
                        <span className="theme-row-name">{theme.name}</span>
                        {theme.isDefault && <span className="theme-row-badge">default</span>}
                      </button>
                      <span className="theme-row-actions">
                        <button type="button" className="icon-button icon-button--small" disabled={busy} onClick={() => void run(() => App.DuplicateTheme(theme.id), 'Copied.')} aria-label={`Duplicate ${theme.name}`} title="Duplicate">
                          <Icon name="layers" size={13} />
                        </button>
                        <button type="button" className="icon-button icon-button--small" disabled={busy} onClick={() => void run(() => App.DeleteTheme(theme.id), 'Theme removed.')} aria-label={`Delete ${theme.name}`} title="Delete">
                          <Icon name="close" size={13} />
                        </button>
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          ))}
        </aside>

        {draft === null ? (
          <p className="themes-none">Choose a theme, or start a new one.</p>
        ) : (
          <section className="themes-editor">
            <div className="theme-preview">
              <div className="theme-stage" style={stageStyle}>
                <div className="theme-sample">
                  {draft.reference === 'top' && <p className="theme-sample-ref theme-sample-ref--top">{sample.reference}</p>}
                  <div className="theme-sample-body">
                    <p ref={linesRef} className="theme-sample-lines">
                      {sample.lines.map((line, index) => <span key={index}>{line}</span>)}
                    </p>
                  </div>
                  {draft.reference !== 'top' && draft.reference !== 'none' && (
                    <p className="theme-sample-ref theme-sample-ref--bottom">{sample.reference}</p>
                  )}
                </div>
              </div>
              {overflow && (
                <p className="theme-overflow" role="status">
                  <Icon name="bell" size={13} />
                  That passage does not fit at this size. Lower the text size or the margins.
                </p>
              )}
            </div>

            <div className="theme-controls">
              <div className="theme-control-col">
                <label className="theme-field">
                  <span className="theme-field-label">Name</span>
                  <input
                    type="text"
                    value={draft.name}
                    onChange={(event) => patch({ name: event.target.value })}
                    autoComplete="off"
                  />
                </label>

                <Chips label="Slide kind" value={draft.kind} options={kinds} onPick={(value) => patch({ kind: value as Kind })} />

                <label className="theme-field">
                  <span className="theme-field-label">Font</span>
                  <select
                    value={draft.style.fontFamily}
                    onChange={(event) => patchStyle({ fontFamily: event.target.value })}
                  >
                    {fonts.map((family) => (
                      <option key={family.value} value={family.value}>{family.label}</option>
                    ))}
                  </select>
                </label>

                <Slider label="Text size" value={draft.style.fontSize} min={2} max={12} step={0.2} unit="%" onPick={(value) => patchStyle({ fontSize: value })} />
                <Slider label="Line spacing" value={draft.style.lineHeight} min={1} max={2.5} step={0.05} unit="" onPick={(value) => patchStyle({ lineHeight: value })} />
                <Slider label="Margins" value={draft.style.margin} min={0} max={25} step={1} unit="%" onPick={(value) => patchStyle({ margin: value })} />

                <div className="theme-field">
                  <span className="theme-field-label">Style</span>
                  <div className="theme-chips">
                    {([['bold', 'Bold'], ['italic', 'Italic'], ['shadow', 'Shadow']] as const).map(([key, label]) => (
                      <button
                        key={key}
                        type="button"
                        className={`theme-chip${draft.style[key] ? ' theme-chip--on' : ''}`}
                        onClick={() => patchStyle({ [key]: !draft.style[key] })}
                      >
                        {label}
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              <div className="theme-control-col">
                <Chips label="Text alignment" value={draft.style.align} options={alignments.map((a) => ({ value: a, label: a }))} onPick={(value) => patchStyle({ align: value })} />
                <Colour label="Text colour" value={draft.style.color} onPick={(value) => patchStyle({ color: value })} />

                <Chips
                  label="Background"
                  value={draft.background.kind}
                  options={backgrounds}
                  onPick={(value) => patchBackground({ kind: value as BackgroundKind })}
                />

                {draft.background.kind !== BackgroundKind.BackgroundImage && (
                  <Colour label={draft.background.kind === BackgroundKind.BackgroundGradient ? 'Colour from' : 'Colour'} value={draft.background.color} onPick={(value) => patchBackground({ color: value })} />
                )}
                {draft.background.kind === BackgroundKind.BackgroundGradient && (
                  <>
                    <Colour label="Colour to" value={draft.background.color2 || draft.background.color} onPick={(value) => patchBackground({ color2: value })} />
                    <Slider label="Angle" value={draft.background.angle} min={0} max={359} step={1} unit="°" onPick={(value) => patchBackground({ angle: Math.round(value) })} />
                  </>
                )}
                {draft.background.kind === BackgroundKind.BackgroundImage && (
                  <div className="theme-field">
                    <span className="theme-field-label">Picture from the library</span>
                    {images.length === 0 ? (
                      <p className="themes-none">No images in the library yet. Add one from the Media tab.</p>
                    ) : (
                      <div className="theme-pictures">
                        {images.map((image) => (
                          <button
                            key={image.id}
                            type="button"
                            className={`theme-picture${draft.background.mediaId === image.id ? ' theme-picture--on' : ''}`}
                            onClick={() => patchBackground({ mediaId: image.id })}
                            title={image.title}
                          >
                            <img src={image.thumbnailUrl} alt="" />
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                <Chips label="Reference" value={draft.reference} options={references} onPick={(value) => patch({ reference: value })} />
              </div>
            </div>

            {problem && <p className="theme-error" role="alert">{problem}</p>}
            {notice && !problem && <p className="theme-notice">{notice}</p>}

            <footer className="theme-editor-foot">
              <button
                type="button"
                className="button button--small"
                disabled={busy || isNew || draft.isDefault}
                onClick={() => void run(() => App.MakeThemeDefault(draft.id), 'That is the default now.')}
                title={isNew ? 'Save the theme first' : 'Use this theme for every new slide of this kind'}
              >
                <Icon name="check" size={13} />{draft.isDefault ? 'Default theme' : 'Make default'}
              </button>
              <span className="theme-foot-hint">
                A slide already live keeps the look it was projected with.
              </span>
              <button type="button" className="button button--small button--primary" disabled={busy || draft.name.trim() === ''} onClick={() => void save()}>
                {isNew ? 'Create theme' : 'Save changes'}
              </button>
            </footer>
          </section>
        )}
      </div>
    </main>
  );
}

function backgroundCss(theme: Theme | null, images: MediaView[]): string {
  if (!theme) return '#081A2E';
  const background = theme.background;
  if (background.kind === BackgroundKind.BackgroundGradient) {
    return `linear-gradient(${background.angle}deg, ${background.color}, ${background.color2 || background.color})`;
  }
  if (background.kind === BackgroundKind.BackgroundImage && background.mediaId) {
    const picture = images.find((image) => image.id === background.mediaId);
    if (picture) return `url('${picture.url}') center / cover no-repeat, ${background.color || '#081A2E'}`;
  }
  return background.color || '#081A2E';
}
