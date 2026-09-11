// Public API of the bookings feature.
// Only export what other features or pages actually consume from outside
// this folder — see 06-auth-shared-tooling.md A3 and 02-bookings-clients.md M4.

export { BookingDetailModals } from './components/BookingDetailModals';
export { BookingCalendar } from './components/BookingCalendar';
export { BookingDetail } from './components/BookingDetail';
export { CreateBookingModal } from './components/CreateBookingModal';
export { CancelBookingModal } from './components/CancelBookingModal';
export { ConfirmPaymentModal } from './components/ConfirmPaymentModal';
export { ManualRefundDialog } from './components/ManualRefundDialog';
export type { CreateBookingPrefill } from './components/create-booking-modal/useBookingReset';

export { useBookings } from './hooks/useBookings';
export { useBookingModals } from './hooks/useBookingModals';
export { useBookingActions } from './hooks/useBookingActions';

export type { ConfirmPaymentDto } from './schemas/booking.schemas';
