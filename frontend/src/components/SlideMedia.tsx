import { FittedImage, frameStyle } from './FittedImage';
import { Icon } from './Icon';
import type { Content } from '../../bindings/zowe-worship/internal/presentation/models';

type Props = {
  content: Content;
  /** The audience window is the player; the studio only mirrors what it shows. */
  player: boolean;
};

/**
 * Draws a media slide. Sound belongs to one window only: the studio panes show
 * the first frame of a clip and stay quiet, so the operator is never hearing the
 * same recording twice from two elements that cannot keep step.
 */
export function SlideMedia({ content, player }: Props) {
  const fit = content.fit === 'cover' ? 'cover' : 'contain';
  const style = frameStyle(content.aspect, content.ratio, fit);
  const source = content.source ?? '';

  if (content.kind === 'audio') {
    return (
      <div className="slide-media slide-media--audio">
        {player && <audio className="slide-media-sound" src={source} autoPlay />}
        <span className="slide-media-icon"><Icon name="music" size={30} /></span>
        <p className="slide-media-title">{content.label}</p>
        {!player && <p className="slide-media-hint">Plays on the audience display</p>}
      </div>
    );
  }

  if (content.kind === 'video') {
    return (
      <div className="slide-media">
        <video
          className="fitted-image"
          src={source}
          style={style}
          autoPlay={player}
          playsInline
          muted={!player}
          preload={player ? 'auto' : 'metadata'}
        />
        {!player && <p className="slide-media-hint">Plays on the audience display</p>}
      </div>
    );
  }

  return (
    <FittedImage
      src={source}
      alt={content.label}
      aspect={content.aspect}
      ratio={content.ratio}
      fit={fit}
    />
  );
}
