import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
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

const rect = (left: number, width: number) =>
  ({ left, width, right: left + width, top: 0, bottom: 40, height: 40, x: left, y: 0, toJSON: () => ({}) }) as DOMRect;

/** A strip of buttons, 90px apart from x=100, with a strip box of 300px at x=100 and a scrollLeft it owns. */
function makeStrip(count: number): HTMLDivElement {
  const strip = document.createElement('div');
  strip.getBoundingClientRect = () => rect(100, 300);
  let scrollLeft = 0;
  Object.defineProperty(strip, 'scrollLeft', {
    get: () => scrollLeft,
    set: (value: number) => {
      scrollLeft = value;
    },
    configurable: true,
  });
  for (let i = 0; i < count; i++) {
    const button = document.createElement('button');
    button.getBoundingClientRect = () => rect(100 + i * 90, 80);
    strip.append(button);
  }
  return strip;
}

const days = Array.from({ length: 14 }, (_, i) => new Date(2026, 9, 12 + i));
const dayAt = (index: number): Date => {
  const day = days[index];
  if (!day) throw new Error(`no day at ${String(index)}`);
  return day;
};

describe('useAutoScrollToSelectedDate on the page', () => {
  beforeEach(() => {
    // Run the frame at once, so the scroll the effect schedules happens inside the test.
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
      callback(0);
      return 1;
    });
    vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => {
      /* no-op */
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('moves only the strip sideways on the first placement, centring the day, and never scrolls the page', () => {
    const strip = makeStrip(days.length);
    const intoView = vi.spyOn(Element.prototype, 'scrollIntoView');
    const pageScroll = vi.spyOn(window, 'scrollTo');

    renderHook(() => {
      useAutoScrollToSelectedDate({ current: strip }, days, dayAt(5));
    });

    expect(intoView).not.toHaveBeenCalled();
    expect(pageScroll).not.toHaveBeenCalled();
    // The day (the sixth button, at x 550) starts 450px into the strip; centring its 80px width in the
    // 300px strip scrolls the strip 110px further, to 340px.
    expect(strip.scrollLeft).toBe(340);
  });

  it('still brings a later selection into view, smoothly', () => {
    const strip = makeStrip(days.length);
    const later = strip.children[9] as HTMLElement;
    const intoView = vi.spyOn(later, 'scrollIntoView');

    const { rerender } = renderHook(
      ({ selectedDate }) => {
        useAutoScrollToSelectedDate({ current: strip }, days, selectedDate);
      },
      { initialProps: { selectedDate: dayAt(5) } },
    );
    rerender({ selectedDate: dayAt(9) });

    expect(intoView).toHaveBeenCalledTimes(1);
    expect(intoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'nearest', inline: 'center' });
  });
});
