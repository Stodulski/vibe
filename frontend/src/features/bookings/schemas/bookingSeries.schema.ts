import { z } from 'zod';
import { exact } from '@/shared/lib/apiParse';
import { bookingSchema } from '@/shared/schemas';
import type {
  BookingSeries,
  BookingSeriesConflict,
  BookingSeriesDetailResponse,
  BookingSeriesUpcoming,
  CreateBookingSeriesResponse,
  EndBookingSeriesResponse,
  SkipSeriesDateResponse,
} from '@/shared/types/api.types';

const bookingSeriesSchema = exact<BookingSeries>(
  z
    .object({
      id: z.string(),
      complex_id: z.string(),
      court_id: z.string(),
      client_id: z.string(),
      weekday: z.number(),
      start_time: z.string(),
      duration_minutes: z.number(),
      price: z.number().nullable().optional(),
      notes: z.string().nullable().optional(),
      starts_on: z.string(),
      ends_on: z.string().nullable().optional(),
      status: z.enum(['active', 'ended']),
      created_at: z.string(),
    })
    .loose(),
);

const seriesReasonSchema = z.enum([
  'cancelled_by_owner',
  'overlap',
  'blocked',
  'closed_day',
  'no_price',
  'client_blocked',
  'missed',
]);

const seriesConflictSchema = exact<BookingSeriesConflict>(
  z.object({ date: z.string(), reason: seriesReasonSchema }).loose(),
);

const seriesUpcomingSchema = exact<BookingSeriesUpcoming>(
  z
    .object({
      date: z.string(),
      state: z.enum(['pending', 'booked', 'exception']),
      booking_id: z.string().optional(),
      reason: seriesReasonSchema.optional(),
    })
    .loose(),
);

export const createBookingSeriesResponseSchema = exact<CreateBookingSeriesResponse>(
  z
    .object({
      series: bookingSeriesSchema,
      bookings: z.array(bookingSchema),
      skipped: z.array(seriesConflictSchema),
    })
    .loose(),
);

export const bookingSeriesDetailResponseSchema = exact<BookingSeriesDetailResponse>(
  z
    .object({
      series: bookingSeriesSchema,
      upcoming: z.array(seriesUpcomingSchema),
    })
    .loose(),
);

export const skipSeriesDateResponseSchema = exact<SkipSeriesDateResponse>(
  z.object({ skipped: seriesConflictSchema }).loose(),
);

export const endBookingSeriesResponseSchema = exact<EndBookingSeriesResponse>(
  z
    .object({
      series: bookingSeriesSchema,
      cancelled: z.array(bookingSchema),
      kept: z.array(bookingSchema),
    })
    .loose(),
);
