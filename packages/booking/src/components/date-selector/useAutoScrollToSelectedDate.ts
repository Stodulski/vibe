import { useEffect, useRef } from 'react';
import { isSameDay } from 'date-fns/isSameDay';

/**
 * Brings the selected date's button into view in the strip. The first
 * placement, on load, moves only the strip sideways: `scrollIntoView` also
 * scrolls the page when the strip sits below the fold (desktop), so loading the
 * booking would jump the visitor down. Later selections, such as a "Ver el"
 * from the full-day panel, use `scrollIntoView` smoothly, so the chosen day is
 * also brought into view when the strip itself is off screen.
 *
 * Deliberately reads `days` from a ref updated every render rather than
 * listing it as a dependency — re-scrolling every time more days load (infinite
 * scroll) would fight the user's own scroll position — so the effect still only
 * re-runs on `selectedDate`, but the dependency array no longer lies about it.
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
    const first = !mountedRef.current;
    mountedRef.current = true;
    const frame = requestAnimationFrame(() => {
      const strip = scrollRef.current;
      if (!strip) return;
      const idx = daysRef.current.findIndex((d) => isSameDay(d, selectedDate));
      const btn = strip.children[idx];
      if (!(btn instanceof HTMLElement)) return;
      if (first) {
        centreInStrip(strip, btn);
      } else {
        btn.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' });
      }
    });
    return () => {
      cancelAnimationFrame(frame);
    };
  }, [scrollRef, selectedDate]);
}

/**
 * Moves the strip, and nothing else, so the day sits in its middle. Setting
 * `scrollLeft` cannot scroll the page; `scrollIntoView` can.
 */
function centreInStrip(strip: HTMLElement, btn: HTMLElement) {
  const stripBox = strip.getBoundingClientRect();
  const btnBox = btn.getBoundingClientRect();
  strip.scrollLeft += btnBox.left - stripBox.left - (stripBox.width - btnBox.width) / 2;
}
