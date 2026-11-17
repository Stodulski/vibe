import { useEffect, useLayoutEffect, useState } from 'react';

/**
 * The ruler's width, measured whenever the element mounts or resizes; a stand-in before
 * the first measure and where there is no layout. Returns a callback ref for the element.
 */
export function useWidth(): [number, HTMLDivElement | null, (element: HTMLDivElement | null) => void] {
  const [width, setWidth] = useState(360);
  const [element, setElement] = useState<HTMLDivElement | null>(null);
  useLayoutEffect(() => {
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
  }, [element]);
  return [width, element, setElement];
}

/** Hands the native wheel event to the picker: React's onWheel is passive and cannot stop the page from scrolling. */
export function useNativeWheel(element: HTMLDivElement | null, onWheel: (event: WheelEvent) => void) {
  useEffect(() => {
    if (!element) return;
    element.addEventListener('wheel', onWheel, { passive: false });
    return () => {
      element.removeEventListener('wheel', onWheel);
    };
  }, [element, onWheel]);
}
