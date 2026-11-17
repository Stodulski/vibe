import { useEffect, type RefObject } from 'react';
import { centeredIndex } from './rulerTrack';

/** The ruler is drawn three times over, so it can turn forever in either direction. */
export const LOOP_COPIES = 3;

/** Moves a ruler that settled on an outer copy to the same hour in the middle copy. */
function recenter(track: HTMLElement, count: number) {
  const index = centeredIndex(track);
  if (index >= count && index < count * 2) return;
  const first = track.children[0] as HTMLElement | undefined;
  const second = track.children[1] as HTMLElement | undefined;
  if (!first || !second) return;
  const copyWidth = (second.offsetLeft - first.offsetLeft) * count;
  track.style.scrollSnapType = 'none';
  track.scrollLeft += index < count ? copyWidth : -copyWidth;
  track.style.scrollSnapType = '';
}

/**
 * Keeps an endless ruler endless: once it stops on the first or last copy of the
 * hours, it jumps, without animation, to the same hour in the middle copy. The jump
 * is one copy's width, so the ticks on screen do not move. It waits for the scroll to
 * end (\`scrollend\`, or a pause where the browser lacks it) so it never cuts a
 * finger's fling short.
 */
export function useLoopWrap(trackRef: RefObject<HTMLDivElement | null>, count: number) {
  useEffect(() => {
    const track = trackRef.current;
    if (!track || count < 2) return;
    const settle = () => {
      recenter(track, count);
    };
    if ('onscrollend' in window) {
      track.addEventListener('scrollend', settle);
      return () => {
        track.removeEventListener('scrollend', settle);
      };
    }
    let timer: ReturnType<typeof setTimeout> | undefined;
    const onScroll = () => {
      clearTimeout(timer);
      timer = setTimeout(settle, 250);
    };
    track.addEventListener('scroll', onScroll, { passive: true });
    return () => {
      clearTimeout(timer);
      track.removeEventListener('scroll', onScroll);
    };
  }, [trackRef, count]);
}
