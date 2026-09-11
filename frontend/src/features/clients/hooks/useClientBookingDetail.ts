import { useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { format } from 'date-fns';
import { useBookingModals, useBookingActions } from '@/features/bookings';
import { queryKeys } from '@/shared/lib/queryKeys';

/**
 * Opens/manages a booking's detail sheet (+ its cancel/confirm-payment
 * actions) from within the client detail drawer, so clicking a row in
 * "Reservas recientes" doesn't have to leave the drawer — reuses the same
 * modals/actions the /bookings page and dashboard drawer are built on.
 */
export function useClientBookingDetail(complexId: string | null, clientId: string) {
  const queryClient = useQueryClient();
  const today = format(new Date(), 'yyyy-MM-dd');
  const modals = useBookingModals(today, complexId);
  const selectedDate = modals.selectedBooking?.date ?? today;

  const invalidateClientDetail = useCallback(() => {
    if (!complexId) return;
    void queryClient.invalidateQueries({ queryKey: queryKeys.clients.detail(complexId, clientId) });
  }, [queryClient, complexId, clientId]);

  const actions = useBookingActions({
    selectedComplexId: complexId,
    selectedDate,
    selectedBooking: modals.selectedBooking,
    setSelectedBooking: modals.setSelectedBooking,
    setDetailOpen: modals.setDetailOpen,
    setCancelOpen: modals.setCancelOpen,
    setPaymentOpen: modals.setPaymentOpen,
    setManualRefundOpen: modals.setManualRefundOpen,
    onActionSuccess: invalidateClientDetail,
  });

  return { ...modals, ...actions };
}
