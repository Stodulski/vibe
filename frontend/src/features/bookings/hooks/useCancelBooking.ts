import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { bookingsApi } from '../api/bookings.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import type {
  Booking,
  BookingsListResponse,
  CancelBookingRequest,
  CancelBookingResponse,
} from '@/shared/types/api.types';

// The complex has to hand this one back by hand — every other outcome
// either already moved automatically or never needed to move at all.
// Mirrors the public cancel flow's `NEEDS_HUMAN_ACTION`
// (`CancelledResultState.tsx`) — `refund.status` is the server's own state
// machine, used here only to pick a toast tone, never to write copy.
const NEEDS_HUMAN_ACTION = new Set<CancelBookingResponse['refund']['status']>(['manual']);

export function useCancelBooking(complexId: string, date: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bookingId, data }: { bookingId: string; data?: CancelBookingRequest }) =>
      bookingsApi.cancel(complexId, bookingId, data),
    onMutate: async ({ bookingId }) => {
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
                  b.id === bookingId ? { ...b, status: 'cancelled' as const } : b,
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
      toast.error(getHttpErrorMessage(error, ES_AR.bookings.cancelError));
    },
    onSuccess: ({ refund }) => {
      // `refund.message` is deliberately ignored here. It is the server's
      // sentence for the person who booked ("el complejo tiene que devolverte
      // este dinero a mano. Comunicate con ellos."), and on the dashboard the
      // reader *is* the complex — it told owners to contact themselves. The
      // server's own comment says as much: the machine-readable `status`
      // travels alongside precisely so a frontend can write its own copy.
      //
      // A split refund still needs the owner's attention for the manual
      // portion even when the automatic (MP) part issued cleanly, so it gets
      // the same warning tone as a fully-manual one.
      const isSplit = refund.manual_amount !== undefined && refund.manual_amount > 0;
      const notify = NEEDS_HUMAN_ACTION.has(refund.status) || isSplit ? toast.warning : toast.success;
      const description = isSplit
        ? `${ES_AR.bookings.refundOwner[refund.status]} ${ES_AR.bookings.refundOwnerManualPart}`
        : ES_AR.bookings.refundOwner[refund.status];
      notify(ES_AR.bookings.cancelSuccess, { description });
    },
    onSettled: (_data, _error, { bookingId }) => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.bookings.byDate(complexId, date),
      });
      void queryClient.invalidateQueries({
        queryKey: queryKeys.bookings.detail(bookingId),
      });
    },
  });
}
