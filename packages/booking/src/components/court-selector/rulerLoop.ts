import { useRef } from 'react';
import { centeredIndex } from './rulerTrack';

/** The ruler is drawn three times over, so it can turn forever in either direction. */
export const LOOP_COPIES = 3;

/**
 * Keeps an endless ruler endless: once it settles on the first or last copy of the
 * hours, it jumps, without animation, to the same hour in the middle copy. The jump is
 * one copy's width, so the ticks on screen do not move.
 */
export function useLoopWrap(count: number) {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  return (track: HTMLElement) => {
    if (count < 2) return;
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      const index = centeredIndex(track);
      if (index >= count && index < count * 2) return;
      const first = track.children[0] as HTMLElement | undefined;
      const second = track.children[1] as HTMLElement | undefined;
      if (!first || !second) return;
      const copyWidth = (second.offsetLeft - first.offsetLeft) * count;
      track.style.scrollSnapType = 'none';
      track.scrollLeft += index < count ? copyWidth : -copyWidth;
      track.style.scrollSnapType = '';
    }, 140);
  };
}
