import { useState } from 'react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export type SeriesEndMode = 'never' | 'until' | 'weeks';

/** At most one of the two, as the API takes them; neither means the series never ends. */
export interface SeriesEnd {
  ends_on?: string;
  weeks?: number;
}

const MIN_WEEKS = 2;
const MAX_WEEKS = 52;

/**
 * The "repeat every week" choice of the create-booking form. Kept outside the
 * react-hook-form schema: none of it is part of an ordinary booking's body, and
 * `cleanBookingPayload` would otherwise have to strip it.
 */
export function useSeriesOptions(open: boolean) {
  const [repeat, setRepeat] = useState(false);
  const [endMode, setEndMode] = useState<SeriesEndMode>('never');
  const [until, setUntil] = useState('');
  const [weeks, setWeeks] = useState('');
  const [error, setError] = useState<string | undefined>(undefined);

  // The modal stays mounted between opens: start every reopen from a clean
  // choice, adjusted during render like `CancelBookingModal` does with its reason.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (!open) {
      setRepeat(false);
      setEndMode('never');
      setUntil('');
      setWeeks('');
      setError(undefined);
    }
  }

  const weeksCount = Number(weeks);

  /** Checks the end choice against the first date; sets `error` and returns false when it is not usable. */
  const validate = (firstDate: string): boolean => {
    let message: string | undefined;
    if (repeat && endMode === 'until') {
      if (!until) message = t.bookings.series.untilRequired;
      else if (until < firstDate) message = t.bookings.series.untilBeforeStart;
    }
    if (
      repeat &&
      endMode === 'weeks' &&
      !(Number.isInteger(weeksCount) && weeksCount >= MIN_WEEKS && weeksCount <= MAX_WEEKS)
    ) {
      message = t.bookings.series.weeksRange;
    }
    setError(message);
    return message === undefined;
  };

  const toEnd = (): SeriesEnd => {
    if (endMode === 'until' && until) return { ends_on: until };
    if (endMode === 'weeks') return { weeks: weeksCount };
    return {};
  };

  return {
    repeat,
    setRepeat: (value: boolean) => {
      setRepeat(value);
      setError(undefined);
    },
    endMode,
    setEndMode: (mode: SeriesEndMode) => {
      setEndMode(mode);
      setError(undefined);
    },
    until,
    setUntil,
    weeks,
    setWeeks,
    error,
    validate,
    toEnd,
  };
}

export type SeriesOptions = ReturnType<typeof useSeriesOptions>;
