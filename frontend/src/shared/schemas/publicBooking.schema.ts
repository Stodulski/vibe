import { z } from 'zod';
import type {
  PublicComplex,
  PublicComplexResponse,
  PublicBookingEnvelope,
  PublicBookingResponse,
  MPConnectResponse,
  MPStatusResponse,
  BookingStatusCancellation,
  BookingStatusDetails,
  BookingStatusResponse,
  RefundMethod,
  CancelInfoResponse,
  RefundStatus,
  RefundEnvelope,
  PublicCancelBookingResponse,
  BookingStatus,
  CollectionStatus,
  BookingRefundStatus,
} from '@/shared/types/api.types';
import { amenitySchema, scheduleSchema } from './complex.schema';
import { courtWithPricesSchema, durationMinutesSchema, sportSchema, courtTypeSchema } from './court.schema';
import { exact } from '@/shared/lib/apiParse';

/**
 * Reads a field that may arrive as `null`, as the key with a value, or not at
 * all, and yields `undefined` for the first and last. Zod's `.optional()`
 * alone rejects an explicit `null`, which is how a cleared column serializes
 * on one projection of a row even where the document only shows the other.
 */
function nullableToAbsent<T extends z.ZodType>(inner: T) {
  return inner
    .nullable()
    .optional()
    .transform((value) => value ?? undefined);
}

// ─── Public Booking ───
//
// `bookingStatusSchema`/`collectionStatusSchema`/`bookingRefundStatusSchema`
// below duplicate the three enums `booking.schema.ts` also declares, rather
// than importing them from there: `booking.ts` imports `RefundEnvelope` from
// `publicBooking.ts` for `CancelBookingResponse`, so `booking.schema.ts`
// needs this file's `refundEnvelopeSchema` — importing back from
// `booking.schema.ts` here would close a real runtime circular-import loop
// between two modules that isn't just recursive schema shapes (which
// `z.lazy` handles) but two `z.enum` literals cheap enough to duplicate
// instead. Both copies are typed against the same handwritten union via
// `satisfies`, so a drift between them fails `tsc`, not just review.

const bookingStatusSchema = z.enum([
  'pending',
  'confirmed',
  'cancelled',
  'completed',
  'no_show',
]) satisfies z.ZodType<BookingStatus>;

const collectionStatusSchema = z.enum(['unpaid', 'deposit_paid', 'fully_paid']) satisfies z.ZodType<CollectionStatus>;

const bookingRefundStatusSchema = z.enum([
  'none',
  'pending',
  'partial',
  'full',
]) satisfies z.ZodType<BookingRefundStatus>;

const publicComplexSchema = exact<PublicComplex>(
  z
    .object({
      id: z.string(),
      name: z.string(),
      slug: z.string(),
      address: z.string(),
      city: z.string(),
      province: z.string(),
      country_code: z.string(),
      currency: z.string(),
      phone: z.string(),
      // `openapi.yaml`'s `PublicComplex` declares these absent-or-string
      // while its `Complex` also allows null, and the two are projections of
      // one row: a venue whose email was cleared sends null on the owner path
      // and, by the document, nothing on the storefront. Accepting both and
      // folding null into "absent" keeps the derived type — the storefront
      // has no "cleared" state to render, only "has one" or "does not".
      email: nullableToAbsent(z.string()),
      logo_url: nullableToAbsent(z.string()),
      cover_url: nullableToAbsent(z.string()),
      deposit_percentage: z.number(),
      cancellation_hours: z.number(),
      latitude: nullableToAbsent(z.number()),
      longitude: nullableToAbsent(z.number()),
      amenities: z.array(amenitySchema),
      payments_enabled: z.boolean(),
    })
    .loose(),
);

export const publicComplexResponseSchema = z
  .object({
    complex: publicComplexSchema,
    // Nullable, not just an array: a Go `nil` slice (zero courts matched at
    // the instant this was queried — seen under concurrent court writes on a
    // shared E2E complex, e.g. blocked-slots-availability.spec.ts) marshals
    // to JSON `null`, not `[]`. Treat it the same as empty rather than
    // failing the whole page's schema.
    courts: z
      .array(courtWithPricesSchema)
      .nullable()
      .transform((v) => v ?? []),
    schedules: z.array(scheduleSchema),
  })
  .loose() satisfies z.ZodType<PublicComplexResponse>;

const publicBookingEnvelopeSchema = z
  .object({
    status: bookingStatusSchema,
    collection_status: collectionStatusSchema,
    refund_status: bookingRefundStatusSchema,
    date: z.string(),
    start_time: z.string(),
    starts_at: z.string(),
    ends_at: z.string(),
    court_name: z.string(),
    complex_name: z.string(),
    price: z.number(),
    deposit_amount: z.number(),
  })
  .loose() satisfies z.ZodType<PublicBookingEnvelope>;

