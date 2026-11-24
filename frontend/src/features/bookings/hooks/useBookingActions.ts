import { useCallback } from 'react';
import { useUpdateBooking } from '@/features/bookings/hooks/useUpdateBooking';
import { useMarkManualRefund } from '@/features/bookings/hooks/useMarkManualRefund';
import { useMarkAttended } from '@/features/bookings/hooks/useMarkAttended';
import { useCancelBooking } from '@/features/bookings/hooks/useCancelBooking';
import { useConfirmPayment } from '@/features/bookings/hooks/useConfirmPayment';
import { useBookingMutations } from './useBookingMutations';
import type { Booking } from '@/shared/types/api.types';
import type { ConfirmPaymentDto } from '@/features/bookings/schemas/booking.schema';

function useNoShowHandler(
  updateBooking: ReturnType<typeof useUpdateBooking>,
  setDetailOpen: (open: boolean) => void,
  setSelectedBooking: (booking: Booking | null) => void,
  onActionSuccess?: () => void,
) {
  return useCallback(
    (booking: Booking) => {
      updateBooking.mutate(
        { bookingId: booking.id, data: { status: 'no_show' } },
        {
          onSuccess: () => {
            setDetailOpen(false);
            setSelectedBooking(null);
            onActionSuccess?.();
          },
        },
      );
    },
    [updateBooking, setDetailOpen, setSelectedBooking, onActionSuccess],
  );
}

function useAttendedHandler(
  markAttended: ReturnType<typeof useMarkAttended>,
  setDetailOpen: (open: boolean) => void,
  setSelectedBooking: (booking: Booking | null) => void,
  onActionSuccess?: () => void,
) {
  return useCallback(
    (booking: Booking) => {
      markAttended.mutate(
        { bookingId: booking.id },
        {
          onSuccess: () => {
            setDetailOpen(false);
            setSelectedBooking(null);
            onActionSuccess?.();
          },
        },
      );
    },
    [markAttended, setDetailOpen, setSelectedBooking, onActionSuccess],
  );
}

function useConfirmCancelHandler(
  cancelBooking: ReturnType<typeof useCancelBooking>,
  selectedBooking: Booking | null,
  setCancelOpen: (open: boolean) => void,
  setDetailOpen: (open: boolean) => void,
  setSelectedBooking: (booking: Booking | null) => void,
  onActionSuccess?: () => void,
) {
  return useCallback(
    (reason?: string) => {
      if (!selectedBooking) return;
      cancelBooking.mutate(
        { bookingId: selectedBooking.id, ...(reason ? { data: { reason } } : {}) },
        {
          onSuccess: () => {
            setCancelOpen(false);
            setDetailOpen(false);
            setSelectedBooking(null);
            onActionSuccess?.();
          },
        },
      );
    },
    [selectedBooking, cancelBooking, setCancelOpen, setDetailOpen, setSelectedBooking, onActionSuccess],
  );
}

function usePaymentSubmitHandler(
  confirmPayment: ReturnType<typeof useConfirmPayment>,
  selectedBooking: Booking | null,
  setPaymentOpen: (open: boolean) => void,
  setDetailOpen: (open: boolean) => void,
  setSelectedBooking: (booking: Booking | null) => void,
  onActionSuccess?: () => void,
) {
  return useCallback(
    (data: ConfirmPaymentDto) => {
      if (!selectedBooking) return;
      confirmPayment.mutate(
        { bookingId: selectedBooking.id, data },
        {
          onSuccess: () => {
            setPaymentOpen(false);
            setDetailOpen(false);
            setSelectedBooking(null);
            onActionSuccess?.();
          },
        },
      );
    },
    [selectedBooking, confirmPayment, setPaymentOpen, setDetailOpen, setSelectedBooking, onActionSuccess],
  );
}

function useManualRefundHandler(
  markManualRefund: ReturnType<typeof useMarkManualRefund>,
  selectedBooking: Booking | null,
  setManualRefundOpen: (open: boolean) => void,
  setDetailOpen: (open: boolean) => void,
  setSelectedBooking: (booking: Booking | null) => void,
  onActionSuccess?: () => void,
) {
  return useCallback(() => {
    if (!selectedBooking) return;
    markManualRefund.mutate(
      { bookingId: selectedBooking.id },
      {
        onSuccess: () => {
          setManualRefundOpen(false);
          setDetailOpen(false);
          setSelectedBooking(null);
          onActionSuccess?.();
        },
      },
    );
  }, [selectedBooking, markManualRefund, setManualRefundOpen, setDetailOpen, setSelectedBooking, onActionSuccess]);
}

interface UseBookingActionsArgs {
  selectedComplexId: string | null;
  selectedDate: string;
  selectedBooking: Booking | null;
  setSelectedBooking: (booking: Booking | null) => void;
  setDetailOpen: (open: boolean) => void;
  setCancelOpen: (open: boolean) => void;
  setPaymentOpen: (open: boolean) => void;
  setManualRefundOpen: (open: boolean) => void;
  // Called after cancel/confirm-payment/no-show/attended succeeds, in addition
  // to the handlers above — lets a caller outside the /bookings page (e.g. the
  // dashboard's own detail drawer) refresh its own cache, which the mutation
  // hooks below don't touch (they refresh the calendar day, Caja, Clientes and
  // the series panel — `invalidateBookingSideEffects` — not the dashboard).
  onActionSuccess?: (() => void) | undefined;
}

export function useBookingActions({
  selectedComplexId,
  selectedDate,
  selectedBooking,
  setSelectedBooking,
  setDetailOpen,
  setCancelOpen,
  setPaymentOpen,
  setManualRefundOpen,
  onActionSuccess,
}: UseBookingActionsArgs) {
  const { cancelBooking, confirmPayment, updateBooking, markManualRefund, markAttended } = useBookingMutations(
    selectedComplexId,
    selectedDate,
  );

  const handleConfirmCancel = useConfirmCancelHandler(
    cancelBooking,
    selectedBooking,
    setCancelOpen,
    setDetailOpen,
    setSelectedBooking,
    onActionSuccess,
  );
  const handlePaymentSubmit = usePaymentSubmitHandler(
    confirmPayment,
    selectedBooking,
    setPaymentOpen,
    setDetailOpen,
    setSelectedBooking,
    onActionSuccess,
  );
  const handleNoShow = useNoShowHandler(updateBooking, setDetailOpen, setSelectedBooking, onActionSuccess);
  const handleAttended = useAttendedHandler(markAttended, setDetailOpen, setSelectedBooking, onActionSuccess);
  const handleConfirmManualRefund = useManualRefundHandler(
    markManualRefund,
    selectedBooking,
    setManualRefundOpen,
    setDetailOpen,
    setSelectedBooking,
    onActionSuccess,
  );

  return {
    cancelBooking,
    confirmPayment,
    markManualRefund,
    handleConfirmCancel,
    handlePaymentSubmit,
    handleNoShow,
    handleAttended,
    handleConfirmManualRefund,
  };
}
