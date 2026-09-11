import { useCallback, useEffect, useRef, useState } from 'react';

/**
 * Tracks an element's own width, so a component can decide how much content
 * fits instead of guessing from a breakpoint. Returns 0 until the element is
 * measured — including in jsdom, where layout never happens, so callers need a
 * sensible fallback for that case.
 */
export function useElementWidth() {
  const [width, setWidth] = useState(0);
  const observerRef = useRef<ResizeObserver>(undefined);

  const ref = useCallback((node: HTMLElement | null) => {
    observerRef.current?.disconnect();
    if (!node) return;

    setWidth(node.getBoundingClientRect().width);

    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) setWidth(entry.contentRect.width);
    });
    observer.observe(node);
    observerRef.current = observer;
  }, []);

  useEffect(
    () => () => {
      observerRef.current?.disconnect();
    },
    [],
  );

  return { ref, width };
}
