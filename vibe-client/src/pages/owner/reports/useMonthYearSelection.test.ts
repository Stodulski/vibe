import { createElement, type ReactNode } from 'react';
import { renderHook, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { useMonthYearSelection } from './useMonthYearSelection';

function wrapperFor(initialEntries: string[]) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return createElement(MemoryRouter, { initialEntries }, children);
  };
}

const now = new Date();
const CREATED_AT = '2020-01-15T00:00:00.000Z';

describe('useMonthYearSelection — reading the URL', () => {
  it('opens the month/year named by the query', () => {
    const { result } = renderHook(() => useMonthYearSelection(CREATED_AT), {
      wrapper: wrapperFor(['/reports?month=3&year=2021']),
    });
    expect(result.current.month).toBe(3);
    expect(result.current.year).toBe(2021);
  });

  it('falls back to the current month/year when the query is empty', () => {
    const { result } = renderHook(() => useMonthYearSelection(CREATED_AT), {
      wrapper: wrapperFor(['/reports']),
    });
    expect(result.current.month).toBe(now.getMonth() + 1);
    expect(result.current.year).toBe(now.getFullYear());
  });

  it('clamps a month outside 1-12 and a year outside range instead of showing nothing', () => {
    const { result } = renderHook(() => useMonthYearSelection(CREATED_AT), {
      wrapper: wrapperFor(['/reports?month=99&year=1900']),
    });
    // month=99 is not a valid ISO month, so it falls back to the current one.
    expect(result.current.month).toBe(now.getMonth() + 1);
    // year=1900 predates the complex, so it clamps up to its creation year.
    expect(result.current.year).toBe(2020);
  });

  it('falls back to the current month/year for a non-numeric query', () => {
    const { result } = renderHook(() => useMonthYearSelection(CREATED_AT), {
      wrapper: wrapperFor(['/reports?month=abc&year=xyz']),
    });
    expect(result.current.month).toBe(now.getMonth() + 1);
    expect(result.current.year).toBe(now.getFullYear());
  });
});

describe('useMonthYearSelection — changing the period', () => {
  it('writes the new month to the URL', () => {
    const { result, rerender } = renderHook(() => useMonthYearSelection(CREATED_AT), {
      wrapper: wrapperFor(['/reports?month=1&year=2021']),
    });

    act(() => {
      result.current.handleMonthChange(5);
    });
    rerender();
    expect(result.current.month).toBe(5);
    expect(result.current.year).toBe(2021);
  });

  it('writes the new year and clamps the month if it falls outside the new year range', () => {
    const { result, rerender } = renderHook(() => useMonthYearSelection(CREATED_AT), {
      wrapper: wrapperFor([`/reports?month=${String(now.getMonth() + 1)}&year=${String(now.getFullYear())}`]),
    });

    act(() => {
      result.current.handleYearChange(2020);
    });
    rerender();
    expect(result.current.year).toBe(2020);
    // 2020 is the creation year, so month can't go earlier than January.
    expect(result.current.month).toBeGreaterThanOrEqual(1);
  });
});
