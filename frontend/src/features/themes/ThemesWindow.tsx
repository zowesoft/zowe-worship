import { useEffect, useRef, useState } from 'react';
import { Window } from '@wailsio/runtime';
import { App } from '../../../bindings/zowe-worship';
import type { MediaView } from '../../../bindings/zowe-worship/models';
import type { FontFamily, Theme } from '../../../bindings/zowe-worship/internal/themes/models';
import { BackgroundKind, Kind, TextCase } from '../../../bindings/zowe-worship/internal/themes/models';
import { Icon } from '../../components/Icon';
import { Dropdown } from '../../components/Dropdown';
import { ThemedSlide } from '../../components/ThemedSlide';

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

// Each option is spelled the way it will look, so the button previews itself.
const cases: { value: TextCase; label: string }[] = [
  { value: TextCase.CaseAsWritten, label: 'As written' },
  { value: TextCase.CaseCapitalized, label: 'Capitalize' },
  { value: TextCase.CaseUppercase, label: 'ALL CAPS' },
  { value: TextCase.CaseLowercase, label: 'lowercase' },
  { value: TextCase.CaseSmallCaps, label: 'Small caps' },
];

// The overflow warning measures the long form, because a theme that fits one
// verse is no use for a whole chapter.
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

const field = 'grid gap-[5px]';
const fieldLabel = 'flex justify-between gap-2 text-secondary text-[9.5px] tracking-[1px] uppercase';
const fieldInput = 'min-w-0 h-[30px] px-2 border border-[#35445C] rounded-md bg-app text-text text-[11.5px]';
const colourInput = 'w-16 h-[26px] p-0 border border-[#35445C] rounded-md bg-app';
const chipBase = 'px-[10px] py-[5px] border border-border rounded-full bg-transparent text-secondary text-[10.5px] whitespace-nowrap hover:text-text hover:bg-elevated';

