import { useRef, useState } from 'react';
import { useCreateBookingSeries } from '../../hooks/useCreateBookingSeries';
import { seriesConflictsFrom } from '../../lib/seriesConflicts';
import { buildSeriesPayload } from './buildSeriesPayload';
import type { CreateBookingDto } from '../../schemas/booking.schema';
import type { SeriesOptions } from './useSeriesOptions';
import type { BookingSeriesConflict, CreateBookingSeriesRequest } from '@/shared/types/api.types';

type DateCourt = NonNullable<CreateBookingSeriesRequest['date_courts']>[number];

/**
 * Sends the create-booking form as a weekly series, and keeps what the retry
 * needs: the request the server refused and the dates it named, each with the
 * courts free that day. The dates live in state of their own (not read off
 * the mutation's error) so the list stays on screen while the retry is in
 * flight instead of blinking back to the form.
 */
export function useSeriesSubmit(complexId: string, series: SeriesOptions) {
  const createSeries = useCreateBookingSeries(complexId);
  const lastRequest = useRef<CreateBookingSeriesRequest | null>(null);
  const [conflicts, setConflicts] = useState<BookingSeriesConflict[]>([]);

  const send = (request: CreateBookingSeriesRequest, onDone: () => void) => {
    lastRequest.current = request;
    createSeries.mutate(request, {
      onSuccess: onDone,
      onError: (error) => {
        setConflicts(seriesConflictsFrom(error));
      },
    });
  };

  return {
    createSeries,
    /** The dates the server refused, while the owner decides what to do about them. */
    conflicts,
    submit: (data: CreateBookingDto, onDone: () => void) => {
      setConflicts([]);
      send(buildSeriesPayload(data, series.toEnd()), onDone);
    },
    /**
     * Sends the refused series again, on the same court: the dates in
     * `dateCourts` are booked on the court picked for them, every other
     * refused date is left out. Only the dates the owner was shown are named
     * (`skip_dates`): one taken since is refused again instead of skipped.
     */
    submitWithChoices: (dateCourts: DateCourt[], onDone: () => void) => {
      if (!lastRequest.current) return;
      send(
        {
          ...lastRequest.current,
          skip_conflicts: true,
          skip_dates: conflicts.map((c) => c.date),
          ...(dateCourts.length > 0 ? { date_courts: dateCourts } : {}),
        },
        onDone,
      );
    },
    dismissConflicts: () => {
      setConflicts([]);
      createSeries.reset();
    },
  };
}
