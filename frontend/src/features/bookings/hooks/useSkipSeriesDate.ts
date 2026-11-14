import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { bookingSeriesApi } from '../api/bookingSeries.api';
import { formatDayMonth } from '../lib/seriesDates';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';

const t = ES_AR;

/** Frees one date of a fixed weekly booking, the block on the grid that has no booking yet. */
export function useSkipSeriesDate(complexId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ seriesId, date }: { seriesId: string; date: string }) =>
      bookingSeriesApi.skip(complexId, seriesId, date),
    onSuccess: ({ skipped }, { seriesId }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.courts.blockedSlotsBase(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.bookings.series(complexId, seriesId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.bookings.byComplex(complexId) });
      toast.success(t.bookings.series.skipSuccess(formatDayMonth(skipped.date)));
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.bookings.series.skipError));
    },
  });
}
