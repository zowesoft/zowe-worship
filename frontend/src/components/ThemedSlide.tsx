import { useLayoutEffect, useRef } from 'react';
import type { CSSProperties, Ref } from 'react';
import type { Theme } from '../../bindings/zopresent/internal/themes/models';
import { BackgroundKind, TextCase } from '../../bindings/zopresent/internal/themes/models';

// Below this the words stop being readable across a room.
const minScale = 0.3;

// Mirrors themes.DefaultTheme on the Go side, for slides written by an older
// build or before any theme existed.
const fallback = {
  fontFamily: 'Georgia, "Times New Roman", serif',
  fontSize: 4.6,
  lineHeight: 1.45,
  margin: 9,
  color: '#F3F6FC',
  align: 'center',
};

function caseVars(value: TextCase | undefined): { transform: string; variant: string } {
  switch (value) {
    case TextCase.CaseCapitalized:
      return { transform: 'capitalize', variant: 'normal' };
    case TextCase.CaseUppercase:
      return { transform: 'uppercase', variant: 'normal' };
    case TextCase.CaseLowercase:
      return { transform: 'lowercase', variant: 'normal' };
    case TextCase.CaseSmallCaps:
      return { transform: 'none', variant: 'small-caps' };
    default:
      return { transform: 'none', variant: 'normal' };
  }
}

// The colour sits under the picture so a deleted file shows a plain background
// rather than a broken image icon.
function backgroundOf(theme: Theme | undefined, urlFor?: (mediaID: string) => string): string {
  const color = theme?.background.color || '#081A2E';
  const background = theme?.background;
  if (!background) return color;

  if (background.kind === BackgroundKind.BackgroundGradient) {
    return `linear-gradient(${background.angle}deg, ${background.color}, ${background.color2 || background.color})`;
  }
  if (background.kind === BackgroundKind.BackgroundImage && background.mediaId && urlFor) {
    const url = urlFor(background.mediaId);
    if (url) return `${color} url('${url}') center / cover no-repeat`;
  }
  return color;
}

export function themeVars(theme: Theme | undefined, urlFor?: (mediaID: string) => string): CSSProperties {
  const bodyCase = caseVars(theme?.style.case);
  const referenceCase = caseVars(theme?.reference.case);
  const bodySize = theme?.style.fontSize || fallback.fontSize;
  const reference = theme?.reference;

  // Unset reference properties follow the slide text, resolved here so the
  // stylesheet never has to guess which properties were meant to inherit.
  return {
    '--ff': theme?.style.fontFamily || fallback.fontFamily,
    '--fs': `${bodySize}cqh`,
    '--lh': String(theme?.style.lineHeight ?? fallback.lineHeight),
    '--mg': String(theme?.style.margin ?? fallback.margin),
    '--fg': theme?.style.color || fallback.color,
    '--ta': theme?.style.align || fallback.align,
    '--ttransform': bodyCase.transform,
    '--tvariant': bodyCase.variant,
    '--bg': backgroundOf(theme, urlFor),
    '--weight': theme?.style.bold ? '700' : '400',
    '--slant': theme?.style.italic ? 'italic' : 'normal',
    '--lift': theme?.style.shadow ? '0 0.4cqh 1.2cqh rgba(0,0,0,.65)' : 'none',
    '--rff': reference?.fontFamily || theme?.style.fontFamily || fallback.fontFamily,
    '--rfs': reference && reference.fontSize > 0 ? `${reference.fontSize}cqh` : `calc(${bodySize}cqh * .45)`,
    '--rfg': reference?.color || theme?.style.color || fallback.color,
    '--rta': reference?.align || theme?.style.align || fallback.align,
    '--rtransform': referenceCase.transform,
    '--rvariant': referenceCase.variant,
    '--rweight': reference?.bold ? '700' : 'inherit',
    '--rslant': reference?.italic ? 'italic' : 'inherit',
  } as CSSProperties;
}

