import { z } from 'zod';
import type {
  Booking,
  BookingStatus,
  CollectionStatus,
  BookingRefundStatus,
  BookingsListResponse,
  BookingDetailResponse,
  CancelBookingResponse,
  ConfirmPaymentResponse,
  ManualRefundResponse,
  Client,
} from '@/shared/types/api.types';
import { paginationMetadataSchema } from './envelope.schema';
import { paymentSchema } from './payment.schema';
import { refundEnvelopeSchema } from './publicBooking.schema';
// `clientSchema` lives in `client.schema.ts`, which imports `bookingSchema`
// from this file for `ClientDetailResponse` — see the matching note there.
// `z.lazy` on both sides breaks the eager-evaluation order problem.
import { clientSchema } from './client.schema';
import { exact } from '@/shared/lib/apiParse';

// ─── Booking ───
//
// `CreateBookingRequest`, `CancelBookingRequest`, `ConfirmPaymentRequest` and
// `UpdateBookingRequest` are request bodies, not responses — no schema here.
// `paymentDisplayStatus` is a pure function, not a wire shape — nothing to
// validate.

export const bookingStatusSchema = z.enum([
  'pending',
  'confirmed',
  'cancelled',
  'completed',
  'no_show',
]) satisfies z.ZodType<BookingStatus>;

export const collectionStatusSchema = z.enum([
  'unpaid',
  'deposit_paid',
  'fully_paid',
]) satisfies z.ZodType<CollectionStatus>;

export const bookingRefundStatusSchema = z.enum([
  'none',
  'pending',
  'partial',
  'full',
]) satisfies z.ZodType<BookingRefundStatus>;

export const bookingSchema = exact<Booking>(
  z
    .object({
      id: z.string(),
      complex_id: z.string(),
      court_id: z.string(),
      client_id: z.string(),
      starts_at: z.string(),
      ends_at: z.string(),
      date: z.string(),
      start_time: z.string(),
      duration_minutes: z.number(),
      price: z.number(),
      deposit_amount: z.number(),
      status: bookingStatusSchema,
      collection_status: collectionStatusSchema,
      refund_status: bookingRefundStatusSchema,
      notes: z.string().nullable().optional(),
      court_name: z.string(),
      client_name: z.string(),
      client_phone: z.string(),
      reminder_sent_2h: z.boolean(),
      created_by: z.string().optional(),
      created_at: z.string(),
      updated_at: z.string(),
    })
    .loose(),
);

export const bookingsListResponseSchema = z
  .object({
    bookings: z.array(bookingSchema),
    metadata: paginationMetadataSchema,
  })
  .loose() satisfies z.ZodType<BookingsListResponse>;

export const bookingDetailResponseSchema = exact<BookingDetailResponse>(
  z
    .object({
      booking: bookingSchema,
      client: z.lazy((): z.ZodType<Client> => clientSchema).optional(),
      payment: paymentSchema.optional(),
      payments: z.array(paymentSchema),
    })
    .loose(),
);

/** `{ booking: Booking }` — `bookingsApi.create` / `bookingsApi.update`. */
export const bookingEnvelopeSchema = z
  .object({
    booking: bookingSchema,
  })
  .loose() satisfies z.ZodType<{ booking: Booking }>;

export const cancelBookingResponseSchema = z
  .object({
    booking: bookingSchema,
    refund: refundEnvelopeSchema,
  })
  .loose() satisfies z.ZodType<CancelBookingResponse>;

export const confirmPaymentResponseSchema = z
  .object({
    booking: bookingSchema,
    payment: paymentSchema,
  })
  .loose() satisfies z.ZodType<ConfirmPaymentResponse>;

export const manualRefundResponseSchema = z
  .object({
    booking: bookingSchema,
    payments: z.array(paymentSchema),
    returned_amount: z.number(),
  })
  .loose() satisfies z.ZodType<ManualRefundResponse>;
