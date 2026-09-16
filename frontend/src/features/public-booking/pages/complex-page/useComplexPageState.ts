import { useState, useCallback, useEffect, useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import { format } from 'date-fns/format';
import type { SelectedSlot } from '@/features/public-booking';
import { useUnsavedWork } from '@/shared/hooks/useUnsavedWork';
import type { DurationMinutes, Sport } from '@/shared/types/api.types';
import { parseDateParam } from './schema';

// Matches the availability endpoint's own default, so the initial fetch (made
// before the visitor has answered anything) asks for the same duration the
// picker will show once it renders. This is a request default only — it must
// never be read as "what the picker should show as selected"; see
// `selectedDuration` below, which is what the picker actually uses.
const DEFAULT_DURATION: DurationMinutes = 90;

const SPORTS: readonly Sport[] = ['padel', 'tennis', 'soccer', 'basketball', 'volleyball', 'hockey', 'pickleball'];
const DURATIONS: readonly DurationMinutes[] = [60, 90, 120];

/** The flow's answers as the URL carries them; anything malformed is unanswered. */
function readFlow(params: URLSearchParams) {
  const durationParam = Number(params.get('duration'));
  const time = params.get('time');
  return {
    sport: SPORTS.find((s) => s === params.get('sport')) ?? null,
    duration: DURATIONS.find((d) => d === durationParam) ?? null,
    time: time && /^([01]\d|2[0-3]):[0-5]\d$/.test(time) ? time : null,
  };
}

/** Merges into the query, dropping keys set to null. Never pushes history. */
function useQueryPatch() {
  const [, setSearchParams] = useSearchParams();
  return useCallback(
    (patch: Record<string, string | null>) => {
      setSearchParams(
        (current) => {
          const next = new URLSearchParams(current);
          for (const [key, value] of Object.entries(patch)) {
            if (value === null) next.delete(key);
            else next.set(key, value);
          }
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );
}

// Debounce the date string for API calls (300ms) to avoid rapid-fire requests
// when the user taps multiple dates quickly. The visual selection is instant.
function useDebouncedDateStr(dateStr: string) {
  const [debouncedDateStr, setDebouncedDateStr] = useState(dateStr);
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedDateStr(dateStr);
    }, 300);
    return () => {
      clearTimeout(timer);
    };
  }, [dateStr]);
  return debouncedDateStr;
}

// Every change here also drops the open hour: a different day, sport or
// duration is a different grid, and the question belongs to the old one.
function useFlowChangeHandlers(setSelectedSlot: (slot: null) => void, patchParams: ReturnType<typeof useQueryPatch>) {
  const handleDateSelect = useCallback(
    (date: Date) => {
      setSelectedSlot(null);
      patchParams({ date: format(date, 'yyyy-MM-dd'), time: null });
    },
    [setSelectedSlot, patchParams],
  );

  const handleSportFilter = useCallback(
    (sport: Sport | null) => {
      setSelectedSlot(null);
      patchParams({ sport, time: null });
    },
    [setSelectedSlot, patchParams],
  );

  const handleDurationChange = useCallback(
    (newDuration: DurationMinutes) => {
      setSelectedSlot(null);
      patchParams({ duration: String(newDuration), time: null });
    },
    [setSelectedSlot, patchParams],
  );

  return { handleDateSelect, handleSportFilter, handleDurationChange };
}

/**
 * The booking flow's answers, kept in the URL: `?date=&sport=&duration=&time=`.
 *
 * The URL is the state so that leaving and coming back lands on the last
 * step taken rather than the first. The confirm page's "change hour" link and
 * the browser's own back button both return here with the same query, and the
 * page reopens exactly where the visitor was — sport and duration answered,
 * the court question open at the hour they had chosen. Only what narrows the
 * inventory lives here; the chosen court does not, since it is on its way to
 * the confirm page and is invalidated by fresh availability anyway.
 *
 * `date`, `sport`, `duration` and `time` are derived from `searchParams` on
 * every render rather than copied into their own `useState` — the URL is the
 * single source of truth, so a browser back/forward within this route (which
 * changes `searchParams` without a `patchParams` call) is reflected
 * immediately instead of leaving stale state behind. `selectedSlot` is the
 * only piece that stays in `useState`: it is never written to the URL.
 *
 * That asymmetry is exactly what `useUnsavedWork` is told about below. A
 * reload — including one the PWA decides on to pick up a new build while the
 * tab is in the background — lands on the same query, so the day, sport,
 * duration and open hour rebuild themselves and nothing is lost. The chosen
 * court and slot live only in memory, and a client who picks a court, switches
 * to WhatsApp to ask a friend and comes back would find the page reset. So the
 * page counts as busy while, and only while, a slot is selected.
 */
export function useComplexPageState() {
  const [searchParams] = useSearchParams();
  const patchParams = useQueryPatch();
  const flow = readFlow(searchParams);

  // `parseDateParam` returns a fresh `Date` on every call; memoizing on the
  // raw `?date=` string keeps `selectedDate`'s identity stable across
  // unrelated re-renders. That stability matters here — `DateSelector`'s
  // auto-scroll effect depends on `selectedDate` by reference, and a new
  // instance every render would re-trigger it even when the day is unchanged.
  const dateParam = searchParams.get('date');
  const selectedDate = useMemo(() => parseDateParam(dateParam), [dateParam]);

  const sportFilter = flow.sport;
  // Two different questions share `flow.duration`: what to request, and what
  // to paint as chosen. `duration` answers the first — it must never be null,
  // or the initial availability fetch and the picker would disagree about
  // what they are showing. `selectedDuration` answers the second, and stays
  // exactly what the visitor answered (or hasn't): passing the defaulted
  // value to a "selected" prop is what used to make 90 min arrive already
  // highlighted, answering a question nobody had asked yet.
  const duration = flow.duration ?? DEFAULT_DURATION;
  const selectedDuration = flow.duration;
  const pendingStartTime = flow.time;
  // Which questions the URL already answered, so the steps open past them.
  const answeredFromUrl = { sport: flow.sport !== null, duration: flow.duration !== null };

  const [selectedSlot, setSelectedSlot] = useState<SelectedSlot | null>(null);
  useUnsavedWork(selectedSlot !== null);

  const dateStr = format(selectedDate, 'yyyy-MM-dd');
  const debouncedDateStr = useDebouncedDateStr(dateStr);

  const setPendingStartTime = useCallback(
    (startTime: string | null) => {
      patchParams({ time: startTime });
    },
    [patchParams],
  );

  const { handleDateSelect, handleSportFilter, handleDurationChange } = useFlowChangeHandlers(
    setSelectedSlot,
    patchParams,
  );

  return {
    selectedDate,
    selectedSlot,
    setSelectedSlot,
    sportFilter,
    duration,
    selectedDuration,
    pendingStartTime,
    setPendingStartTime,
    answeredFromUrl,
    dateStr,
    debouncedDateStr,
    handleDateSelect,
    handleSportFilter,
    handleDurationChange,
  };
}
