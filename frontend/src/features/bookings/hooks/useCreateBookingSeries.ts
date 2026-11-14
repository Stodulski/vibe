import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { bookingSeriesApi } from '../api/bookingSeries.api';
import { seriesConflictsFrom } from '../lib/seriesConflicts';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useIdempotentMutation, type WithAttemptKey } from '@/shared/lib/idempotency';
import type { CreateBookingSeriesRequest } from '@/shared/types/api.types';

const t = ES_AR;

export function useCreateBookingSeries(complexId: string) {
  const queryClient = useQueryClient();

  return useIdempotentMutation({
    // `attemptKey` goes to the header, never the body (see `useCreateBooking`).
    mutationFn: ({ attemptKey, ...data }: WithAttemptKey<CreateBookingSeriesRequest>) =>
      bookingSeriesApi.create(complexId, data, attemptKey),
    onSuccess: ({ bookings, skipped }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.bookings.byComplex(complexId) });
      toast.success(t.bookings.series.createSuccess(bookings.length, skipped.length));
    },
    onError: (error: unknown) => {
      // The dates are listed in the modal itself, with the choice to skip them.
      if (seriesConflictsFrom(error).length > 0) return;
      toast.error(getHttpErrorMessage(error, t.bookings.series.createError));
    },
  });
}
