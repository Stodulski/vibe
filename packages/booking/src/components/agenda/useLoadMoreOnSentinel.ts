import { useEffect, useRef } from 'react';

/**
 * Calls `onNearEnd` when the sentinel the returned ref is attached to enters the
 * viewport, so a long list grows as its reader scrolls down to it. It watches
 * only while `enabled` holds, and starts watching again whenever `resetKey`
 * changes, so a sentinel still in view loads the next page at once. Where the
 * browser has no IntersectionObserver it does nothing.
 *
 * `onNearEnd` is read from a ref updated every render, as in
 * `useLoadMoreOnScroll`: the observer is created once per watch, and it always
 * calls the latest callback.
 */
export function useLoadMoreOnSentinel(onNearEnd: () => void, enabled: boolean, resetKey: number) {
  const sentinelRef = useRef<HTMLDivElement>(null);
  const onNearEndRef = useRef(onNearEnd);

  useEffect(() => {
    onNearEndRef.current = onNearEnd;
  });

  useEffect(() => {
    const el = sentinelRef.current;
    if (!enabled || !el || typeof IntersectionObserver === 'undefined') return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) onNearEndRef.current();
      },
      { rootMargin: '200px 0px' },
    );
    observer.observe(el);
    return () => {
      observer.disconnect();
    };
  }, [enabled, resetKey]);

  return sentinelRef;
}
