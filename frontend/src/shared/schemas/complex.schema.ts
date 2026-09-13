import { z } from 'zod';
import type { Amenity, Complex, DayOfWeek, Schedule, BlockedSlot } from '@/shared/types/api.types';
import { exact } from '@/shared/lib/apiParse';

// ─── Complex ───

export const amenitySchema = z.enum([
  'parking',
  'changing_rooms',
  'showers',
  'bar',
  'racket_rental',
  'pro_shop',
  'wifi',
  'lockers',
  'lessons',
  'tournaments',
  'accessible',
  'match_recording',
]) satisfies z.ZodType<Amenity>;

export const complexSchema = exact<Complex>(
  z
    .object({
      id: z.string(),
      owner_id: z.string(),
      name: z.string(),
      slug: z.string(),
      address: z.string(),
      city: z.string(),
      province: z.string(),
      country_code: z.string(),
      currency: z.string(),
      phone: z.string(),
      // `openapi.yaml` leaves all five out of `Complex.required`: a venue
      // that has never been given one sends no key at all, and one that was
      // cleared sends null. The schema used to demand the key.
      email: z.string().nullable().optional(),
      logo_url: z.string().nullable().optional(),
      cover_url: z.string().nullable().optional(),
      deposit_percentage: z.number(),
      cancellation_hours: z.number(),
      latitude: z.number().nullable().optional(),
      longitude: z.number().nullable().optional(),
      is_active: z.boolean(),
      amenities: z.array(amenitySchema),
      court_count: z.number().optional(),
      payments_enabled: z.boolean(),
      mp_user_id: z.string().nullable().optional(),
      mp_token_expires_at: z.string().nullable().optional(),
      created_at: z.string(),
      updated_at: z.string(),
      // Optimistic-concurrency counter. Always present on a GET — the PUT
      // handlers read it back from the body/`If-Match` to detect a stale write.
      version: z.number(),
    })
    .loose(),
);

export const dayOfWeekSchema = z.enum([
  'monday',
  'tuesday',
  'wednesday',
  'thursday',
  'friday',
  'saturday',
  'sunday',
]) satisfies z.ZodType<DayOfWeek>;

export const scheduleSchema = z
  .object({
    id: z.string(),
    complex_id: z.string(),
    day: dayOfWeekSchema,
    open_time: z.string(),
    close_time: z.string(),
    is_closed: z.boolean(),
  })
  .loose() satisfies z.ZodType<Schedule>;

export const blockedSlotSchema = exact<BlockedSlot>(
  z
    .object({
      id: z.string(),
      court_id: z.string(),
      date: z.string(),
      start_time: z.string(),
      end_time: z.string(),
      reason: z.string().nullable(),
      created_by: z.string().nullable().optional(),
      created_at: z.string(),
      court_name: z.string().optional(),
    })
    .loose(),
);

// ─── Responses (`complexApi`) ───

export const complexesListEnvelopeSchema = z
  .object({
    complexes: z.array(complexSchema),
  })
  .loose() satisfies z.ZodType<{ complexes: Complex[] }>;

export const complexEnvelopeSchema = z
  .object({
    complex: complexSchema,
  })
  .loose() satisfies z.ZodType<{ complex: Complex }>;

export const deleteComplexResponseSchema = z
  .object({
    message: z.string(),
    courts_deactivated: z.number(),
  })
  .loose() satisfies z.ZodType<{ message: string; courts_deactivated: number }>;

export const schedulesEnvelopeSchema = z
  .object({
    schedules: z.array(scheduleSchema),
  })
  .loose() satisfies z.ZodType<{ schedules: Schedule[] }>;

export const slugAvailableResponseSchema = exact<{
  slug: string;
  valid: boolean;
  available: boolean;
  suggestion?: string;
}>(
  z
    .object({
      slug: z.string(),
      valid: z.boolean(),
      available: z.boolean(),
      suggestion: z.string().optional(),
    })
    .loose(),
);
