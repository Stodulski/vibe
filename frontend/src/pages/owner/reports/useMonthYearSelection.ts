import { useCallback, useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import { MONTH_NAMES } from './constants';

/** Parses `?month=` / `?year=`, falling back to the current month/year for a missing or malformed value. */
function parseMonthYearParams(searchParams: URLSearchParams, now: Date): { month: number; year: number } {
  const monthRaw = searchParams.get('month');
  const yearRaw = searchParams.get('year');
  // `Number(null)` is `0`, not `NaN` — a missing param must be told apart
  // from a present-but-malformed one, or an absent `?year=` would parse as
  // the year 0 instead of falling back to today's.
  const monthParam = monthRaw === null ? NaN : Number(monthRaw);
  const yearParam = yearRaw === null ? NaN : Number(yearRaw);
  const month = Number.isInteger(monthParam) && monthParam >= 1 && monthParam <= 12 ? monthParam : now.getMonth() + 1;
  // Any integer is accepted here, even one far outside the complex's actual
  // range — the caller clamps it to `[minYear, maxYear]` once it knows that
  // range, rather than this function guessing a cutoff of its own.
  const year = Number.isInteger(yearParam) ? yearParam : now.getFullYear();
  return { month, year };
}

/**
 * The chosen month/year live in the URL (`?month=&year=`) so a refresh or
 * the back button returns the owner to the period they were reading instead
 * of always resetting to the current month.
 */
export function useMonthYearSelection(createdAtIso: string | undefined) {
  const [searchParams, setSearchParams] = useSearchParams();
  // Stable references across re-renders — recreating `Date` objects on every
  // render would defeat `availableMonths`' memoization below.
  const now = useMemo(() => new Date(), []);

  const { month: monthParam, year: yearParam } = parseMonthYearParams(searchParams, now);

  const createdAt = useMemo(() => (createdAtIso ? new Date(createdAtIso) : null), [createdAtIso]);
  const minYear = createdAt ? createdAt.getFullYear() : now.getFullYear();
  const maxYear = now.getFullYear();

  // A year the URL cannot answer for (before the complex existed, or in the
  // future) clamps to the nearest one it can, rather than showing nothing.
  const year = Math.min(maxYear, Math.max(minYear, yearParam));

  // Determine which months are selectable for the chosen year.
  const availableMonths = useMemo(() => {
    const createdMonth = year === createdAt?.getFullYear() ? createdAt.getMonth() + 1 : 1;
    const maxMonth = year === now.getFullYear() ? now.getMonth() + 1 : 12;
    return MONTH_NAMES.map((name, i) => ({
      value: i + 1,
      label: name,
      disabled: i + 1 < createdMonth || i + 1 > maxMonth,
    }));
  }, [year, createdAt, now]);

  const minMonth = year === createdAt?.getFullYear() ? createdAt.getMonth() + 1 : 1;
  const maxMonth = year === now.getFullYear() ? now.getMonth() + 1 : 12;
  const month = Math.min(maxMonth, Math.max(minMonth, monthParam));

  const patchMonthYear = useCallback(
    (next: { month: number; year: number }) => {
      setSearchParams(
        (current) => {
          const params = new URLSearchParams(current);
          params.set('month', String(next.month));
          params.set('year', String(next.year));
          return params;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  // Clamp month when year changes and current month is out of range.
  const handleYearChange = (newYear: number) => {
    const clampedYear = Math.max(minYear, Math.min(maxYear, newYear));
    const newMaxMonth = clampedYear === now.getFullYear() ? now.getMonth() + 1 : 12;
    const newMinMonth = clampedYear === createdAt?.getFullYear() ? createdAt.getMonth() + 1 : 1;
    const clampedMonth = Math.min(newMaxMonth, Math.max(newMinMonth, month));
    patchMonthYear({ month: clampedMonth, year: clampedYear });
  };

  const handleMonthChange = (newMonth: number) => {
    const entry = availableMonths.find((m) => m.value === newMonth);
    if (entry && !entry.disabled) patchMonthYear({ month: newMonth, year });
  };

  return { month, year, minYear, maxYear, availableMonths, handleYearChange, handleMonthChange };
}
