import { useEffect, useState, type RefObject } from 'react';

export interface OverlayScale {
  x: number;
  y: number;
}

/**
 * The `scale()` that stretches a fixed `baseWidth` x `baseHeight` layer over
 * whatever size `frameRef` currently has, tracked through a ResizeObserver so
 * the layer follows the frame as the layout changes (the auth card is
 * narrower at `md` than at `lg`). Stays at 1:1 until the frame has a size,
 * so a not-yet-laid-out frame never collapses the layer to nothing.
 */
export function useOverlayScale(
  frameRef: RefObject<HTMLElement | null>,
  baseWidth: number,
  baseHeight: number,
): OverlayScale {
  const [scale, setScale] = useState<OverlayScale>({ x: 1, y: 1 });

  useEffect(() => {
    const frame = frameRef.current;
    if (!frame) return;

    const fit = () => {
      const { width, height } = frame.getBoundingClientRect();
      if (width > 0 && height > 0) {
        setScale({ x: width / baseWidth, y: height / baseHeight });
      }
    };
    fit();

    const observer = new ResizeObserver(fit);
    observer.observe(frame);
    return () => {
      observer.disconnect();
    };
  }, [frameRef, baseWidth, baseHeight]);

  return scale;
}
