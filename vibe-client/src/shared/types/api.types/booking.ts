import type { Client } from './client';
import type { DurationMinutes } from './court';
import type { Payment } from './payment';
import type { RefundEnvelope } from './publicBooking';

// ─── Booking ───

export interface Booking {
  id: string;
  complex_id: string;
  court_id: string;
  client_id: string;
  /**
   * The booking's span as two absolute instants (ISO 8601). Read these, not the
   * three fields below.
   *
   * A booking may run past midnight, and at that point a time of day no longer
   * says which day it belongs to: an end of `"01:00"` sorts below a start of
   * `"23:00"` under exactly the comparison the occupancy checks were built on,
   * so the booking reads as occupying no time at all. The server derives these
   * from the same `bookings.span` its exclusion constraint enforces, so what
   * the grid draws and what a write is refused for come from one value.
   *
   * There is no `end_time` beside them. The column it came from is gone
   * (vibe-server dropped the column): it stored what the clock would read, wrapped
   * at midnight, so it could not answer the one question an end has to.
   * `date` and `start_time` stay until every reader has moved off them.
   *
   * Render these with `formatInstantTime` / `formatHourRange`, never with
   * `formatTime` — that one slices the first five characters off a string and
   * returns `"2026-"` when handed an instant.
   */
  starts_at: string;
  ends_at: string;
  date: string;
  start_time: string;
  duration_minutes: number;
  price: number;
  deposit_amount: number;
  status: BookingStatus;
  /**
   * How much of the price has been collected, and where the give-back stands.
   *
   * These are two independent facts and the server keeps them in two columns
   * (the payment_status split). They used to be one `payment_status` field, which meant
   * the collection fact was destroyed the moment a refund started: a booking
   * that took only a deposit and is being refunded read `refund_pending`, and
   * so did one that had been paid in full. Use {@link paymentDisplayStatus}
   * where a single badge is wanted.
   */
  collection_status: CollectionStatus;
  refund_status: BookingRefundStatus;
  notes?: string | null;
  court_name: string;
  client_name: string;
  client_phone: string;
  reminder_sent_2h: boolean;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export type BookingStatus = 'pending' | 'confirmed' | 'cancelled' | 'completed' | 'no_show';

/** How much of the booking's price the venue has collected. */
export type CollectionStatus = 'unpaid' | 'deposit_paid' | 'fully_paid';

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
export type BookingRefundStatus = 'none' | 'pending' | 'partial' | 'full';

/**
 * The single value a compact badge shows for the pair above.
 *
 * It is the vocabulary the old `payment_status` field had, and every label,
 * colour and icon keyed on it is unchanged — a badge has room for one word, and
 * a refund under way is the more urgent of the two facts. Anywhere with room
 * for both, read `collection_status` and `refund_status` directly: this
 * projection is lossy on purpose and is the exact loss the payment_status split removed
 * from the database.
 */
export type PaymentDisplayStatus = CollectionStatus | 'refund_pending' | 'partial_refund' | 'refunded';

export function paymentDisplayStatus(payment: {
  collection_status: CollectionStatus;
  refund_status: BookingRefundStatus;
}): PaymentDisplayStatus {
  switch (payment.refund_status) {
    case 'pending':
      return 'refund_pending';
    case 'partial':
      return 'partial_refund';
    case 'full':
      return 'refunded';
    default:
      return payment.collection_status;
  }
}

export interface CreateBookingRequest {
  court_id: string;
  date: string;
  start_time: string;
  client_phone: string;
  client_first_name: string;
  client_last_name: string;
  client_email?: string;
  duration_minutes: DurationMinutes;
  payment_option?: 'unpaid' | 'deposit' | 'full';
  deposit_amount?: number;
  payment_method?: 'cash' | 'transfer';
  notes?: string;
  /**
   * Centavos. Owner-path only: overrides the server-computed price. Required
   * when no price rule covers the booking's span (the server then rejects a
   * missing `price` with a field-level validation error).
   */
  price?: number;
}

export interface CancelBookingRequest {
  reason?: string;
}

/**
 * Same `refund` shape the public cancel flow gets (`RefundEnvelope`) —
 * `internal/bookings/actions.go` answers the owner cancel with an identical
 * `{"booking", "refund"}` envelope, so this reuses the type rather than
 * redeclaring it.
 */
export interface CancelBookingResponse {
  booking: Booking;
  refund: RefundEnvelope;
}

export interface ConfirmPaymentRequest {
  method: 'cash' | 'transfer';
  amount: number;
}

export interface BookingsListResponse {
  bookings: Booking[];
  metadata: {
    next_cursor?: string;
    has_more: boolean;
    total_count?: number;
  };
}

export interface BookingDetailResponse {
  booking: Booking;
  client?: Client;
  payment?: Payment;
  payments: Payment[];
}

export interface ManualRefundResponse {
  booking: Booking;
  payments: Payment[];
  /** Centavos. */
  returned_amount: number;
}

export interface ConfirmPaymentResponse {
  booking: Booking;
  payment: Payment;
}
