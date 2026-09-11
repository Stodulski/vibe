import { useCallback, useEffect, useRef } from 'react';

/**
 * Wires the court columns' horizontal scroller to the court names above it.
 *
 * There is no React state here, and there is no room for any: this grid renders
 * a Radix popover per free slot — around a hundred and seventy of them — so a
 * single re-render of the component that owns the scroller costs about half a
 * second. It used to hold whether each of two arrow shortcuts could be shown,
 * and the first pixel of every drag flipped one of them, which froze the drag
 * on the frame after the pointer went down. The arrows are gone; the rule they
 * taught is not.
 *
 * Columns are reached by dragging or by the trackpad, both of which the browser
 * and useDragScroll already handle without anything re-rendering.
 */
export function useColumnScroller() {
  const ref = useRef<HTMLDivElement>(null);
  const trackRef = useRef<HTMLDivElement>(null);
  const frameRef = useRef<HTMLDivElement>(null);

  /**
   * Drags the court names along with the columns. They live outside the
   * scroller so they can stick to the page while it scrolls — inside it,
   * `sticky` would resolve against the scroller, which never moves vertically,
   * and the names would scroll away with the grid.
   *
   * Written straight to the node: this fires on every scroll event, and a
   * re-render per frame is what made dragging judder.
   */
  const syncTrack = useCallback(() => {
    const el = ref.current;
    if (!el) return;

    if (trackRef.current) trackRef.current.style.transform = `translateX(${String(-el.scrollLeft)}px)`;

    // Which edges still have columns behind them, for the frame's fades. A
    // pixel of slack on each side: a scroller's own arithmetic lands on
    // fractional pixels at some zoom levels, and an exact comparison leaves
    // the end fade lit over a grid that has nothing more to show.
    const frame = frameRef.current;
    if (frame) {
      frame.toggleAttribute('data-more-start', el.scrollLeft > 1);
      frame.toggleAttribute('data-more-end', el.scrollLeft + el.clientWidth < el.scrollWidth - 1);
    }
  }, []);

  /**
   * Marks the scroller `data-scrollable` while the columns are wider than the
   * space they have. The grab cursor hangs off that attribute in CSS, so the
   * hand only appears when there is somewhere to drag to — on a wide screen
   * showing every court, the columns are not a scroller and should not offer
   * to be one.
   *
   * Written straight to the node for the same reason `syncTrack` is: this
   * fires on every resize, and holding it in React state would re-render the
   * whole grid to change one cursor.
   */
  const syncScrollable = useCallback(() => {
    const el = ref.current;
    if (el) el.toggleAttribute('data-scrollable', el.scrollWidth > el.clientWidth);
    // The same resize decides which fades belong, so re-ask while we are here.
    syncTrack();
  }, [syncTrack]);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    syncTrack();
    el.addEventListener('scroll', syncTrack, { passive: true });
    return () => {
      el.removeEventListener('scroll', syncTrack);
    };
  }, [syncTrack]);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    syncScrollable();

    // Both boxes: the scroller narrows with the viewport, and its content
    // widens when courts are added — either one can flip the answer.
    const observer = new ResizeObserver(syncScrollable);
    observer.observe(el);
    if (el.firstElementChild) observer.observe(el.firstElementChild);
    return () => {
      observer.disconnect();
    };
  }, [syncScrollable]);

  return { ref, trackRef, frameRef };
}
