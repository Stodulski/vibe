import { useEffect, useRef } from 'react';
import { isSameDay } from 'date-fns/isSameDay';

/**
 * Scrolls the selected date's button into view: instantly on first mount,
 * smoothly on subsequent selection changes. Deliberately reads `days` from a
 * ref updated every render rather than listing it as a dependency —
 * re-scrolling every time more days load (infinite scroll) would fight the
 * user's own scroll position — so the effect still only re-runs on
 * `selectedDate`, but the dependency array no longer lies about it.
 */
export function useAutoScrollToSelectedDate(
  scrollRef: React.RefObject<HTMLDivElement | null>,
  days: Date[],
  selectedDate: Date,
) {
  const mountedRef = useRef(false);
  const daysRef = useRef(days);

  // Refs can't be written during render (React flags it, and rightly so —
  // it's a side effect). No deps: this runs after every commit, keeping
  // `daysRef` current before the effect below ever reads it.
  useEffect(() => {
    daysRef.current = days;
  });

  useEffect(() => {
    const scrollToSelected = (behavior: ScrollBehavior) => {
      if (!scrollRef.current) return;
      const idx = daysRef.current.findIndex((d) => isSameDay(d, selectedDate));
      const btn = scrollRef.current.children[idx];
      if (btn instanceof HTMLElement) {
        btn.scrollIntoView({ behavior, block: 'nearest', inline: 'center' });
      }
    };

    // First mount: scroll instantly without animation. Subsequent date
    // changes: scroll smoothly.
    const behavior: ScrollBehavior = mountedRef.current ? 'smooth' : 'instant';
    mountedRef.current = true;
    const frame = requestAnimationFrame(() => {
      scrollToSelected(behavior);
    });
    return () => {
      cancelAnimationFrame(frame);
    };
  }, [scrollRef, selectedDate]);
}
