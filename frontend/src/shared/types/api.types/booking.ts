import type { DurationMinutes } from './court';
import type { Payment } from './payment';
import type { Body, Ok, Spec } from './spec';

// ─── Booking ───

/**
 * `court_name`, `client_name` and `client_phone` are narrowed back to
 * required. `openapi.yaml` calls them "Enrichment, present when joined" and
 * leaves them out of `required`, but every booking endpoint this client calls
 * joins them, and 60-odd render sites read them as plain strings — a name
 * rendered as `undefined` would be a worse answer than a failed parse, which
 * is what `bookingSchema` already gives (it requires all three).
 *
 * Read `starts_at`/`ends_at`, not `date`/`start_time`: a booking may run past
 * midnight, and a time of day no longer says which day it belongs to once it
 * does — an end of `"01:00"` sorts below a start of `"23:00"` under exactly
 * the comparison the occupancy checks were built on, so the booking reads as
 * occupying no time at all. Render them with `formatInstantTime` /
 * `formatHourRange`, never with `formatTime`, which slices the first five
 * characters off a string and returns `"2026-"` when handed an instant.
 */
export type Booking = Spec<'Booking'> & Required<Pick<Spec<'Booking'>, 'court_name' | 'client_name' | 'client_phone'>>;

export type BookingStatus = Spec<'BookingStatus'>;

/** How much of the booking's price the venue has collected. */
export type CollectionStatus = Spec<'CollectionStatus'>;

/**
 * Where the give-back stands. `partial` is the split state: the
 * MercadoPago-backed rows came back automatically and a cash or transfer
 * balance is still owed by hand, which only the manual-refund endpoint closes.
 *
 * The name carries the `Booking` prefix because `RefundStatus` in
 * `publicBooking.ts` is already taken by a different question — what one
 * cancellation *did* about the money (`issued`, `queued`, `manual`, …) — and
 * both are re-exported flat from `api.types`.
 */
export type BookingRefundStatus = Spec<'RefundStatus'>;

/**
 * The single value a compact badge shows for the pair above. A client-side
 * projection, not a wire field: it is the vocabulary the old `payment_status`
 * field had, and every label, colour and icon keyed on it is unchanged — a
 * badge has room for one word, and a refund under way is the more urgent of
 * the two facts. Anywhere with room for both, read `collection_status` and
 * `refund_status` directly: this projection is lossy on purpose and is the
 * exact loss the payment_status split removed from the database.
 */
export type PaymentDisplayStatus = CollectionStatus | 'outstanding' | 'refund_pending' | 'partial_refund' | 'refunded';

/** The fields that say whether a booking's hours are over and whether anyone marked attendance. */
interface BookingProgress {
  status: BookingStatus;
  collection_status: CollectionStatus;
  refund_status: BookingRefundStatus;
  ends_at: string;
  attended_at?: string | null | undefined;
}

function bookingHasEnded(booking: { ends_at: string }, now: number = Date.now()): boolean {
  return Date.parse(booking.ends_at) <= now;
}

// The booking status as the owner reads it. Once its hours are over a booking
// reads "Turno finalizado" until the owner marks whether the client came:
// "Se presentó" (completed with attended_at) or "No se presentó" (no_show).
// The completion sweep runs every 30 minutes, so a confirmed booking whose
// hours just ended is already finished too.
export type BookingDisplayStatus = BookingStatus | 'finished';

export function bookingDisplayStatus(
  booking: Pick<BookingProgress, 'status' | 'ends_at' | 'attended_at'>,
  now: number = Date.now(),
): BookingDisplayStatus {
  if (booking.status === 'completed' && !booking.attended_at) return 'finished';
  if (booking.status === 'confirmed' && bookingHasEnded(booking, now)) return 'finished';
  return booking.status;
}

// The colour a booking takes on the court grid. A booking played and not paid
// in full keeps the amber of money still to collect; the words stay apart: the
// booking status says how the turn went, the payment status says "Pago incompleto".
export type BookingBarStatus = BookingDisplayStatus | 'outstanding';

