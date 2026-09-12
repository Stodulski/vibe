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

/**
 * `sport`, `court_type` and `day` stay `z.string()`, and the types reopen the
 * document's closed vocabularies with `Open<>` to match. This response is the
 * public slot grid — the revenue path — so a venue that adds a court in a
 * sport this build predates must cost that court its label, not the whole
 * day's availability; the labels already fall back to the raw value.
 *
 * There is no `duration_minutes` here any more either: the handwritten type
 * carried one, marked "not sent by the server", and nothing ever read it.
 */
export const courtAvailabilitySchema = exact<CourtAvailability>(
  z
    .object({
      court_id: z.string(),
      court_name: z.string(),
      sport: z.string(),
      court_type: z.string(),
      description: z.string().optional(),
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
