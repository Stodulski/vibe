import { Outlet } from 'react-router-dom';
import { BookingRoot } from '@/features/public-booking/BookingRoot';
import { getPublicBookingConfig } from './publicBookingRuntime';

/**
 * The booking providers around the booking pages. The app's `Providers` already
 * mounts the Toaster, so it is switched off here to avoid rendering every toast twice.
 */
export function PublicBookingRoot() {
  return (
    <BookingRoot config={getPublicBookingConfig()} toaster={false}>
      <Outlet />
    </BookingRoot>
  );
}
