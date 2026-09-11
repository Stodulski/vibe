import { z } from 'zod';
import type { AvailabilitySlot, CourtAvailability, AvailabilityData } from '@/shared/types/api.types';
import { exact } from '@/shared/lib/apiParse';

// ─── Availability ───

export const availabilitySlotSchema = z
  .object({
    start_time: z.string(),
    end_time: z.string(),
    start_min: z.number(),
    duration_minutes: z.number(),
    price: z.number(),
    available: z.boolean(),
  })
  .loose() satisfies z.ZodType<AvailabilitySlot>;

export const courtAvailabilitySchema = exact<CourtAvailability>(
  z
    .object({
      court_id: z.string(),
      court_name: z.string(),
      sport: z.string(),
      court_type: z.string(),
      description: z.string().optional(),
      duration_minutes: z.number().optional(),
      slots: z.array(availabilitySlotSchema),
    })
    .loose(),
);

export const availabilityDataSchema = z
  .object({
    date: z.string(),
    day: z.string(),
    is_open: z.boolean(),
    courts: z.array(courtAvailabilitySchema),
  })
  .loose() satisfies z.ZodType<AvailabilityData>;

/** `{ availability: AvailabilityData }` — `publicBookingApi.getAvailability`. */
export const availabilityEnvelopeSchema = z
  .object({
    availability: availabilityDataSchema,
  })
  .loose() satisfies z.ZodType<{ availability: AvailabilityData }>;
