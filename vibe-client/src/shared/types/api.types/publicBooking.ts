import type { Amenity, Schedule } from './complex';
import type { CourtWithPrices, DurationMinutes, Sport, CourtType } from './court';
import type { Booking, BookingRefundStatus, BookingStatus, CollectionStatus } from './booking';

// ─── Public Booking ───

/**
 * What `GET /api/v1/public/complexes/:slug` actually sends — an explicit
 * projection of `Complex` (see `internal/complexes/handlers.go`'s
 * `publicComplex`), not the owner-facing shape. It deliberately excludes
 * `owner_id`, `mp_user_id`, `is_active`, `created_at`, and `updated_at`;
 * `mp_user_id` (the MercadoPago collector id the payment path checks
 * incoming payments against) is replaced by `payments_enabled`, the only
 * thing a client needed from it. Do not widen this back to `Complex` —
 * that reuse is what let a dropped field silently compile as `undefined`.
 */
export interface PublicComplex {
  id: string;
  name: string;
  slug: string;
  address: string;
  city: string;
  province: string;
  country_code: string;
  currency: string;
  phone: string;
  email?: string | null;
  logo_url?: string | null;
  cover_url?: string | null;
  deposit_percentage: number;
  cancellation_hours: number;
  latitude?: number | null;
  longitude?: number | null;
  amenities: Amenity[];
  payments_enabled: boolean;
}

export interface PublicComplexResponse {
  complex: PublicComplex;
  courts: CourtWithPrices[];
  schedules: Schedule[];
}

export interface PublicBookingRequest {
  complex_id: string;
  court_id: string;
  date: string;
  start_time: string;
  duration_minutes: DurationMinutes;
  client_first_name: string;
  client_last_name: string;
  client_phone: string;
  client_email: string;
  client_notes?: string;
}

/**
 * What `booking` holds in `POST /api/v1/book`'s response
 * (`internal/bookings/public.go`'s `PublicBook`) — an explicit projection,
 * not `Booking`. It deliberately excludes the booking's primary key and
 * every other internal field (`id`, `complex_id`, `court_id`, `client_id`,
 * `duration_minutes`, `notes`, `created_by`, `created_at`, `updated_at`):
 * only what a confirmation screen needs to render.
 */
export interface PublicBookingEnvelope {
  status: BookingStatus;
  collection_status: CollectionStatus;
  refund_status: BookingRefundStatus;
  date: string;
  start_time: string;
  /** RFC3339, Argentina offset — the booking's span. */
  starts_at: string;
  ends_at: string;
  court_name: string;
  complex_name: string;
  price: number;
  deposit_amount: number;
}

export interface PublicBookingResponse {
  booking: PublicBookingEnvelope;
  /**
   * The opaque, expiring, hashed credential that now authorizes the three
   * public routes (`GET /book/status`, `GET /book/cancel-info`,
   * `POST /book/cancel`) in place of the booking's id
   * (specs/booking-link-credential on the server). Carry this forward
   * everywhere the old `booking.id` used to go — never the id itself, which
   * the server no longer sends.
   */
  token: string;
  mp_init_point?: string;
  mp_preference_id?: string;
  service_fee?: number;
  total_client_pays?: number;
}

export interface MPConnectRequest {
  code: string;
  redirect_uri: string;
}

export interface MPConnectResponse {
  connected: boolean;
  mp_user_id?: string;
}

export interface MPStatusResponse {
  connected: boolean;
  mp_user_id?: string;
  /**
   * The MercadoPago application id the API can actually exchange an OAuth
   * code with (`internal/complexes/handlers.go`'s `MercadoPagoStatus`,
   * omitted when the server has none configured). The client must prefer
   * this over its own `VITE_MP_APP_ID` build-time value — a client built
   * against a different app id would send the seller through an
   * authorization the API cannot complete.
   */
  app_id?: string;
}

/**
 * The cancellation window as `GET /book/status` sees it right now — distinct
 * from `CancelInfoResponse`'s own shape, which is fetched separately by the
 * cancel page and answers `can_refund`/`refund_method` instead. This one
 * only tells the success page whether to still offer cancelling at all
 * (`can_cancel`) and, while it can, whether that would carry a refund.
 */
export interface BookingStatusCancellation {
  can_cancel: boolean;
  /** RFC3339, Argentina offset. Null when there is no fixed deadline (e.g. cancellable with refund right up to the turn itself). */
  refund_deadline: string | null;
  can_refund_now: boolean;
  cancellation_hours: number;
}

