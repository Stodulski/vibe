import { useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { format } from 'date-fns/format';
import { useBookingModals, useBookingActions } from '@/features/bookings';
import { queryKeys } from '@/shared/lib/queryKeys';

/**
 * Opens/manages a booking's detail sheet (+ its cancel/confirm-payment
 * actions) directly on the dashboard, so clicking a row in "Próximas
 * reservas" doesn't have to leave the page — reuses the exact same
 * modals/actions the /bookings page itself is built on.
 */
export function useDashboardBookingDetail(complexId: string | null) {
  const queryClient = useQueryClient();
  const today = format(new Date(), 'yyyy-MM-dd');
  const modals = useBookingModals(today, complexId);

  const invalidateDashboardStats = useCallback(() => {
    if (!complexId) return;
    void queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.stats(complexId) });
  }, [queryClient, complexId]);

  const actions = useBookingActions({
    selectedComplexId: complexId,
    // The booking's own day: "Próximas reservas" lists more than today, and
    // the calendar day to refresh after cobrar/cancelar is the booking's.
    selectedDate: modals.selectedBooking?.date ?? today,
    selectedBooking: modals.selectedBooking,
    setSelectedBooking: modals.setSelectedBooking,
    setDetailOpen: modals.setDetailOpen,
    setCancelOpen: modals.setCancelOpen,
    setPaymentOpen: modals.setPaymentOpen,
    setManualRefundOpen: modals.setManualRefundOpen,
    onActionSuccess: invalidateDashboardStats,
  });

  return { ...modals, ...actions };
}
