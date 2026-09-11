import { useCallback, useMemo, useState } from 'react';
import type { Booking } from '@/shared/types/api.types';
import type { CreateBookingPrefill } from '@/features/bookings/components/create-booking-modal/useBookingReset';
import { useBooking } from './useBooking';

function useCancelBookingInfo(selectedBooking: Booking | null) {
  return useMemo(() => {
    if (!selectedBooking) return undefined;
    return {
      court_name: selectedBooking.court_name,
      date: selectedBooking.date,
      starts_at: selectedBooking.starts_at,
      ends_at: selectedBooking.ends_at,
      client_name: selectedBooking.client_name,
    };
  }, [selectedBooking]);
}

/**
 * The selected booking's id is the real state — the object itself is
 * resolved from the same query cache `BookingDetail`'s own `useBooking` fills
 * (see 03-owner-pages-dashboard.md M8: "guardá el id, no el objeto"). `seed`
 * is only the render before that query has data (or before a `complexId` is
 * known at all, for a caller that hasn't been updated to pass one yet — see
 * this hook's own "Needs another owner" note) — it is never written back to
 * once the id is set, so it can't go stale the way holding the whole object
 * in state could.
 */
function useSelectedBooking(complexId: string | null) {
  const [selectedBookingId, setSelectedBookingId] = useState<string | null>(null);
  const [seed, setSeed] = useState<Booking | null>(null);

  const { data: detail } = useBooking(complexId, selectedBookingId);
  const selectedBooking = detail?.booking ?? seed;

  const select = useCallback((booking: Booking | null) => {
    setSelectedBookingId(booking?.id ?? null);
    setSeed(booking);
  }, []);

  return { selectedBooking, selectedBookingId, setSelectedBooking: select };
}

function useManualRefundHandlers(
  setSelectedBooking: (booking: Booking) => void,
  setDetailOpen: (open: boolean) => void,
) {
  const [manualRefundOpen, setManualRefundOpen] = useState(false);
  const [manualRefundAmount, setManualRefundAmount] = useState(0);

  const handleManualRefundOpen = useCallback(
    (booking: Booking, amount: number) => {
      setSelectedBooking(booking);
      setDetailOpen(false);
      setManualRefundAmount(amount);
      setManualRefundOpen(true);
    },
    [setSelectedBooking, setDetailOpen],
  );

  return { manualRefundOpen, setManualRefundOpen, manualRefundAmount, handleManualRefundOpen };
}

/** Closes the detail sheet and opens a different one on the same booking — the shared shape of "cancel" and "confirm payment". */
function useCloseDetailAndOpen(
  setSelectedBooking: (booking: Booking) => void,
  setDetailOpen: (open: boolean) => void,
  setTargetOpen: (open: boolean) => void,
) {
  return useCallback(
    (booking: Booking) => {
      setSelectedBooking(booking);
      setDetailOpen(false);
      setTargetOpen(true);
    },
    [setSelectedBooking, setDetailOpen, setTargetOpen],
  );
}

/**
 * `complexId` is optional (and defaults to `null`) so existing callers that
 * only pass `selectedDate` keep working — without it, `selectedBooking` falls
 * back to whatever was last selected (the pre-fix behavior) instead of being
 * kept fresh from the query cache. Pass it to get the cache-backed value.
 */
export function useBookingModals(selectedDate: string, complexId: string | null = null) {
  const { selectedBooking, selectedBookingId, setSelectedBooking } = useSelectedBooking(complexId);
  const [detailOpen, setDetailOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [paymentOpen, setPaymentOpen] = useState(false);
  const [createPrefill, setCreatePrefill] = useState<CreateBookingPrefill>();

  const cancelBookingInfo = useCancelBookingInfo(selectedBooking);
  const manualRefund = useManualRefundHandlers(setSelectedBooking, setDetailOpen);

  const handleSelectBooking = useCallback(
    (booking: Booking) => {
      setSelectedBooking(booking);
      setDetailOpen(true);
    },
    [setSelectedBooking],
  );

  const handleCreateFromSlot = useCallback((prefill: CreateBookingPrefill) => {
    setCreatePrefill(prefill);
    setCreateOpen(true);
  }, []);

  const handleCancelBooking = useCloseDetailAndOpen(setSelectedBooking, setDetailOpen, setCancelOpen);
  const handleConfirmPaymentOpen = useCloseDetailAndOpen(setSelectedBooking, setDetailOpen, setPaymentOpen);

  const handleOpenCreate = useCallback(() => {
    setCreatePrefill({ date: selectedDate });
    setCreateOpen(true);
  }, [selectedDate]);

  return {
    selectedBooking,
    selectedBookingId,
    setSelectedBooking,
    detailOpen,
    setDetailOpen,
    createOpen,
    setCreateOpen,
    cancelOpen,
    setCancelOpen,
    paymentOpen,
    setPaymentOpen,
    createPrefill,
    cancelBookingInfo,
    handleSelectBooking,
    handleCreateFromSlot,
    handleCancelBooking,
    handleConfirmPaymentOpen,
    handleOpenCreate,
    ...manualRefund,
  };
}