/**
 * What `booking` holds in `GET /book/status`'s response
 * (`internal/bookings/public.go`'s `BookingStatus`). Only `status`,
 * `collection_status` and `refund_status` are guaranteed — every other field
 * was added after the
 * success page first shipped, so an older server that hasn't been restarted
 * yet can still answer with just those two. Callers must fall back (to a
 * locally cached `BookingInfo`, or to hiding the affected block) rather than
 * assume the rest of the shape is there.
 */
export interface BookingStatusDetails {
  status: BookingStatus;
  collection_status: CollectionStatus;
  refund_status: BookingRefundStatus;
  complex_name?: string;
  complex_address?: string;
  complex_phone?: string | null;
  court_name?: string;
  sport?: Sport;
  court_type?: CourtType;
  /** YYYY-MM-DD */
  date?: string;
  /** HH:MM */
  start_time?: string;
  /** RFC3339, Argentina offset. */
  starts_at?: string;
  /**
   * RFC3339, Argentina offset. The only field that can say a booking ends on
   * the following day, which is why the success page's merge gates on it.
   */
  ends_at?: string;
  duration_minutes?: DurationMinutes;
  /** Centavos. */
  price?: number;
  /** Centavos. */
  deposit_amount?: number;
  /** Centavos. */
  service_fee?: number;
  /** Centavos. */
  remaining_amount?: number;
  cancellation?: BookingStatusCancellation;
}

export interface BookingStatusResponse {
  booking: BookingStatusDetails;
}

/**
 * How the deposit would come back, from `cancel-info` (`internal/bookings/cancel.go`'s
 * `refundByMercadoPago` / `refundByHand` / `refundNotApplicable`):
 * - `"mercadopago"` — returns automatically to the card or account it was paid from.
 * - `"manual"` — money was paid and only a person at the venue can return it
 *   (cash, a transfer, or a MercadoPago payment carrying no payment id).
 * - `"none"` — nothing was paid, or it is already back.
 *
 * This is independent of `can_refund`: it answers *how* the money would move,
 * `can_refund` answers *whether* it will (the cancellation window AND this not
 * being `"none"`).
 */
export type RefundMethod = 'mercadopago' | 'manual' | 'none';

export interface CancelInfoResponse {
  booking: {
    status: string;
    date: string;
    start_time: string;
    court_name: string;
    complex_name: string;
    /** Below: added by the in-flight `cancel-info` extension — absent on a server that predates it. */
    sport?: Sport;
    court_type?: CourtType;
    /** RFC3339, Argentina offset. */
    starts_at?: string;
    /** RFC3339, Argentina offset. */
    ends_at?: string;
    duration_minutes?: DurationMinutes;
    complex_address?: string;
  };
  can_cancel: boolean;
  can_refund: boolean;
  refund_method: RefundMethod;
  cancellation_hours: number;
  /** Centavos. Includes the service fee. Added by the in-flight `cancel-info` extension. */
  refund_amount?: number;
  /** Centavos. Added by the in-flight `cancel-info` extension. */
  paid_amount?: number;
}

export interface PublicCancelBookingRequest {
  token: string;
}

/**
 * `refund.status` mirrors the server's own refund state machine
 * (`data.RefundResult` in `internal/data/refunds.go`) — `queued` vs `issued`
 * vs `already_issued` are distinctions the client cannot derive on its own.
 * Use this only to choose presentation (icon, tone, whether to highlight the
 * amount); render `refund.message` for the actual copy, never a client-side
 * re-derivation of it.
 */
export type RefundStatus = 'none' | 'not_eligible' | 'issued' | 'already_issued' | 'queued' | 'manual';

export interface RefundEnvelope {
  status: RefundStatus;
  /** Spanish, server-authored sentence for this exact outcome (`internal/bookings/cancel.go`'s `refundMessages`). */
  message: string;
  /** Centavos. Omitted by the server when zero. */
  amount?: number;
  /** Centavos still owed to the client by hand (cash/transfer portion). Present only on a split refund. */
  manual_amount?: number;
  /** Server-authored sentence describing the manual portion. Present only on a split refund. */
  manual_message?: string;
}

export interface PublicCancelBookingResponse {
  booking: Pick<Booking, 'status' | 'collection_status' | 'refund_status'>;
  /** True only when money actually moved (`outcome.MoneyReturned()`), not merely requested. */
  refunded: boolean;
  refund: RefundEnvelope;
}
