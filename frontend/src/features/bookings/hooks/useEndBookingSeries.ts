import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { bookingSeriesApi } from '../api/bookingSeries.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';

const t = ES_AR;

export function useEndBookingSeries(complexId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ seriesId, fromDate }: { seriesId: string; fromDate: string }) =>
      bookingSeriesApi.end(complexId, seriesId, fromDate),
    onSuccess: ({ cancelled, kept }) => {
      // Every booking query: lists, and the cancelled occurrences' own detail entries.
      void queryClient.invalidateQueries({ queryKey: queryKeys.bookings.all });
      // The freed dates stop being blocks on the grid.
      void queryClient.invalidateQueries({ queryKey: queryKeys.courts.blockedSlotsBase(complexId) });
      toast.success(t.bookings.series.endSuccess(cancelled.length, kept.length));
    },
    onError: (error: unknown) => {
      // Same shape as the booking errors: a title, then what the server said.
      const detail = getHttpErrorMessage(error, '');
      toast.error(t.bookings.series.endError, detail ? { description: detail } : undefined);
    },
  });
}
