import { useEffect, useRef, useState } from 'react';

/**
 * Whether a `position: sticky` element is currently pinned.
 *
 * CSS gives no selector for it, so this watches a zero-height sentinel placed
 * immediately above the sticky element: once the sentinel leaves the top of
 * the scrollport, the element below it must be stuck.
 *
 * The scrollport is the window: the page scrolls, and the app shell's chrome
 * holds its place by being `fixed`. The sticky element parks *below* that
 * chrome, so the sentinel has to be watched against a viewport shortened by
 * the same amount — otherwise "stuck" is reported a masthead's height late,
 * after the names have already slid under the bar.
 */
const CHROME_HEIGHT_PX = 64;

export function useStuck() {
  const sentinelRef = useRef<HTMLDivElement>(null);
  const [stuck, setStuck] = useState(false);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        setStuck(entry !== undefined && !entry.isIntersecting);
      },
      { root: null, rootMargin: `-${String(CHROME_HEIGHT_PX)}px 0px 0px 0px`, threshold: 0 },
    );
    observer.observe(sentinel);
    return () => {
      observer.disconnect();
    };
  }, []);

  return { sentinelRef, stuck };
}
