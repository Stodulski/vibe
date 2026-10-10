import { z } from 'zod';
import type { CourtPrice, CourtType, CourtWithPrices, DurationMinutes, Sport, Weekday } from '../api/types';
import { exact } from '../lib/apiParse';

// The court pieces the storefront payloads use. The app keeps the full
// `shared/schemas/court.schema.ts`; the two must agree.

export const sportSchema = z.enum([
  'padel',
  'tennis',
  'soccer',
  'basketball',
  'volleyball',
  'hockey',
  'pickleball',
]) satisfies z.ZodType<Sport>;

export const courtTypeSchema = z.enum(['indoor', 'outdoor', 'semi_covered']) satisfies z.ZodType<CourtType>;

const dayTypeSchema = z.enum([
  'monday',
  'tuesday',
  'wednesday',
  'thursday',
  'friday',
  'saturday',
  'sunday',
]) satisfies z.ZodType<Weekday>;

export const durationMinutesSchema = z.union([
  z.literal(60),
  z.literal(90),
  z.literal(120),
]) satisfies z.ZodType<DurationMinutes>;

// Kept unwrapped so `courtWithPricesSchema` can call `.extend()` on it.
const courtShape = z
  .object({
    id: z.string(),
    complex_id: z.string(),
    name: z.string(),
    sport: sportSchema,
    court_type: courtTypeSchema,
    is_active: z.boolean(),
    // Absent when the owner has never written one, null when it was cleared.
    description: z.string().nullable().optional(),
    created_at: z.string(),
    updated_at: z.string(),
  })
  .loose();

const courtPriceSchema = z
  .object({
    id: z.string(),
    court_id: z.string(),
    price: z.number(),
    day_type: dayTypeSchema,
    time_from: z.string(),
    time_to: z.string(),
    from_min: z.number(),
    to_min: z.number(),
  })
  .loose() satisfies z.ZodType<CourtPrice>;

export const courtWithPricesSchema = exact<CourtWithPrices>(
  courtShape.extend({
    prices: z.array(courtPriceSchema),
  }),
);
