import { z } from 'zod';
import type { Amenity, Schedule, Weekday } from '../api/types';

// The complex pieces the storefront payloads use. The app keeps its own full
// `shared/schemas/complex.schema.ts` for the owner panel; these two must agree
// with it, and the types they are checked against are asserted in the app.

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

const weekdaySchema = z.enum([
  'monday',
  'tuesday',
  'wednesday',
  'thursday',
  'friday',
  'saturday',
  'sunday',
]) satisfies z.ZodType<Weekday>;

export const scheduleSchema = z
  .object({
    id: z.string(),
    complex_id: z.string(),
    day: weekdaySchema,
    open_time: z.string(),
    close_time: z.string(),
    is_closed: z.boolean(),
  })
  .loose() satisfies z.ZodType<Schedule>;
