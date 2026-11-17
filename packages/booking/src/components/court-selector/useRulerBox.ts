import { useEffect, useLayoutEffect, useState, type RefObject } from 'react';

/** The ruler's width, measured; a stand-in before the first measure and where there is no layout. */
export function useWidth(ref: RefObject<HTMLDivElement | null>): number {
  const [width, setWidth] = useState(360);
  useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return;
    const measure = () => {
      if (element.clientWidth > 0) setWidth(element.clientWidth);
    };
    measure();
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(measure);
    observer?.observe(element);
    return () => {
      observer?.disconnect();
    };
  }, [ref]);
  return width;
}

/** Hands the native wheel event to the picker: React's onWheel is passive and cannot stop the page from scrolling. */
export function useNativeWheel(ref: RefObject<HTMLDivElement | null>, onWheel: (event: WheelEvent) => void) {
  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    element.addEventListener('wheel', onWheel, { passive: false });
    return () => {
      element.removeEventListener('wheel', onWheel);
    };
  }, [ref, onWheel]);
}
