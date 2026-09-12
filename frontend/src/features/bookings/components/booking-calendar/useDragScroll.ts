import { useRef, useState, type PointerEvent as ReactPointerEvent, type RefObject } from 'react';

/**
 * Movement below this is a click's own jitter, not a drag.
 *
 * It decides one thing only: whether to capture the pointer. Capturing steals
 * the release from whatever slot sits under it, and a press that wandered two
 * pixels should still open that slot.
 *
 * It deliberately does NOT gate the scrolling. It used to, and the first three
 * pixels of every drag moved nothing before the fourth jumped four at once —
 * a dead zone and a snap, at the exact moment a drag most needs to feel
 * attached to the cursor.
 */
const DRAG_THRESHOLD_PX = 4;

/**
 * Grab-and-drag horizontal scrolling for the court columns.
 *
 * The columns follow the pointer from its first pixel of travel. What the
 * threshold decides is only whether to capture the pointer — below it the
 * press keeps its target, so pressing a free slot still opens its duration
 * popover rather than being eaten by the scroll.
 *
 * Scrolling itself is done by writing `scrollLeft` straight from the pointer
 * position — no state per move, so nothing re-renders while dragging and the
 * columns track the cursor exactly.
 *
 * Touch is left to the browser's own panning, which is better than anything
 * reimplemented here, so only a mouse starts a drag.
 */
export function useDragScroll(ref: RefObject<HTMLDivElement | null>) {
  const [dragging, setDragging] = useState(false);
  const origin = useRef({ x: 0, scrollLeft: 0, active: false });

  const onPointerDown = (e: ReactPointerEvent<HTMLDivElement>) => {
    const el = ref.current;
    if (!el || e.pointerType !== 'mouse' || e.button !== 0) return;
    origin.current = { x: e.clientX, scrollLeft: el.scrollLeft, active: true };
  };

  const onPointerMove = (e: ReactPointerEvent<HTMLDivElement>) => {
    const el = ref.current;
    if (!el || !origin.current.active) return;
    const dx = e.clientX - origin.current.x;

    // Capture once the movement is past a click's jitter, so a plain click
    // keeps its target. The columns have been following since the first
    // pixel either way.
    if (!dragging && Math.abs(dx) >= DRAG_THRESHOLD_PX) {
      el.setPointerCapture(e.pointerId);
      setDragging(true);
    }

    const target = origin.current.scrollLeft - dx;
    el.scrollLeft = target;

    // Re-anchor whenever the browser clamps at either end. Without this,
    // pulling 100px further into a wall builds up 100px of debt that has to
    // be paid back before the columns move again — which is exactly what
    // "it takes effort to get going at the edges" is.
    if (el.scrollLeft !== target) {
      origin.current.x = e.clientX;
      origin.current.scrollLeft = el.scrollLeft;
    }
  };

  const endDrag = (e: ReactPointerEvent<HTMLDivElement>) => {
    const el = ref.current;
    origin.current.active = false;
    if (el?.hasPointerCapture(e.pointerId) === true) el.releasePointerCapture(e.pointerId);
    setDragging(false);
  };

  return {
    dragging,
    dragHandlers: {
      onPointerDown,
      onPointerMove,
      onPointerUp: endDrag,
      onPointerCancel: endDrag,
    },
  };
}