export function bookingBarStatus(booking: BookingProgress, now: number = Date.now()): BookingBarStatus {
  return paymentDisplayStatus(booking, now) === 'outstanding' ? 'outstanding' : bookingDisplayStatus(booking, now);
}

export function paymentDisplayStatus(
  payment: Pick<BookingProgress, 'status' | 'collection_status' | 'refund_status' | 'ends_at'>,
  now: number = Date.now(),
): PaymentDisplayStatus {
  switch (payment.refund_status) {
    case 'pending':
      return 'refund_pending';
    case 'partial':
      return 'partial_refund';
    case 'full':
      return 'refunded';
    default: {
      // A booking whose hours are over and that is not paid in full: the
      // money is now due, not just partly collected. A no-show is settled by
      // its own rule (the deposit is kept), so it is not counted here.
      const over = payment.status === 'completed' || (payment.status === 'confirmed' && bookingHasEnded(payment, now));
      if (over && payment.collection_status !== 'fully_paid') return 'outstanding';
      return payment.collection_status;
    }
  }
}

/**
 * `duration_minutes` is narrowed to {@link DurationMinutes}: the document
 * types it as a bare integer and the server rejects anything that is not a
 * permitted slot duration, so sending one is a round trip to a 422.
 */
export type CreateBookingRequest = Omit<Body<'bookingsCreate'>, 'duration_minutes'> & {
  duration_minutes: DurationMinutes;
};

export type CancelBookingRequest = Body<'bookingsCancel'>;

/**
 * Same `refund` shape the public cancel flow gets (`RefundEnvelope`) — the
 * owner cancel answers with an identical `{"booking", "refund"}` envelope.
 */
export type CancelBookingResponse = Omit<Ok<'bookingsCancel'>, 'booking'> & { booking: Booking };

export type ConfirmPaymentRequest = Body<'bookingsConfirmPayment'>;

export type ConfirmPaymentResponse = Omit<Ok<'bookingsConfirmPayment'>, 'booking' | 'payment'> & {
  booking: Booking;
  payment: Payment;
};

export type BookingsListResponse = Omit<Ok<'bookingsList'>, 'bookings'> & { bookings: Booking[] };

export type BookingDetailResponse = Omit<Ok<'bookingsGet'>, 'booking' | 'payment' | 'payments'> & {
  booking: Booking;
  payment?: Payment;
  payments: Payment[];
};

export type ManualRefundResponse = Omit<Ok<'bookingsManualRefund'>, 'booking' | 'payments'> & {
  booking: Booking;
  payments: Payment[];
};

// ─── Booking series (turnos fijos) ───

export type BookingSeries = Spec<'BookingSeries'>;

export type BookingSeriesConflict = Spec<'BookingSeriesConflict'>;

/** A court free on a conflict date of the refused series, as the create-series 409 lists it. */
export type BookingSeriesFreeCourt = Spec<'BookingSeriesFreeCourt'>;

/** Same `duration_minutes` narrowing as {@link CreateBookingRequest}. */
export type CreateBookingSeriesRequest = Omit<Body<'bookingSeriesCreate'>, 'duration_minutes'> & {
  duration_minutes: DurationMinutes;
};

export type CreateBookingSeriesResponse = Omit<Ok<'bookingSeriesCreate'>, 'bookings'> & { bookings: Booking[] };

export type BookingSeriesUpcoming = Spec<'BookingSeriesUpcoming'>;

export type BookingSeriesDetailResponse = Ok<'bookingSeriesGet'>;

export type SkipSeriesDateResponse = Ok<'bookingSeriesSkip'>;

export type EndBookingSeriesResponse = Omit<Ok<'bookingSeriesEnd'>, 'cancelled' | 'kept'> & {
  cancelled: Booking[];
  kept: Booking[];
};