export const publicBookingResponseSchema = exact<PublicBookingResponse>(
  z
    .object({
      booking: publicBookingEnvelopeSchema,
      token: z.string(),
      mp_init_point: z.string().optional(),
      mp_preference_id: z.string().optional(),
      service_fee: z.number().optional(),
      total_client_pays: z.number().optional(),
    })
    .loose(),
);

/** `mp_user_id` is required here, unlike on `mp/status`: a successful connect always names the account it linked. */
export const mpConnectResponseSchema = exact<MPConnectResponse>(
  z
    .object({
      connected: z.boolean(),
      mp_user_id: z.string(),
    })
    .loose(),
);

export const mpStatusResponseSchema = exact<MPStatusResponse>(
  z
    .object({
      connected: z.boolean(),
      mp_user_id: z.string().optional(),
      app_id: z.string().optional(),
    })
    .loose(),
);

const bookingStatusCancellationSchema = exact<BookingStatusCancellation>(
  z
    .object({
      can_cancel: z.boolean(),
      // Null when there is no fixed deadline (cancellable with refund right
      // up to the turn itself), and absent on a server that predates it.
      refund_deadline: z.string().nullable().optional(),
      can_refund_now: z.boolean(),
      cancellation_hours: z.number(),
    })
    .loose(),
);

/**
 * Only `status`, `collection_status` and `refund_status` are guaranteed by
 * the server (see the type's own doc comment) — every other field stays
 * `.optional()` to match, so a booking answered by an older server still
 * parses instead of throwing on an absent field the UI already falls back for.
 */
const bookingStatusDetailsSchema = exact<BookingStatusDetails>(
  z
    .object({
      status: bookingStatusSchema,
      collection_status: collectionStatusSchema,
      refund_status: bookingRefundStatusSchema,
      complex_name: z.string().optional(),
      complex_address: z.string().optional(),
      complex_phone: nullableToAbsent(z.string()),
      court_name: z.string().optional(),
      sport: sportSchema.optional(),
      court_type: courtTypeSchema.optional(),
      date: z.string().optional(),
      start_time: z.string().optional(),
      starts_at: z.string().optional(),
      ends_at: z.string().optional(),
      duration_minutes: durationMinutesSchema.optional(),
      price: z.number().optional(),
      deposit_amount: z.number().optional(),
      service_fee: z.number().optional(),
      remaining_amount: z.number().optional(),
      cancellation: bookingStatusCancellationSchema.optional(),
    })
    .loose(),
);

export const bookingStatusResponseSchema = z
  .object({
    booking: bookingStatusDetailsSchema,
  })
  .loose() satisfies z.ZodType<BookingStatusResponse>;

const refundMethodSchema = z.enum(['mercadopago', 'manual', 'none']) satisfies z.ZodType<RefundMethod>;

export const cancelInfoResponseSchema = exact<CancelInfoResponse>(
  z
    .object({
      booking: z
        .object({
          status: bookingStatusSchema,
          date: z.string(),
          start_time: z.string(),
          court_name: z.string(),
          complex_name: z.string(),
          sport: sportSchema.optional(),
          court_type: courtTypeSchema.optional(),
          starts_at: z.string().optional(),
          ends_at: z.string().optional(),
          duration_minutes: durationMinutesSchema.optional(),
          complex_address: z.string().optional(),
        })
        .loose(),
      can_cancel: z.boolean(),
      can_refund: z.boolean(),
      refund_method: refundMethodSchema,
      cancellation_hours: z.number(),
      refund_amount: z.number().optional(),
      paid_amount: z.number().optional(),
    })
    .loose(),
);

const refundStatusSchema = z.enum([
  'none',
  'not_eligible',
  'issued',
  'already_issued',
  'queued',
  'manual',
]) satisfies z.ZodType<RefundStatus>;

export const refundEnvelopeSchema = exact<RefundEnvelope>(
  z
    .object({
      status: refundStatusSchema,
      message: z.string(),
      amount: z.number().optional(),
      manual_amount: z.number().optional(),
      manual_message: z.string().optional(),
    })
    .loose(),
);

export const publicCancelBookingResponseSchema = z
  .object({
    booking: z
      .object({
        status: bookingStatusSchema,
        collection_status: collectionStatusSchema,
        refund_status: bookingRefundStatusSchema,
      })
      .loose(),
    refunded: z.boolean(),
    refund: refundEnvelopeSchema,
  })
  .loose() satisfies z.ZodType<PublicCancelBookingResponse>;
