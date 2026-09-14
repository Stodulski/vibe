import { describe, it, expect } from 'vitest';
import { render, renderHook } from '@testing-library/react';
import { hasUnsavedWork } from '@/shared/lib/unsavedWork';
import { useUnsavedWork } from './useUnsavedWork';

/** A screen that does nothing but declare whether it is holding work. */
function Marker({ isDirty }: { isDirty: boolean }) {
  useUnsavedWork(isDirty);
  return null;
}

// PWA-09: the one place the registration lives, so a guarded form, the public
// slot picker and the confirm form all declare "busy" the same way.
describe('useUnsavedWork', () => {
  it('registers nothing while the caller reports itself clean', () => {
    renderHook(() => {
      useUnsavedWork(false);
    });

    expect(hasUnsavedWork()).toBe(false);
  });

  it('registers while dirty and clears when the caller goes clean again', () => {
    const { rerender } = renderHook(
      ({ isDirty }: { isDirty: boolean }) => {
        useUnsavedWork(isDirty);
      },
      { initialProps: { isDirty: false } },
    );

    rerender({ isDirty: true });
    expect(hasUnsavedWork()).toBe(true);

    rerender({ isDirty: false });
    expect(hasUnsavedWork()).toBe(false);
  });

  it('clears the registration on unmount, even with work still declared', () => {
    const { unmount } = renderHook(() => {
      useUnsavedWork(true);
    });
    expect(hasUnsavedWork()).toBe(true);

    unmount();

    expect(hasUnsavedWork()).toBe(false);
  });

  // `useId` gives each caller its own entry, so one screen going clean cannot
  // speak for another that is still busy.
  it('keeps two callers apart', () => {
    const { rerender } = render(
      <>
        <Marker isDirty />
        <Marker isDirty />
      </>,
    );
    expect(hasUnsavedWork()).toBe(true);

    rerender(
      <>
        <Marker isDirty={false} />
        <Marker isDirty />
      </>,
    );
    expect(hasUnsavedWork()).toBe(true);

    rerender(
      <>
        <Marker isDirty={false} />
        <Marker isDirty={false} />
      </>,
    );
    expect(hasUnsavedWork()).toBe(false);
  });
});
