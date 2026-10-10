import { z } from 'zod';
import type {
  BookingRefundStatus,
  BookingStatus,
  BookingStatusCancellation,
  BookingStatusDetails,
  BookingStatusResponse,
  CancelInfoResponse,
  CollectionStatus,
  PublicBookingEnvelope,
  PublicBookingResponse,
  PublicCancelBookingResponse,
  PublicComplex,
  PublicComplexResponse,
  RefundEnvelope,
  RefundMethod,
  RefundStatus,
} from '../api/types';
import { exact } from '../lib/apiParse';
import { amenitySchema, scheduleSchema } from './complex.schema';
import { courtTypeSchema, courtWithPricesSchema, durationMinutesSchema, sportSchema } from './court.schema';

/**
 * Reads a field that may arrive as `null`, as the key with a value, or not at
 * all, and yields `undefined` for the first and last. A cleared column
 * serializes as null on one projection of a row and as nothing on the other.
 */
function nullableToAbsent<T extends z.ZodType>(inner: T) {
  return inner
    .nullable()
    .optional()
    .transform((value) => value ?? undefined);
}

// The status enums are declared here rather than imported from the app. Each
// is checked against its type with `satisfies`, so a drift fails `tsc`.

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
      // Absent or null on the owner path; the storefront has no "cleared"
      // state to render, only "has one" or "does not".
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
    // Nullable, not just an array: a Go nil slice marshals to JSON null.
    // Treat it the same as empty rather than failing the whole page.
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

const bookingStatusCancellationSchema = exact<BookingStatusCancellation>(
  z
    .object({
      can_cancel: z.boolean(),
      // Null when there is no fixed deadline, and absent on a server that predates it.
      refund_deadline: z.string().nullable().optional(),
      can_refund_now: z.boolean(),
      cancellation_hours: z.number(),
    })
    .loose(),
);

/**
 * Only `status`, `collection_status` and `refund_status` are guaranteed; every
 * other field stays optional so a booking answered by an older server still
 * parses.
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

const refundEnvelopeSchema = exact<RefundEnvelope>(
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
