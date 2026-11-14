import { useMemo } from 'react';
import { useSelectedComplex, useSchedules } from '@/features/complex';
import { useCourts, useBlockedSlots, useFixedBlocks, useEventBlocks, useMatchBlocks } from '@/features/courts';
import { useBookings, useBookingModals, useBookingActions } from '@/features/bookings';
import { useDateNav } from './use-bookings-page/useDateNav';
import type { CourtEventBlock } from '@/shared/types/api.types';

// Takes a resolved `complexId` rather than resolving it itself — the page
// only ever mounts this hook once `useSelectedComplex` has a real id (see
// `BookingsPage`'s guard), so nothing here needs a `?? ''` fallback for a
// "no complex yet" state that can't actually occur at this point.
export function useBookingsPage(complexId: string) {
  const { complex } = useSelectedComplex();
  const { data: courts = [] } = useCourts(complexId);
  const { data: schedules = [] } = useSchedules(complexId, complex?.slug);

  const dateNav = useDateNav();
  const { selectedDate } = dateNav;

  const { data: bookings = [], isLoading, isError, refetch } = useBookings(complexId, selectedDate);
  const { data: blockedSlots = [] } = useBlockedSlots(complexId, selectedDate, selectedDate);
  const { data: fixedBlocks = [] } = useFixedBlocks(complexId, selectedDate, selectedDate);
  const { data: openPlayBlocks = [] } = useEventBlocks(complexId, selectedDate, selectedDate);
  const { data: matchBlocks = [] } = useMatchBlocks(complexId, selectedDate, selectedDate);
  // Events and tournament matches are drawn the same way; one stable list keeps the grid's memoised columns.
  const eventBlocks = useMemo<CourtEventBlock[]>(
    () => [...openPlayBlocks, ...matchBlocks],
    [openPlayBlocks, matchBlocks],
  );

  const modals = useBookingModals(selectedDate, complexId);
  const actions = useBookingActions({
    selectedComplexId: complexId,
    selectedDate,
    selectedBooking: modals.selectedBooking,
    setSelectedBooking: modals.setSelectedBooking,
    setDetailOpen: modals.setDetailOpen,
    setCancelOpen: modals.setCancelOpen,
    setPaymentOpen: modals.setPaymentOpen,
    setManualRefundOpen: modals.setManualRefundOpen,
  });

  return {
    // Complex
    complex,
    selectedComplexId: complexId,
    courts,
    schedules,

    // Date
    ...dateNav,

    // Data
    bookings,
    blockedSlots,
    fixedBlocks,
    eventBlocks,
    isLoading,
    isError,
    refetch,

    // Booking selection + modals
    ...modals,

    // Actions
    ...actions,
  };
}