function Chips({ label, value, options, onPick }: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onPick: (value: string) => void;
}) {
  return (
    <div className={field}>
      <span className={fieldLabel}>{label}</span>
      <div className="flex flex-wrap gap-[5px]">
        {options.map((option) => (
          <button
            key={option.value}
            type="button"
            className={`${chipBase}${option.value === value ? ' border-blue! bg-selected! text-text!' : ''}`}
            onClick={() => onPick(option.value)}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}

function Slider({ label, value, min, max, step, unit, display, onPick }: {
  label: string;
  value: number;
  min: number;
  max: number;
  step: number;
  unit: string;
  display?: string;
  onPick: (value: number) => void;
}) {
  return (
    <label className={field}>
      <span className={`${fieldLabel} [&_em]:not-italic [&_em]:text-text [&_em]:tracking-normal`}>
        {label}
        <em>{display ?? `${value}${unit}`}</em>
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

// A writable size box with a dropdown of common sizes, like a word processor; a
// size the presets miss can still be typed.
function SizePicker({ label, value, min, max, step, presets, onPick }: {
  label: string;
  value: number;
  min: number;
  max: number;
  step: number;
  presets: number[];
  onPick: (value: number) => void;
}) {
  const [text, setText] = useState(String(value));

  // The field is writable, so its text must follow a value changed elsewhere:
  // a preset click, or switching theme.
  useEffect(() => { setText(String(value)); }, [value]);

  // Clamp and round to the step, so a typed value lands on the same grid a drag
  // would have chosen.
  function commit(raw: string) {
    const parsed = Number(raw);
    if (raw.trim() === '' || Number.isNaN(parsed)) { setText(String(value)); return; }
    const rounded = Math.round(parsed / step) * step;
    const clamped = Math.min(max, Math.max(min, rounded));
    // Trim the float noise step divisions leave behind (0.1 * 3 = 0.30000000004).
    const clean = Number(clamped.toFixed(4));
    onPick(clean);
    setText(String(clean));
  }

  return (
    <div className={field}>
      <span className={fieldLabel}>{label}</span>
      <Dropdown
        role="listbox"
        align="right"
        offset={6}
        panelClassName="min-w-full max-h-[188px] overflow-y-auto"
        trigger={({ open, toggle }) => (
          <>
            <input
              type="text"
              inputMode="decimal"
              className={`${fieldInput} w-full pr-7 [&::-webkit-inner-spin-button]:appearance-none`}
              value={text}
              aria-label={label}
              onChange={(event) => setText(event.target.value)}
              onBlur={(event) => commit(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') { event.currentTarget.blur(); return; }
                if (event.key === 'Escape') { setText(String(value)); event.currentTarget.blur(); return; }
                if (event.key === 'ArrowUp') { event.preventDefault(); commit(String(Number(text || value) + step)); return; }
                if (event.key === 'ArrowDown') { event.preventDefault(); commit(String(Number(text || value) - step)); }
              }}
            />
            <button
              type="button"
              className="absolute inset-y-0 right-0 grid place-items-center w-6 border-0 rounded-r-md bg-transparent text-secondary hover:text-text [&_svg]:transition-transform [&_svg]:duration-150 hover:[&_svg]:text-blue"
              aria-label={`Common ${label.toLowerCase()}s`}
              aria-haspopup="listbox"
              aria-expanded={open}
              onClick={toggle}
              tabIndex={-1}
            >
              <Icon name="chevron" size={13} className={open ? 'rotate-180' : ''} />
            </button>
          </>
        )}
        menu={({ close }) => presets.map((preset) => (
          <button
            key={preset}
            type="button"
            role="option"
            aria-selected={preset === value}
            className={[
              'flex items-center gap-2 p-[7px] px-2 border-0 rounded-md bg-transparent text-text text-[11.5px] text-left whitespace-nowrap hover:bg-selected [&_svg]:shrink-0',
              preset === value ? '[&_svg]:text-teal' : '[&_svg]:text-transparent',
            ].join(' ')}
            onClick={() => { onPick(preset); setText(String(preset)); close(); }}
          >
            <Icon name="check" size={13} />
            <span>{preset === 0 ? 'Same as text' : `${preset}%`}</span>
          </button>
        ))}
      />
    </div>
  );
}

function Colour({ label, value, onPick }: {
  label: string;
  value: string;
  onPick: (value: string) => void;
}) {
  return (
    <label className={`${field} items-center grid-flow-col grid-cols-[minmax(0,1fr)_64px]`}>
      <span className={fieldLabel}>{label}</span>
      <input type="color" className={colourInput} value={value} onChange={(event) => onPick(event.target.value)} />
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
  const [fitted, setFitted] = useState(1);
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

  // A slide shrinks its text until the words fit, so the editor reports how far
  // it shrank: an operator can see a chosen size being pulled back for a long
  // passage.
  const shrunk = fitted < 0.98;

  function patch(change: Partial<Theme>) {
    setDraft((current) => (current ? { ...current, ...change } : current));
  }
  function patchStyle(change: Partial<Theme['style']>) {
    setDraft((current) => (current ? { ...current, style: { ...current.style, ...change } } : current));
  }
  function patchBackground(change: Partial<Theme['background']>) {
    setDraft((current) => (current ? { ...current, background: { ...current.background, ...change } } : current));
  }
  function patchReference(change: Partial<Theme['reference']>) {
    setDraft((current) => (current ? { ...current, reference: { ...current.reference, ...change } } : current));
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

  // Every mutation returns the whole list, so one helper refreshes the rows and
  // keeps the editor pointed at a theme that still exists.
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

  // The preview draws through the same component the studio and projector use, so
  // a theme cannot look right here and wrong on the wall.
  const pictureURL = (mediaID: string) => images.find((image) => image.id === mediaID)?.url ?? '';

  return (
    <main className="h-full grid grid-rows-[auto_minmax(0,1fr)] gap-3 pt-3.5 px-4 pb-4 overflow-hidden" aria-label="Themes">
      <header className="flex items-end justify-between gap-3.5 [&_h2]:text-[17px] [&_h2]:font-semibold [&_h2]:mt-[3px]">
        <div>
          <p className="text-[9.5px] uppercase tracking-[1.3px] font-semibold text-secondary">Presentation</p>
          <h2>Themes</h2>
        </div>
        <button type="button" className="inline-grid place-items-center w-[34px] h-[34px] border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50" onClick={() => Window.Close()} aria-label="Close themes" title="Close">
          <Icon name="close" size={16} />
        </button>
      </header>

      <div className="min-h-0 grid grid-cols-[212px_minmax(0,1fr)] gap-4 max-[900px]:grid-cols-[170px_minmax(0,1fr)]">
        <aside className="min-h-0 overflow-y-auto grid gap-3.5 content-start pr-1" aria-label="Saved themes">
          {byKind.map((group) => (
            <section key={group.value} className="[&_h3]:text-[9.5px] [&_h3]:tracking-[1.1px] [&_h3]:uppercase [&_h3]:text-secondary [&_ul]:m-0 [&_ul]:p-0 [&_ul]:list-none [&_ul]:grid [&_ul]:gap-[3px] [&_li]:flex [&_li]:items-center [&_li]:gap-0.5">
              <div className="flex items-center justify-between gap-2 mb-1.5">
                <h3>{group.label}</h3>
                <button type="button" className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50" onClick={() => void startNew(group.value)} aria-label={`New ${group.label} theme`} title={`New ${group.label} theme`}>
                  <Icon name="plus" size={13} />
                </button>
              </div>
              {group.items.length === 0 ? (
                <p className="text-secondary text-[10.5px]">None yet.</p>
              ) : (
                <ul>
                  {group.items.map((theme) => (
                    <li key={theme.id}>
                      <button
                        type="button"
                        className={`flex-1 min-w-0 flex items-center gap-[7px] px-2 py-[7px] border border-transparent rounded-md bg-transparent text-text text-[11.5px] text-left hover:bg-elevated ${draft?.id === theme.id && !isNew ? ' border-blue! bg-selected!' : ''}`}
                        onClick={() => pick(theme)}
                      >
                        <span className="w-[11px] h-[11px] rounded-[3px] border border-border shrink-0" style={{ background: theme.background.color || theme.style.color }} />
                        <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{theme.name}</span>
                        {theme.isDefault && <span className="text-teal text-[8.5px] tracking-[0.7px] uppercase">default</span>}
                      </button>
                      <span className="flex gap-0.5">
                        <button type="button" className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50" disabled={busy} onClick={() => void run(() => App.DuplicateTheme(theme.id), 'Copied.')} aria-label={`Duplicate ${theme.name}`} title="Duplicate">
                          <Icon name="layers" size={13} />
                        </button>
                        <button type="button" className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50" disabled={busy} onClick={() => void run(() => App.DeleteTheme(theme.id), 'Theme removed.')} aria-label={`Delete ${theme.name}`} title="Delete">
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
          <p className="text-secondary text-[10.5px]">Choose a theme, or start a new one.</p>
        ) : (
          <section className="min-h-0 overflow-y-auto grid gap-3 content-start pr-1">
            <div>
              <ThemedSlide
                theme={draft}
                lines={sample.lines}
                reference={sample.reference}
                linesRef={linesRef}
                urlFor={pictureURL}
                className="w-full max-h-[240px] aspect-[16/9] mx-auto border border-[#30445F] rounded-lg"
                onFit={setFitted}
              />
              {shrunk && (
                <p className="flex items-center gap-1.5 text-amber text-[10.5px]" role="status">
                  <Icon name="bell" size={13} />
                  A long passage is shrunk to fit, so it reads a little smaller than this size.
                </p>
              )}
            </div>

            <div className="grid grid-cols-2 gap-[18px] max-[900px]:grid-cols-1">
              <div className="grid gap-[11px] content-start">
                <label className={field}>
                  <span className={fieldLabel}>Name</span>
                  <input
                    type="text"
                    value={draft.name}
                    onChange={(event) => patch({ name: event.target.value })}
                    autoComplete="off"
                    className={fieldInput}
                  />
                </label>

                <Chips label="Slide kind" value={draft.kind} options={kinds} onPick={(value) => patch({ kind: value as Kind })} />

                <label className={field}>
                  <span className={fieldLabel}>Font</span>
                  <select
                    value={draft.style.fontFamily}
                    onChange={(event) => patchStyle({ fontFamily: event.target.value })}
                    className={fieldInput}
                  >
                    {fonts.map((family) => (
                      <option key={family.value} value={family.value}>{family.label}</option>
                    ))}
                  </select>
                </label>

                <SizePicker label="Text size" value={draft.style.fontSize} min={2} max={12} step={0.2} presets={[3, 4, 5, 6, 7, 8, 9, 10, 11, 12]} onPick={(value) => patchStyle({ fontSize: value })} />
                <Slider label="Line spacing" value={draft.style.lineHeight} min={1} max={2.5} step={0.05} unit="" onPick={(value) => patchStyle({ lineHeight: value })} />
                <Slider label="Margins" value={draft.style.margin} min={0} max={25} step={1} unit="%" onPick={(value) => patchStyle({ margin: value })} />

                <div className={field}>
                  <span className={fieldLabel}>Style</span>
                  <div className="flex flex-wrap gap-[5px]">
                    {([['bold', 'Bold'], ['italic', 'Italic'], ['shadow', 'Shadow']] as const).map(([key, label]) => (
                      <button
                        key={key}
                        type="button"
                        className={`${chipBase}${draft.style[key] ? ' border-blue! bg-selected! text-text!' : ''}`}
                        onClick={() => patchStyle({ [key]: !draft.style[key] })}
                      >
                        {label}
                      </button>
                    ))}
                  </div>
                </div>

                <Chips label="Letter case" value={draft.style.case} options={cases} onPick={(value) => patchStyle({ case: value as TextCase })} />
              </div>

              <div className="grid gap-[11px] content-start">
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
                  <div className={field}>
                    <span className={fieldLabel}>Picture from the library</span>
                    {images.length === 0 ? (
                      <p className="text-secondary text-[10.5px]">No images in the library yet. Add one from the Media tab.</p>
                    ) : (
                      <div className="grid grid-cols-[repeat(auto-fill,minmax(74px,1fr))] gap-1.5 max-h-[132px] overflow-y-auto">
                        {images.map((image) => (
                          <button
                            key={image.id}
                            type="button"
                            className={`p-0 border border-border rounded-md overflow-hidden bg-transparent ${draft.background.mediaId === image.id ? ' border-blue! shadow-[0_0_0_2px_var(--color-selected)]' : ''}`}
                            onClick={() => patchBackground({ mediaId: image.id })}
                            title={image.title}
                          >
                            <img src={image.thumbnailUrl} alt="" className="block w-full h-[52px] object-cover" />
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                )}

              </div>
            </div>

            <section className="grid gap-3 border-t border-border pt-3">
              <Chips
                label="Reference"
                value={draft.reference.placement}
                options={references}
                onPick={(value) => patchReference({ placement: value })}
              />

              {draft.reference.placement !== 'none' && (
                <div className="grid grid-cols-2 gap-y-3.5 gap-x-[18px] items-start max-[900px]:grid-cols-1">
                  <label className={field}>
                    <span className={fieldLabel}>Reference font</span>
                    <select
                      value={draft.reference.fontFamily}
                      onChange={(event) => patchReference({ fontFamily: event.target.value })}
                      className={fieldInput}
                    >
                      <option value="">Same as slide text</option>
                      {fonts.map((family) => (
                        <option key={family.value} value={family.value}>{family.label}</option>
                      ))}
                    </select>
                  </label>

                  <SizePicker
                    label="Reference size"
                    value={draft.reference.fontSize}
                    min={0}
                    max={8}
                    step={0.2}
                    // Zero is the "follow the slide text" choice, so it sits where
                    // an operator looks first.
                    presets={[0, 2, 3, 4, 5, 6, 7, 8]}
                    onPick={(value) => patchReference({ fontSize: value })}
                  />

                  <Chips
                    label="Reference alignment"
                    value={draft.reference.align}
                    options={[{ value: '', label: 'Same as text' }, ...alignments.map((a) => ({ value: a, label: a }))]}
                    onPick={(value) => patchReference({ align: value })}
                  />

                  <Chips
                    label="Reference case"
                    value={draft.reference.case}
                    options={cases}
                    onPick={(value) => patchReference({ case: value as TextCase })}
                  />

                  <div className={field}>
                    <span className={fieldLabel}>Reference emphasis</span>
                    <div className="flex flex-wrap gap-[5px]">
                      {([['bold', 'Bold'], ['italic', 'Italic']] as const).map(([key, label]) => (
                        <button
                          key={key}
                          type="button"
                          className={`${chipBase}${draft.reference[key] ? ' border-blue! bg-selected! text-text!' : ''}`}
                          onClick={() => patchReference({ [key]: !draft.reference[key] })}
                        >
                          {label}
                        </button>
                      ))}
                    </div>
                  </div>

                  <div className={`${field} items-center grid-flow-col grid-cols-[minmax(0,1fr)_64px]`}>
                    <span className={`${fieldLabel} [&_em]:not-italic [&_em]:text-text [&_em]:tracking-normal`}>
                      Reference colour
                      <em>{draft.reference.color === '' ? 'same as text' : ''}</em>
                    </span>
                    <input
                      type="color"
                      className={colourInput}
                      value={draft.reference.color || draft.style.color}
                      onChange={(event) => patchReference({ color: event.target.value })}
                    />
                  </div>

                  {draft.reference.color !== '' && (
                    <button type="button" className="justify-self-start border-0 p-0 bg-transparent text-blue text-[10.5px] underline" onClick={() => patchReference({ color: '' })}>
                      Use the slide colour
                    </button>
                  )}
                </div>
              )}
            </section>

            {problem && <p className="text-error text-[11.5px]" role="alert">{problem}</p>}
            {notice && !problem && <p className="text-teal text-[11.5px]">{notice}</p>}

            <footer className="flex items-center justify-between gap-3 border-t border-border pt-[11px]">
              <button
                type="button"
                className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]"
                disabled={busy || isNew || draft.isDefault}
                onClick={() => void run(() => App.MakeThemeDefault(draft.id), 'That is the default now.')}
                title={isNew ? 'Save the theme first' : 'Use this theme for every new slide of this kind'}
              >
                <Icon name="check" size={13} />{draft.isDefault ? 'Default theme' : 'Make default'}
              </button>
              <span className="text-[#6F809B] text-[10px]">
                A slide already live keeps the look it was projected with.
              </span>
              <button type="button" className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-blue! rounded-md bg-blue! text-white! text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!" disabled={busy || draft.name.trim() === ''} onClick={() => void save()}>
                {isNew ? 'Create theme' : 'Save changes'}
              </button>
            </footer>
          </section>
        )}
      </div>
    </main>
  );
}
