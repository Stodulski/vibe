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

export const publicComplexSchema = exact<PublicComplex>(
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
      email: z.string().nullable().optional(),
      logo_url: z.string().nullable().optional(),
      cover_url: z.string().nullable().optional(),
      deposit_percentage: z.number(),
      cancellation_hours: z.number(),
      latitude: z.number().nullable().optional(),
      longitude: z.number().nullable().optional(),
      amenities: z.array(amenitySchema),
      payments_enabled: z.boolean(),
    })
    .loose(),
);

export const publicComplexResponseSchema = z
  .object({
    complex: publicComplexSchema,
    courts: z.array(courtWithPricesSchema),
    schedules: z.array(scheduleSchema),
  })
  .loose() satisfies z.ZodType<PublicComplexResponse>;

export const publicBookingEnvelopeSchema = z
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

export const mpConnectResponseSchema = exact<MPConnectResponse>(
  z
    .object({
      connected: z.boolean(),
      mp_user_id: z.string().optional(),
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

export const bookingStatusCancellationSchema = z
  .object({
    can_cancel: z.boolean(),
    refund_deadline: z.string().nullable(),
    can_refund_now: z.boolean(),
    cancellation_hours: z.number(),
  })
  .loose() satisfies z.ZodType<BookingStatusCancellation>;

/**
 * Only `status`, `collection_status` and `refund_status` are guaranteed by
 * the server (see the type's own doc comment) — every other field stays
 * `.optional()` to match, so a booking answered by an older server still
 * parses instead of throwing on an absent field the UI already falls back for.
 */
export const bookingStatusDetailsSchema = exact<BookingStatusDetails>(
  z
    .object({
      status: bookingStatusSchema,
      collection_status: collectionStatusSchema,
      refund_status: bookingRefundStatusSchema,
      complex_name: z.string().optional(),
      complex_address: z.string().optional(),
      complex_phone: z.string().nullable().optional(),
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

export const refundMethodSchema = z.enum(['mercadopago', 'manual', 'none']) satisfies z.ZodType<RefundMethod>;

export const cancelInfoResponseSchema = exact<CancelInfoResponse>(
  z
    .object({
      booking: z
        .object({
          status: z.string(),
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

export const refundStatusSchema = z.enum([
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
