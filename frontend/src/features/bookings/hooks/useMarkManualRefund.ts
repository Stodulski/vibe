import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { bookingsApi } from '../api/bookings.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useIdempotencyKey } from '@/shared/lib/idempotency';
import type { Booking, BookingsListResponse } from '@/shared/types/api.types';

export function useMarkManualRefund(complexId: string, date: string) {
  const queryClient = useQueryClient();
  const attempt = useIdempotencyKey();

  return useMutation({
    mutationFn: ({ bookingId }: { bookingId: string }) =>
      bookingsApi.markManualRefund(complexId, bookingId, attempt.current()),
    onMutate: async ({ bookingId }) => {
      attempt.begin();
      await queryClient.cancelQueries({
        queryKey: queryKeys.bookings.byDate(complexId, date),
      });

      const previousQueries = queryClient.getQueriesData<BookingsListResponse>({
        queryKey: queryKeys.bookings.byDate(complexId, date),
      });

      queryClient.setQueriesData<BookingsListResponse>(
        { queryKey: queryKeys.bookings.byDate(complexId, date) },
        (old) =>
          old
            ? {
                ...old,
                bookings: old.bookings.map((b: Booking) =>
                  b.id === bookingId ? { ...b, refund_status: 'full' as const } : b,
                ),
              }
            : old,
      );

      return { previousQueries };
    },
    onError: (error: unknown, _vars, context) => {
      context?.previousQueries.forEach(([key, data]) => {
        queryClient.setQueryData(key, data);
      });
      toast.error(getHttpErrorMessage(error, ES_AR.bookings.manualRefundError));
    },
    onSuccess: (_data, { bookingId }) => {
      toast.success(ES_AR.bookings.manualRefundSuccess);
      void queryClient.invalidateQueries({
        queryKey: queryKeys.bookings.detail(bookingId),
      });
    },
    onSettled: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.bookings.byDate(complexId, date),
      });
    },
  });
}
