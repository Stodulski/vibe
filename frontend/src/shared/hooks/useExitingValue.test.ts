import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useExitingValue } from './useExitingValue';

// Typed on `renderHook` rather than on each `initialProps` literal: the literal
// would otherwise narrow to `{ v: string }` and reject the very `null` rerender
// these tests exist to exercise.
const render = (initial: string | null) =>
  renderHook<string | null, { v: string | null }>(({ v }) => useExitingValue(v), {
    initialProps: { v: initial },
  });

describe('useExitingValue', () => {
  it('passes a live value straight through', () => {
    const { result } = render('a');
    expect(result.current).toBe('a');
  });

  it('starts at null when there is nothing to show', () => {
    const { result } = render(null);
    expect(result.current).toBeNull();
  });

  it('keeps the last value once the caller clears it', () => {
    const { result, rerender } = render('a');

    rerender({ v: null });

    expect(result.current).toBe('a');
  });

  it('treats undefined the same as null', () => {
    const { result, rerender } = renderHook<string | null, { v?: string | undefined }>(({ v }) => useExitingValue(v), {
      initialProps: { v: 'a' },
    });

    rerender({ v: undefined });

    expect(result.current).toBe('a');
  });

  it('takes the new value over the retained one as soon as it arrives', () => {
    const { result, rerender } = render('a');

    rerender({ v: null });
    rerender({ v: 'b' });

    expect(result.current).toBe('b');
  });

  it('does not resurrect an older value after the newer one is cleared', () => {
    const { result, rerender } = render('a');

    rerender({ v: 'b' });
    rerender({ v: null });

    expect(result.current).toBe('b');
  });
});
