import { useEffect, useRef } from 'react';

/**
 * Loads more days when the horizontally-scrolling strip nears its end
 * (debounced 300ms). Returns the ref to attach to the scroll container.
 *
 * `onNearEnd` is read from a ref updated every render rather than listed as
 * an effect dependency: the effect attaches the scroll listener once, on
 * mount, and the ref means that listener always calls the latest `onNearEnd`
 * without the listener signature promising it is a stable updater — a
 * promise its type never actually makes.
 */
export function useLoadMoreOnScroll(onNearEnd: () => void) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const scrollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const onNearEndRef = useRef(onNearEnd);

  // Refs can't be written during render (React flags it, and rightly so —
  // it's a side effect). No deps: this runs after every commit, keeping
  // `onNearEndRef` current before the scroll listener below ever reads it.
  useEffect(() => {
    onNearEndRef.current = onNearEnd;
  });

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const onScroll = () => {
      if (scrollTimerRef.current) clearTimeout(scrollTimerRef.current);
      scrollTimerRef.current = setTimeout(() => {
        if (el.scrollLeft + el.clientWidth >= el.scrollWidth - 100) {
          onNearEndRef.current();
        }
      }, 300);
    };
    el.addEventListener('scroll', onScroll, { passive: true });
    return () => {
      el.removeEventListener('scroll', onScroll);
      if (scrollTimerRef.current) clearTimeout(scrollTimerRef.current);
    };
  }, []);

  return scrollRef;
}
