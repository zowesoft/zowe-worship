type Props = {
  src: string;
  alt: string;
  /** Output shape to frame the picture in. 0 means use the image's own shape. */
  ratio?: number;
  /** contain shows all of it; cover fills the frame and loses the edges. */
  fit?: 'contain' | 'cover';
  className?: string;
};

/**
 * The largest box of the chosen shape that fits the frame, with the picture
 * placed inside it. The ratio goes on the image itself: a replaced element
 * honours `aspect-ratio` together with both max constraints, which is what makes
 * this letterbox correctly in a 240px studio pane and on a projector without the
 * box having to measure anything.
 */
export function FittedImage({ src, alt, ratio = 0, fit = 'contain', className }: Props) {
  const style: React.CSSProperties = {
    objectFit: fit,
    maxWidth: '100%',
    maxHeight: '100%',
    width: 'auto',
    height: 'auto',
  };
  if (ratio > 0) {
    style.aspectRatio = String(ratio);
  }

  return (
    <img
      className={`fitted-image${className ? ` ${className}` : ''}`}
      src={src}
      alt={alt}
      style={style}
    />
  );
}
