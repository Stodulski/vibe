import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useOverlayScale } from './useOverlayScale';

// Built once per test and handed to the hook as a stable ref, the way a
// component's `useRef` would be: a fresh object on every render would re-run
// the hook's effect endlessly.
function frameOfSize(width: number, height: number) {
  const el = document.createElement('div');
  el.getBoundingClientRect = () => ({ width, height }) as DOMRect;
  return { current: el };
}

describe('useOverlayScale', () => {
  it('scales a fixed-size layer to cover the frame it is placed over', () => {
    const frame = frameOfSize(448, 44);
    const { result } = renderHook(() => useOverlayScale(frame, 400, 40));

    expect(result.current).toEqual({ x: 1.12, y: 1.1 });
  });

  // The frame has no size before layout (and none at all in a hidden tab);
  // collapsing the layer to 0×0 there would leave a button nothing can click.
  it('stays at 1:1 while the frame has no size yet', () => {
    const frame = frameOfSize(0, 0);
    const { result } = renderHook(() => useOverlayScale(frame, 400, 40));

    expect(result.current).toEqual({ x: 1, y: 1 });
  });

  it('stays at 1:1 when there is no frame to measure', () => {
    const frame = { current: null };
    const { result } = renderHook(() => useOverlayScale(frame, 400, 40));

    expect(result.current).toEqual({ x: 1, y: 1 });
  });
});