// fitScale finds the largest fraction of the theme's text size at which the
// passage still fits, writing it to --fit on the stage every length is measured
// against so the text, gap and reference shrink together.
function fitScale(stage: HTMLElement, element: HTMLElement, box: HTMLElement): number {
  const setScale = (scale: number) => stage.style.setProperty('--fit', String(scale));

  for (let scale = 1; scale >= minScale - 1e-6; scale -= 0.05) {
    setScale(scale);
    if (element.scrollHeight <= box.clientHeight + 1) return scale;
  }
  setScale(minScale);
  return minScale;
}

type ThemedSlideProps = {
  theme?: Theme;
  lines: string[];
  reference: string;
  // The editor hands in its own ref so it can measure overflow; every other
  // surface lets the component own the node it fits.
  linesRef?: Ref<HTMLParagraphElement>;
  urlFor?: (mediaID: string) => string;
  className?: string;
  // Reports the drawn scale so the editor can tell when a passage was shrunk.
  onFit?: (scale: number) => void;
};

// The reference label shares one look wherever it is placed; only its grid row
// differs between the top and bottom placements.
const refClasses = 'm-0 [font-family:var(--rff)] [font-size:calc(var(--rfs)*var(--fit,1))] [color:var(--rfg)] [text-align:var(--rta)] [text-transform:var(--rtransform)] [font-variant:var(--rvariant)] [font-weight:var(--rweight)] [font-style:var(--rslant)] [text-shadow:var(--lift)]';

export function ThemedSlide({ theme, lines, reference, linesRef, urlFor, className, onFit }: ThemedSlideProps) {
  const placement = theme?.reference.placement || 'bottom';
  const stageRef = useRef<HTMLDivElement>(null);
  const ownLines = useRef<HTMLParagraphElement>(null);
  const linesNode = linesRef ?? ownLines;

  // Re-fit whenever the words, reference or theme change; the theme's size is
  // the ceiling the fit shrinks from.
  const fitKey = `${lines.join('\n')}\u0000${reference}\u0000${JSON.stringify(theme?.style ?? null)}\u0000${JSON.stringify(theme?.reference ?? null)}`;

  useLayoutEffect(() => {
    const stage = stageRef.current;
    const element = ownLines.current ?? (linesRef && typeof linesRef !== 'function' ? linesRef.current : null);
    const box = element?.parentElement;
    if (!stage || !element || !box) return;

    // The fit always runs; only the report back to the caller is optional.
    const apply = () => {
      const scale = fitScale(stage, element, box);
      onFit?.(scale);
    };
    apply();

    // The stage is sized by its container, never by the words, so the fit cannot
    // re-trigger the observer and loop.
    const observer = new ResizeObserver(apply);
    observer.observe(stage);
    return () => observer.disconnect();
  }, [fitKey, linesRef, onFit]);

  return (
    <div ref={stageRef} className={`[container-type:size] relative overflow-hidden ${className ? ` ${className}` : ''}`} style={themeVars(theme, urlFor)}>
      <div className="absolute inset-0 grid grid-rows-[auto_minmax(0,1fr)_auto] gap-[calc(var(--fs)*var(--fit,1)*.4)] py-[calc(var(--mg)*1cqh)] px-[calc(var(--mg)*1cqw)] [background:var(--bg)] [color:var(--fg)] [font-family:var(--ff)] [font-size:calc(var(--fs)*var(--fit,1))] leading-[var(--lh)] [font-weight:var(--weight)] [font-style:var(--slant)] [text-align:var(--ta)]">
        {placement === 'top' && <p className={`${refClasses} [grid-row:1]`}>{reference}</p>}
        <div className="[grid-row:2] min-h-0 grid content-center overflow-hidden">
          <p ref={linesNode} className="m-0 [text-transform:var(--ttransform)] [font-variant:var(--tvariant)] [&>span]:block [&>span]:[text-shadow:var(--lift)]">
            {lines.map((line, index) => <span key={index}>{line}</span>)}
          </p>
        </div>
        {placement !== 'top' && placement !== 'none' && (
          <p className={`${refClasses} [grid-row:3]`}>{reference}</p>
        )}
      </div>
    </div>
  );
}
