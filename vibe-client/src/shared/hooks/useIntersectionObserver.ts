import { useEffect, useRef } from 'react';

/**
 * Calls `onIntersect` when the observed element enters the viewport.
 * Used as a sentinel for infinite-scroll lists.
 */
export function useIntersectionObserver(onIntersect: () => void, enabled: boolean) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!enabled) return;
    const el = ref.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) onIntersect();
      },
      { rootMargin: '200px' },
    );

    observer.observe(el);
    return () => {
      observer.disconnect();
    };
  }, [onIntersect, enabled]);

  return ref;
}
