import { useRef, useState } from 'react';
import { useCreateBookingSeries } from '../../hooks/useCreateBookingSeries';
import { seriesConflictsFrom } from '../../lib/seriesConflicts';
import { buildSeriesPayload } from './buildSeriesPayload';
import type { CreateBookingDto } from '../../schemas/booking.schema';
import type { SeriesOptions } from './useSeriesOptions';
import type { BookingSeriesConflict, CreateBookingSeriesRequest } from '@/shared/types/api.types';

/**
 * Sends the create-booking form as a weekly series, and keeps what the
 * "create skipping these dates" retry needs: the request the server refused
 * and the dates it named. The dates live in state of their own (not read off
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
    submitSkippingConflicts: (onDone: () => void) => {
      if (lastRequest.current) send({ ...lastRequest.current, skip_conflicts: true }, onDone);
    },
    dismissConflicts: () => {
      setConflicts([]);
      createSeries.reset();
    },
  };
}
