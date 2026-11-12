import type { RefObject } from 'react';
import { FittedImage, frameStyle } from './FittedImage';
import { Icon } from './Icon';
import type { Content } from '../../bindings/zopresent/internal/presentation/models';

type Props = {
  content: Content;
  /** The player element actually plays; a mirror only shows the first frame. */
  player: boolean;
  /** Off for a local preview player that waits for the operator to press play. */
  autoPlay?: boolean;
  /** The player keeps hold of the element so transport commands can reach it. */
  mediaRef?: RefObject<HTMLMediaElement | null>;
  onEnded?: () => void;
};

/**
 * Draws a media slide. A player element plays and is heard; mirrors show the
 * first frame of a clip and stay quiet, so the same recording is never sounding
 * twice from two elements that cannot keep step.
 */
export function SlideMedia({ content, player, autoPlay = player, mediaRef, onEnded }: Props) {
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
      <div className="relative grid grid-rows-none content-center justify-items-center items-center gap-3.5 w-full h-full px-2.5 py-1 overflow-hidden">
        {content.background && (
          <>
            <img src={content.background} alt="" className="absolute inset-0 w-full h-full object-cover" />
            <div className="absolute inset-0 bg-[rgba(4,9,18,0.45)]" />
          </>
        )}
        {player && <audio ref={attach} className="hidden" src={source} autoPlay={autoPlay} onEnded={onEnded} />}
        <span className={`relative grid place-items-center w-14 h-14 border rounded-[14px] ${content.background ? 'border-[#5E749A] bg-[rgba(11,20,36,0.72)]' : 'border-[#30466A] bg-selected'} text-[#8CABFF]`}><Icon name="music" size={30} /></span>
        <p className={`relative text-[15px] font-semibold text-center ${content.background ? '[text-shadow:0_1px_8px_rgba(0,0,0,0.7)]' : ''}`}>{content.label}</p>
        {!player && <p className="relative text-secondary text-[10px]">Plays on the audience display</p>}
      </div>
    );
  }

  if (content.kind === 'video') {
    return (
      <div className="relative grid grid-rows-[minmax(0,1fr)_auto] justify-items-center items-center gap-1.5 w-full h-full px-2.5 py-1 overflow-hidden">
        {content.background && <img src={content.background} alt="" className="absolute inset-0 w-full h-full object-cover" />}
        <video
          ref={attach}
          className={`relative block ${content.background ? 'z-[1]' : ''}`}
          src={source}
          style={style}
          autoPlay={autoPlay}
          playsInline
          muted={!player}
          preload={player ? 'auto' : 'metadata'}
          onEnded={onEnded}
        />
        {!player && <p className="relative text-secondary text-[10px]">Plays on the audience display</p>}
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
