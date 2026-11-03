import type { RefObject } from 'react';
import { FittedImage, frameStyle } from './FittedImage';
import { Icon } from './Icon';
import type { Content } from '../../bindings/zowe-worship/internal/presentation/models';

type Props = {
  content: Content;
  /** The audience window is the player; the studio only mirrors what it shows. */
  player: boolean;
  /** The player keeps hold of the element so transport commands can reach it. */
  mediaRef?: RefObject<HTMLMediaElement | null>;
  onEnded?: () => void;
};

/**
 * Draws a media slide. Sound belongs to one window only: the studio panes show
 * the first frame of a clip and stay quiet, so the operator is never hearing the
 * same recording twice from two elements that cannot keep step.
 */
export function SlideMedia({ content, player, mediaRef, onEnded }: Props) {
  const fit = content.fit === 'cover' ? 'cover' : 'contain';
  const style = frameStyle(content.aspect, content.ratio, fit);
  const source = content.source ?? '';

  // A callback ref, because the element is either a video or an audio and the
  // caller only ever needs the media half of what it got.
  const attach = (element: HTMLVideoElement | HTMLAudioElement | null) => {
    if (mediaRef) mediaRef.current = element;
  };

  if (content.kind === 'audio') {
    return (
      <div className="grid grid-rows-none content-center justify-items-center items-center gap-3.5 w-full h-full px-2.5 py-1">
        {player && <audio ref={attach} className="hidden" src={source} autoPlay onEnded={onEnded} />}
        <span className="grid place-items-center w-14 h-14 border border-[#30466A] rounded-[14px] bg-selected text-[#8CABFF]"><Icon name="music" size={30} /></span>
        <p className="text-[15px] font-semibold text-center">{content.label}</p>
        {!player && <p className="text-secondary text-[10px]">Plays on the audience display</p>}
      </div>
    );
  }

  if (content.kind === 'video') {
    return (
      <div className="grid grid-rows-[minmax(0,1fr)_auto] justify-items-center items-center gap-1.5 w-full h-full px-2.5 py-1">
        <video
          ref={attach}
          className="block"
          src={source}
          style={style}
          autoPlay={player}
          playsInline
          muted={!player}
          preload={player ? 'auto' : 'metadata'}
          onEnded={onEnded}
        />
        {!player && <p className="text-secondary text-[10px]">Plays on the audience display</p>}
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
