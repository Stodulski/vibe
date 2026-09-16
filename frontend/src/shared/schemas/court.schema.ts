import { z } from 'zod';
import type {
  Sport,
  CourtType,
  DayType,
  DurationMinutes,
  Court,
  CourtPrice,
  CourtWithPrices,
  BlockedSlot,
} from '@/shared/types/api.types';
import { blockedSlotSchema } from './complex.schema';
import { exact } from '@/shared/lib/apiParse';

// ─── Court ───

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
]) satisfies z.ZodType<DayType>;

export const durationMinutesSchema = z.union([
  z.literal(60),
  z.literal(90),
  z.literal(120),
]) satisfies z.ZodType<DurationMinutes>;

// Kept unwrapped (not run through `exact`) so `courtWithPricesSchema` below
// can still call `.extend()` on it — `exact` returns a plain `z.ZodType`,
// which drops `ZodObject`-only methods.
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

export const courtSchema = exact<Court>(courtShape);

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

// ─── Responses (`courtsApi`) ───

export const courtsListEnvelopeSchema = z
  .object({
    courts: z.array(courtWithPricesSchema),
  })
  .loose() satisfies z.ZodType<{ courts: CourtWithPrices[] }>;

export const courtEnvelopeSchema = z
  .object({
    court: courtSchema,
  })
  .loose() satisfies z.ZodType<{ court: Court }>;

export const pricesEnvelopeSchema = z
  .object({
    prices: z.array(courtPriceSchema),
  })
  .loose() satisfies z.ZodType<{ prices: CourtPrice[] }>;

/** `BlockedSlot` lives in `complex.ts`, not `court.ts` — `blockSlot`/`listBlockedSlots` are `courtsApi` endpoints that return it anyway. */
export const blockedSlotEnvelopeSchema = z
  .object({
    blocked_slot: blockedSlotSchema,
  })
  .loose() satisfies z.ZodType<{ blocked_slot: BlockedSlot }>;

export const blockedSlotsEnvelopeSchema = z
  .object({
    blocked_slots: z.array(blockedSlotSchema),
  })
  .loose() satisfies z.ZodType<{ blocked_slots: BlockedSlot[] }>;
