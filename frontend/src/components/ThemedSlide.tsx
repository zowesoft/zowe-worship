import type { CSSProperties, Ref } from 'react';
import type { Theme } from '../../bindings/zowe-worship/internal/themes/models';
import { BackgroundKind, TextCase } from '../../bindings/zowe-worship/internal/themes/models';
import './themed-slide.css';

// The readable look the app falls back to, matching themes.DefaultTheme on the
// Go side. A slide built by an older build, or before any theme existed, still
// has to be legible on the wall.
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

// A theme stores a picture by media id, so each surface supplies the address it
// can actually load. The colour sits under the picture so a deleted file shows a
// plain background rather than a broken image icon.
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

  // Anything the reference leaves unset follows the slide text, resolved here so
  // the stylesheet never has to guess which properties were meant to inherit.
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

type ThemedSlideProps = {
  theme?: Theme;
  lines: string[];
  reference: string;
  // Only the editor needs to measure the text; the surfaces just draw it.
  linesRef?: Ref<HTMLParagraphElement>;
  urlFor?: (mediaID: string) => string;
  className?: string;
};

export function ThemedSlide({ theme, lines, reference, linesRef, urlFor, className }: ThemedSlideProps) {
  const placement = theme?.reference.placement || 'bottom';

  return (
    <div className={`themed-stage${className ? ` ${className}` : ''}`} style={themeVars(theme, urlFor)}>
      <div className="themed-slide">
        {placement === 'top' && <p className="themed-ref themed-ref--top">{reference}</p>}
        <div className="themed-slide-body">
          <p ref={linesRef} className="themed-lines">
            {lines.map((line, index) => <span key={index}>{line}</span>)}
          </p>
        </div>
        {placement !== 'top' && placement !== 'none' && (
          <p className="themed-ref themed-ref--bottom">{reference}</p>
        )}
      </div>
    </div>
  );
}
