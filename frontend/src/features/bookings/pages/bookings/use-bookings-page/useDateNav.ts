import { useCallback, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { format } from 'date-fns/format';
import { addDays } from 'date-fns/addDays';
import { parseISO } from 'date-fns/parseISO';
import { isValid } from 'date-fns/isValid';
import { es } from 'date-fns/locale/es';

const DATE_PARAM_RE = /^\d{4}-\d{2}-\d{2}$/;

/** Parses the `?date=` search param, falling back to today when it is missing or malformed. */
function parseDateParam(param: string | null): string {
  if (param && DATE_PARAM_RE.test(param) && isValid(parseISO(param))) return param;
  return format(new Date(), 'yyyy-MM-dd');
}

/**
 * The selected day lives in the URL (`?date=`), not local state, so a
 * refresh or the browser's back button lands the owner back on the day they
 * were looking at instead of always resetting to today.
 */
export function useDateNav() {
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedDate = parseDateParam(searchParams.get('date'));
  const [calendarOpen, setCalendarOpen] = useState(false);

  const setSelectedDate = useCallback(
    (date: string) => {
      setSearchParams(
        (current) => {
          const next = new URLSearchParams(current);
          next.set('date', date);
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  const dateLabel = useMemo(() => {
    const d = parseISO(selectedDate);
    return format(d, "EEEE d 'de' MMMM", { locale: es });
  }, [selectedDate]);

  const todayStr = format(new Date(), 'yyyy-MM-dd');
  const isToday = selectedDate === todayStr;
  const isPast = selectedDate < todayStr;

  // Reads the current date from the URL itself (not the closed-over
  // `selectedDate`) so two quick clicks never race against a stale value.
  const shiftDate = useCallback(
    (deltaDays: number) => {
      setSearchParams(
        (current) => {
          const currentDate = parseDateParam(current.get('date'));
          const next = new URLSearchParams(current);
          next.set('date', format(addDays(parseISO(currentDate), deltaDays), 'yyyy-MM-dd'));
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  // Not memoized: all three end up on the `onClick` of a plain shadcn
  // `Button` in DateNavControls, which is not wrapped in `memo`. A stable
  // identity buys nothing there, and the hook's own consumers re-render on
  // every date change anyway (PERF-04). `shiftDate` and `setSelectedDate`
  // keep their `useCallback`: both are effect-free URL writers handed to
  // other components as props.
  const handlePrevDay = () => {
    shiftDate(-1);
  };
  const handleNextDay = () => {
    shiftDate(1);
  };

  const handleGoToToday = () => {
    setSelectedDate(format(new Date(), 'yyyy-MM-dd'));
  };

  return {
    selectedDate,
    setSelectedDate,
    dateLabel,
    isToday,
    isPast,
    calendarOpen,
    setCalendarOpen,
    handlePrevDay,
    handleNextDay,
    handleGoToToday,
  };
}
