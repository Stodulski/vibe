import { useEffect, useRef } from 'react';

/** The index of the tick whose center is closest to the track's center. */
export function centeredIndex(track: HTMLElement): number {
  const middle = track.scrollLeft + track.clientWidth / 2;
  let best = 0;
  let distance = Number.POSITIVE_INFINITY;
  Array.from(track.children).forEach((child, index) => {
    const el = child as HTMLElement;
    const d = Math.abs(el.offsetLeft + el.offsetWidth / 2 - middle);
    if (d < distance) {
      distance = d;
      best = index;
    }
  });
  return best;
}

/** Brings tick `index` to the center, smoothly. */
export function centerTick(track: HTMLElement | null, index: number) {
  const tick = track?.children[index] as HTMLElement | undefined;
  if (!track || !tick) return;
  track.scrollTo({ left: tick.offsetLeft + tick.offsetWidth / 2 - track.clientWidth / 2, behavior: 'smooth' });
}

/** A mouse wheel moves the ruler sideways: a desktop without a trackpad has no other way to slide it. */
export function useWheelSlides(trackRef: React.RefObject<HTMLDivElement | null>) {
  useEffect(() => {
    const track = trackRef.current;
    if (!track) return;
    const onWheel = (event: WheelEvent) => {
      if (Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return;
      event.preventDefault();
      track.scrollBy({ left: event.deltaY });
    };
    track.addEventListener('wheel', onWheel, { passive: false });
    return () => {
      track.removeEventListener('wheel', onWheel);
    };
  }, [trackRef]);
}

/**
 * Dragging the ruler with a mouse, as a finger does on a phone. Snapping is off while
 * it moves, and on release the nearest hour settles under the marker. A drag never
 * counts as a tap on the tick it ends over.
 */
export function useMouseDrag(trackRef: React.RefObject<HTMLDivElement | null>) {
  const drag = useRef<{ x: number; left: number; moved: boolean } | null>(null);
  const justDragged = useRef(false);
  const onPointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    if (event.pointerType !== 'mouse' || event.button !== 0) return;
    drag.current = { x: event.clientX, left: event.currentTarget.scrollLeft, moved: false };
  };
  const onPointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    const state = drag.current;
    if (!state) return;
    const dx = event.clientX - state.x;
    if (!state.moved && Math.abs(dx) < 5) return;
    if (!state.moved) {
      state.moved = true;
      event.currentTarget.setPointerCapture(event.pointerId);
      event.currentTarget.style.scrollSnapType = 'none';
    }
    event.currentTarget.scrollLeft = state.left - dx;
  };
  const onPointerEnd = (event: React.PointerEvent<HTMLDivElement>) => {
    const state = drag.current;
    drag.current = null;
    if (!state?.moved) return;
    justDragged.current = true;
    const track = event.currentTarget;
    track.style.scrollSnapType = '';
    centerTick(trackRef.current, centeredIndex(track));
  };
  // Swallows the click a drag ends with, so the tick under the pointer is not tapped.
  const onClickCapture = (event: React.MouseEvent) => {
    if (!justDragged.current) return;
    justDragged.current = false;
    event.stopPropagation();
    event.preventDefault();
  };
  return { onPointerDown, onPointerMove, onPointerUp: onPointerEnd, onPointerCancel: onPointerEnd, onClickCapture };
}

/** A scroll handler that runs `update` once per frame, however many scroll events a fast swipe fires. */
export function useFrameScroll(update: (track: HTMLDivElement) => void) {
  const frame = useRef<number | null>(null);
  const latest = useRef(update);
  useEffect(() => {
    latest.current = update;
  });
  useEffect(
    () => () => {
      if (frame.current !== null) cancelAnimationFrame(frame.current);
    },
    [],
  );
  return (event: React.UIEvent<HTMLDivElement>) => {
    const track = event.currentTarget;
    if (frame.current !== null) return;
    frame.current = requestAnimationFrame(() => {
      frame.current = null;
      latest.current(track);
    });
  };
}
