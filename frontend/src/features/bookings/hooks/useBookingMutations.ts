import type { UseMutationResult } from '@tanstack/react-query';
import { useCancelBooking } from '@/features/bookings/hooks/useCancelBooking';
import { useConfirmPayment } from '@/features/bookings/hooks/useConfirmPayment';
import { useUpdateBooking } from '@/features/bookings/hooks/useUpdateBooking';
import { useMarkManualRefund } from '@/features/bookings/hooks/useMarkManualRefund';

// `selectedComplexId` may be `null` before the owner's complex resolves.
// The mutation hooks below are only ever invoked from user-triggered
// handlers (cancel/confirm-payment/no-show), which require a booking to
// already be loaded — that in turn requires `selectedComplexId` to be
// truthy in practice. Rather than trust that and silently fall back to
// `?? ''` (which would hit `PATCH /complexes//bookings/:id` if it were ever
// wrong — 03-owner-pages-dashboard.md M1), each mutation's `mutate`/
// `mutateAsync` becomes an explicit no-op until a real id exists.
function disabledMutation<TData, TError, TVariables, TContext>(
  mutation: UseMutationResult<TData, TError, TVariables, TContext>,
): UseMutationResult<TData, TError, TVariables, TContext> {
  return {
    ...mutation,
    mutate: () => {
      /* no complex selected yet — never reachable from the UI, kept for safety */
    },
    mutateAsync: () => Promise.reject(new Error('useBookingMutations: no complex selected')),
  };
}

export function useBookingMutations(selectedComplexId: string | null, selectedDate: string) {
  const cancelBookingMutation = useCancelBooking(selectedComplexId ?? '', selectedDate);
  const confirmPaymentMutation = useConfirmPayment(selectedComplexId ?? '', selectedDate);
  const updateBookingMutation = useUpdateBooking(selectedComplexId ?? '');
  const markManualRefundMutation = useMarkManualRefund(selectedComplexId ?? '', selectedDate);

  const isReady = selectedComplexId !== null;

  return {
    cancelBooking: isReady ? cancelBookingMutation : disabledMutation(cancelBookingMutation),
    confirmPayment: isReady ? confirmPaymentMutation : disabledMutation(confirmPaymentMutation),
    updateBooking: isReady ? updateBookingMutation : disabledMutation(updateBookingMutation),
    markManualRefund: isReady ? markManualRefundMutation : disabledMutation(markManualRefundMutation),
  };
}
