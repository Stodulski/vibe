import { describe, it, expect } from 'vitest';
import { createElement, type ReactNode } from 'react';
import { renderHook, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { format } from 'date-fns/format';
import { useDateNav } from './useDateNav';

function wrapperFor(initialEntries: string[]) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return createElement(MemoryRouter, { initialEntries }, children);
  };
}

describe('useDateNav — reading the URL', () => {
  it('opens the day named by ?date=', () => {
    const { result } = renderHook(() => useDateNav(), {
      wrapper: wrapperFor(['/bookings?date=2026-09-10']),
    });
    expect(result.current.selectedDate).toBe('2026-09-10');
  });

  it('falls back to today for a missing date param', () => {
    const { result } = renderHook(() => useDateNav(), {
      wrapper: wrapperFor(['/bookings']),
    });
    expect(result.current.selectedDate).toBe(format(new Date(), 'yyyy-MM-dd'));
  });

  it('falls back to today for a malformed date param', () => {
    const { result } = renderHook(() => useDateNav(), {
      wrapper: wrapperFor(['/bookings?date=not-a-date']),
    });
    expect(result.current.selectedDate).toBe(format(new Date(), 'yyyy-MM-dd'));
  });
});

describe('useDateNav — changing the day', () => {
  it('updates the URL when a new day is picked directly', () => {
    const { result, rerender } = renderHook(() => useDateNav(), {
      wrapper: wrapperFor(['/bookings?date=2026-09-10']),
    });

    act(() => {
      result.current.setSelectedDate('2026-09-15');
    });
    rerender();
    expect(result.current.selectedDate).toBe('2026-09-15');
  });

  it('moves one day forward and back, reading the current URL rather than a stale value', () => {
    const { result, rerender } = renderHook(() => useDateNav(), {
      wrapper: wrapperFor(['/bookings?date=2026-09-10']),
    });

    act(() => {
      result.current.handleNextDay();
    });
    rerender();
    expect(result.current.selectedDate).toBe('2026-09-11');

    act(() => {
      result.current.handlePrevDay();
    });
    rerender();
    expect(result.current.selectedDate).toBe('2026-09-10');
  });
});
