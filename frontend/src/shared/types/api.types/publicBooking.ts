import type { Amenity } from './complex';
import type { CourtWithPrices, DurationMinutes } from './court';
import type { Body, Ok, Spec } from './spec';

// ─── Public Booking ───

/**
 * What `GET /api/v1/public/complexes/:slug` sends — the storefront
 * projection of `Complex`, not the owner-facing shape. It deliberately
 * excludes `owner_id`, `mp_user_id`, `is_active`, `created_at` and
 * `updated_at`; `mp_user_id` (the MercadoPago collector id the payment path
 * checks incoming payments against) is replaced by `payments_enabled`, the
 * only thing a client needed from it. Do not widen this back to `Complex` —
 * that reuse is what let a dropped field silently compile as `undefined`.
 *
 * `amenities` is narrowed to {@link Amenity} for the same reason it is on
 * `Complex`: the document types the response field as `string[]`.
 */
export type PublicComplex = Omit<Spec<'PublicComplex'>, 'amenities'> & { amenities: Amenity[] };

export type PublicComplexResponse = Omit<Ok<'complexesGetPublic'>, 'complex' | 'courts'> & {
  complex: PublicComplex;
  courts: CourtWithPrices[];
};

/** `duration_minutes` narrowed to the durations the slot picker offers — see {@link DurationMinutes}. */
export type PublicBookingRequest = Omit<Body<'bookingsPublicCreate'>, 'duration_minutes'> & {
  duration_minutes: DurationMinutes;
};

/**
 * What `booking` holds in `POST /api/v1/book`'s response — an explicit
 * projection, not `Booking`. It deliberately excludes the booking's primary
 * key and every other internal field (`id`, `complex_id`, `court_id`,
 * `client_id`, `duration_minutes`, `notes`, `created_by`, `created_at`,
 * `updated_at`): only what a confirmation screen needs to render.
 */
export type PublicBookingEnvelope = Spec<'PublicBookingResult'>['booking'];

/**
 * `token` is the opaque, expiring, hashed credential that authorizes the
 * three public routes (`GET /book/status`, `GET /book/cancel-info`,
 * `POST /book/cancel`) in place of the booking's id. Carry it forward
 * everywhere the old `booking.id` used to go — never the id itself, which
 * the server no longer sends.
 */
export type PublicBookingResponse = Ok<'bookingsPublicCreate'>;

export type MPConnectRequest = Body<'complexesConnectMercadoPago'>;

export type MPConnectResponse = Ok<'complexesConnectMercadoPago'>;

/**
 * `app_id` is the MercadoPago application id the API can actually exchange
 * an OAuth code with (omitted when the server has none configured). The
 * client must prefer it over its own `VITE_MP_APP_ID` build-time value — a
 * client built against a different app id would send the seller through an
 * authorization the API cannot complete.
 */
export type MPStatusResponse = Ok<'complexesMercadoPagoStatus'>;

/**
 * The cancellation window as `GET /book/status` sees it right now — distinct
 * from {@link CancelInfoResponse}'s own shape, which is fetched separately by
 * the cancel page and answers `can_refund`/`refund_method` instead. This one
 * only tells the success page whether to still offer cancelling at all
 * (`can_cancel`) and, while it can, whether that would carry a refund.
 */
export type BookingStatusCancellation = Spec<'PublicBookingStatus'>['cancellation'];

/**
 * What `booking` holds in `GET /book/status`'s response.
 *
 * Only `status`, `collection_status` and `refund_status` are treated as
 * guaranteed. `openapi.yaml` marks the rest required, and a current server
 * does send them — but they were added after the success page shipped, and a
 * server that has not been restarted yet answers with just the three. The
 * page falls back (to a locally cached booking, or to hiding the affected
 * block) rather than assume the rest of the shape is there, and
 * `bookingStatusDetailsSchema` parses them as optional to match.
 */
export type BookingStatusDetails = Pick<
  Spec<'PublicBookingStatus'>,
  'status' | 'collection_status' | 'refund_status'
> &
  Partial<
    Omit<Spec<'PublicBookingStatus'>, 'status' | 'collection_status' | 'refund_status' | 'duration_minutes'>
  > & { duration_minutes?: DurationMinutes };

export type BookingStatusResponse = Omit<Ok<'bookingsPublicStatus'>, 'booking'> & { booking: BookingStatusDetails };

/**
 * How the deposit would come back, from `cancel-info`:
 * - `"mercadopago"` — returns automatically to the card or account it was paid from.
 * - `"manual"` — money was paid and only a person at the venue can return it
 *   (cash, a transfer, or a MercadoPago payment carrying no payment id).
 * - `"none"` — nothing was paid, or it is already back.
 *
 * This is independent of `can_refund`: it answers *how* the money would move,
 * `can_refund` answers *whether* it will (the cancellation window AND this not
 * being `"none"`).
 */
export type RefundMethod = Spec<'PublicCancelInfo'>['refund_method'];

/**
 * Same treatment as {@link BookingStatusDetails}: the fields the in-flight
 * `cancel-info` extension added are read as optional here even though the
 * document now marks them required, so a server that predates it degrades
 * instead of failing its response schema.
 */
type CancelInfoBooking = Spec<'PublicCancelInfo'>['booking'];

export type CancelInfoResponse = Omit<Spec<'PublicCancelInfo'>, 'booking' | 'refund_amount' | 'paid_amount'> & {
  booking: Pick<CancelInfoBooking, 'status' | 'date' | 'start_time' | 'court_name' | 'complex_name'> &
    Partial<
      Omit<CancelInfoBooking, 'status' | 'date' | 'start_time' | 'court_name' | 'complex_name' | 'duration_minutes'>
    > & { duration_minutes?: DurationMinutes };
  /** Centavos. Includes the service fee. */
  refund_amount?: number;
  /** Centavos. */
  paid_amount?: number;
};

export type PublicCancelBookingRequest = Body<'bookingsPublicCancel'>;

/**
 * `status` mirrors the server's own refund state machine — `queued` vs
 * `issued` vs `already_issued` are distinctions the client cannot derive on
 * its own. Use it only to choose presentation (icon, tone, whether to
 * highlight the amount); render `message` for the actual copy, never a
 * client-side re-derivation of it.
 */
export type RefundStatus = Spec<'RefundOutcome'>['status'];

export type RefundEnvelope = Spec<'RefundOutcome'>;

export type PublicCancelBookingResponse = Ok<'bookingsPublicCancel'>;
