import { useSelectedComplex, useSchedules } from '@/features/complex';
import { useCourts, useBlockedSlots } from '@/features/courts';
import { useBookings, useBookingModals, useBookingActions } from '@/features/bookings';
import { useDateNav } from './use-bookings-page/useDateNav';

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
    isLoading,
    isError,
    refetch,

    // Booking selection + modals
    ...modals,

    // Actions
    ...actions,
  };
}
