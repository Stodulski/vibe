import { z } from 'zod';
import type { AvailabilitySlot, CourtAvailability, AvailabilityData } from '@/shared/types/api.types';
import { exact } from '@/shared/lib/apiParse';
import { dayOfWeekSchema } from './complex.schema';
import { courtTypeSchema, sportSchema } from './court.schema';

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

/**
 * `sport`, `court_type` and `day` were `z.string()` while the handwritten
 * types said the same; `openapi.yaml` declares all three as closed
 * vocabularies, and the slot grid keys its icons and labels off them.
 *
 * There is no `duration_minutes` here any more either: the handwritten type
 * carried one, marked "not sent by the server", and nothing ever read it.
 */
export const courtAvailabilitySchema = exact<CourtAvailability>(
  z
    .object({
      court_id: z.string(),
      court_name: z.string(),
      sport: sportSchema,
      court_type: courtTypeSchema,
      description: z.string().optional(),
      slots: z.array(availabilitySlotSchema),
    })
    .loose(),
);

export const availabilityDataSchema = z
  .object({
    date: z.string(),
    day: dayOfWeekSchema,
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
