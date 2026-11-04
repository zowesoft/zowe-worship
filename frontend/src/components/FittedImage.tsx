type Props = {
  src: string;
  alt: string;
  /** The stored framing choice. "screen" fills the output and crops the rest. */
  aspect?: string;
  /** Resolved output shape. 0 means the picture's own shape. */
  ratio?: number;
  /** contain shows all of it; cover fills the frame and loses the edges. */
  fit?: 'contain' | 'cover';
  className?: string;
};

/**
 * The largest box of the chosen shape that fits the frame, with the picture
 * placed inside it. The ratio goes on the replaced element itself: it honours
 * `aspect-ratio` together with both max constraints, which is what makes this
 * letterbox correctly in a 240px studio pane and on a projector without the box
 * having to measure anything.
 *
 * The percentages need a parent with a definite height to resolve against —
 * flex centring or an explicit size qualifies, a content-hugging centred grid
 * row does not, and the picture then lays out at full size and gets clipped.
 */
export function frameStyle(aspect?: string, ratio = 0, fit: 'contain' | 'cover' = 'contain'): React.CSSProperties {
  const style: React.CSSProperties = { objectFit: fit };

  if (aspect === 'screen') {
    // No shape of its own: the picture takes the whole output and whatever does
    // not fit is cropped, which is what "fill the screen" means to an operator.
    style.width = '100%';
    style.height = '100%';
    style.objectFit = 'cover';
  } else if (ratio > 0) {
    style.aspectRatio = String(ratio);
    style.maxWidth = '100%';
    style.maxHeight = '100%';
  } else {
    style.maxWidth = '100%';
    style.maxHeight = '100%';
  }

  return style;
}

export function FittedImage({ src, alt, aspect, ratio = 0, fit = 'contain', className }: Props) {
  return (
    <img
      className={`block ${className ? ` ${className}` : ''}`}
      src={src}
      alt={alt}
      style={frameStyle(aspect, ratio, fit)}
    />
  );
}
