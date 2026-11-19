import { useCallback, useRef, useState } from 'react';

/**
 * The height of the element the returned ref is on, kept current while its text
 * wraps differently (a resize, another choice taking the center). The wheel's
 * choices are absolutely placed, so the wheel takes its height from the one in
 * the center instead of leaving room for the tallest it might ever hold.
 */
export function useFocusedHeight(): [number | null, (el: HTMLElement | null) => void] {
  const [height, setHeight] = useState<number | null>(null);
  const observer = useRef<ResizeObserver | null>(null);
  const ref = useCallback((el: HTMLElement | null) => {
    observer.current?.disconnect();
    observer.current = null;
    if (!el) return;
    const measure = () => {
      setHeight(el.offsetHeight);
    };
    measure();
    if (typeof ResizeObserver !== 'undefined') {
      observer.current = new ResizeObserver(measure);
      observer.current.observe(el);
    }
  }, []);
  return [height, ref];
}
