import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export const createCourtSchema = z.object({
  name: z.string().min(1, t.validation.nameRequired).max(100, t.validation.maxChars100),
  sport: z.enum(['padel', 'tennis', 'soccer', 'basketball', 'volleyball', 'hockey', 'pickleball'], {
    message: t.validation.sportInvalid,
  }),
  court_type: z.enum(['indoor', 'outdoor', 'semi_covered'], {
    message: t.validation.courtTypeInvalid,
  }),
  // Capped at 200 where the column is capped (courts_description_length): about two
  // lines on a storefront card, which is what a player skims while comparing
  // two courts.
  //
  // Optional in the schema and not merely empty-able: a court with no
  // description is a valid court, and a caller that has none to give should
  // not have to send "" to say so. The form does send a string, and an empty
  // one is folded back to null by the server.
  description: z.string().max(200, t.validation.maxChars200).optional(),
});

const priceItemSchema = z
  .object({
    price: z.number().min(1, t.validation.priceNonNegative),
    day_type: z.enum(['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'], {
      message: t.validation.dayInvalid,
    }),
    time_from: z.string().min(1, t.validation.timeRequired),
    time_to: z.string().min(1, t.validation.timeRequired),
  })
  // Only equal ends are refused, not an end that reads earlier than its start.
  // A band ending before it begins is how a venue trading past midnight prices
  // its late hours — "22:00 to 01:30" — and the schedule form has accepted the
  // same shape for opening hours all along, labelling it "Día sig.". Equal ends
  // stay out because they say nothing: the server reads them as a full day
  // while an owner typing the same time twice meant an empty band, and neither
  // reading is worth guessing at (the span_min generated column).
  .refine((data) => data.time_from !== data.time_to, {
    message: t.validation.timeRangeNotEmpty,
    path: ['time_to'],
  });

export const updatePricesSchema = z.object({
  prices: z.array(priceItemSchema).min(1, t.validation.atLeastOnePrice),
});

// One field per weekday, matching the price-config dialog's form shape
// (PriceFormValues) rather than the wire shape above — the dialog asks for
// one number per day and derives day_type/time_from/time_to itself
// (usePriceConfigForm's bandFor), so there is nothing for a client-side
// schema to check about those beyond the number the owner actually typed.
//
// A blank field reads back from react-hook-form's `valueAsNumber` as NaN, and
// NaN means the same thing 0 does here — this day has no price set, and
// usePriceConfigForm filters it out of the request before sending. z.nan()
// is listed as its own alternative rather than folded into 0 by a preprocess
// step, because a preprocess step's input type is `unknown`, which breaks
// zodResolver's inference back to PriceFormValues everywhere the form is
// wired up. Only an actual negative number is a mistake worth blocking on.
// The message sits on the union, not only on the number branch: when every
// branch fails, zod reports the union's own error, so a branch-level message
// would never reach the field.
const dayPrice = z.union([z.number().min(0, t.validation.priceNonNegative), z.nan()], {
  error: t.validation.priceNonNegative,
});

export const priceFormSchema = z.object({
  monday: dayPrice,
  tuesday: dayPrice,
  wednesday: dayPrice,
  thursday: dayPrice,
  friday: dayPrice,
  saturday: dayPrice,
  sunday: dayPrice,
});

export type CreateCourtDto = z.infer<typeof createCourtSchema>;
export type PriceFormSchema = z.infer<typeof priceFormSchema>;
