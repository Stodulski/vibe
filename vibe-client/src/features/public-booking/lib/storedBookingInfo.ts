import { bookingInfoSchema, type BookingInfo } from '../components/booking-confirmed/types';
import { safeSessionStorage } from '@/shared/lib/safeStorage';

/**
 * Where the booking being paid for is parked while the browser is away at
 * MercadoPago.
 *
 * The confirm page writes it immediately before handing over to the checkout,
 * because the redirect leaves the app entirely and React state does not
 * survive that. Both ways back read it: the success page to show what was
 * booked, and the failure landing to show what was *not*.
 *
 * The key and the reader used to be declared twice, once on each side of that
 * round trip, which is one rename away from the two halves disagreeing in
 * silence. They live here once.
 */
export const BOOKING_INFO_KEY = 'vibe_booking_info';

/**
 * Reads back what this app itself stored. Returns null for anything it cannot
 * use — no entry, unparseable, or shaped wrong — so every caller has one
 * empty case to design for rather than three failure modes.
 *
 * sessionStorage survives a refresh and is editable by hand, and this exact
 * key/shape can also change between deploys, so `bookingInfoSchema.safeParse`
 * is the real guard here rather than the `typeof === 'object'` check this
 * used to stop at. `safeSessionStorage.getJSON` also covers storage being
 * unreadable outright (private browsing, disabled storage).
 */
export function readStoredBookingInfo(): BookingInfo | null {
  return safeSessionStorage.getJSON(BOOKING_INFO_KEY, bookingInfoSchema);
}
