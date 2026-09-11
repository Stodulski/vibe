import { renderHook } from '@testing-library/react';
import type { MockInstance } from 'vitest';
import { useAutoScrollToSelectedDate } from './useAutoScrollToSelectedDate';

describe('useAutoScrollToSelectedDate', () => {
  let rafSpy: MockInstance<typeof window.requestAnimationFrame>;
  let cancelSpy: MockInstance<typeof window.cancelAnimationFrame>;

  beforeEach(() => {
    // Never auto-runs the callback — these tests only care about whether a
    // frame was scheduled/cancelled, not about the scroll itself.
    rafSpy = vi.spyOn(window, 'requestAnimationFrame').mockReturnValue(123);
    cancelSpy = vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => {
      /* no-op */
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  const scrollRef = { current: null };

  it('schedules a frame on mount', () => {
    renderHook(
      ({ days, selectedDate }) => {
        useAutoScrollToSelectedDate(scrollRef, days, selectedDate);
      },
      { initialProps: { days: [new Date('2026-03-20')], selectedDate: new Date('2026-03-20') } },
    );
    expect(rafSpy).toHaveBeenCalledTimes(1);
  });

  it('cancels the pending animation frame on unmount instead of leaking it', () => {
    const { unmount } = renderHook(
      ({ days, selectedDate }) => {
        useAutoScrollToSelectedDate(scrollRef, days, selectedDate);
      },
      { initialProps: { days: [new Date('2026-03-20')], selectedDate: new Date('2026-03-20') } },
    );
    unmount();
    expect(cancelSpy).toHaveBeenCalledWith(123);
  });

  it('does not schedule a new frame merely because `days` grew (infinite scroll), only on selectedDate change', () => {
    // The same `Date` instance across renders, the way `DateSelector` holds
    // it in state — a fresh `new Date(...)` per render would change identity
    // on its own and defeat the point of this test.
    const day1 = new Date('2026-03-20');
    const day2 = new Date('2026-03-21');

    const { rerender } = renderHook(
      ({ days, selectedDate }) => {
        useAutoScrollToSelectedDate(scrollRef, days, selectedDate);
      },
      { initialProps: { days: [day1], selectedDate: day1 } },
    );
    expect(rafSpy).toHaveBeenCalledTimes(1);

    rerender({ days: [day1, day2], selectedDate: day1 });
    expect(rafSpy).toHaveBeenCalledTimes(1);

    rerender({ days: [day1, day2], selectedDate: day2 });
    expect(rafSpy).toHaveBeenCalledTimes(2);
  });
});
