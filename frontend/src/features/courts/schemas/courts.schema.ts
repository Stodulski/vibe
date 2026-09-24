import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';
import { timeToMinutes } from '@/shared/lib/time';
import { hasAtMostTwoDecimals } from '@/shared/lib/money';

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
    price: z
      .number()
      .min(1, t.validation.priceNonNegative)
      .refine(hasAtMostTwoDecimals, t.validation.amountMaxTwoDecimals),
    day_type: z.enum(['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'], {
      message: t.validation.dayInvalid,
    }),
    time_from: z.string().min(1, t.validation.timeRequired),
    time_to: z.string().min(1, t.validation.timeRequired),
  })
  // Only equal ends are refused, not an end that reads earlier than its start.
  // A band ending before it begins is how a venue trading past midnight prices
  // its late hours — "22:00 to 01:30" — and complex_schedules has accepted the
  // same shape for opening hours all along (no visible marker names this any
  // more anywhere in the app — see `endsOnALaterDay` for where that fact
  // still reaches an accessible name instead). Equal ends stay out because
  // they say nothing: the server reads them as a full day while an owner
  // typing the same time twice meant an empty band, and neither reading is
  // worth guessing at (the span_min generated column).
  .refine((data) => data.time_from !== data.time_to, {
    message: t.validation.timeRangeNotEmpty,
    path: ['time_to'],
  });

export const updatePricesSchema = z.object({
  prices: z.array(priceItemSchema).min(1, t.validation.atLeastOnePrice),
});

// A blank field reads back as NaN — `DayRow`/`BandRow` set it explicitly via
// `useMoneyInput`'s `undefined` (the money-formatting field's own way of
// reporting "empty", see that hook's own doc comment) — and NaN means the
// same thing 0 does here — this band has no price set, and
// usePriceConfigForm filters it out of the request before sending. z.nan()
// is listed as its own alternative rather than folded into 0 by a preprocess
// step, because a preprocess step's input type is `unknown`, which breaks
// zodResolver's inference back to PriceFormValues everywhere the form is
// wired up. Only an actual negative number is a mistake worth blocking on
// at this level; a MISSING price is judged per day below, because whether it
// is a mistake depends on how many bands the day has.
// The message sits on the union, not only on the number branch: when every
// branch fails, zod reports the union's own error, so a branch-level message
// would never reach the field.
const bandPrice = z
  .union([z.number().min(0, t.validation.priceNonNegative), z.nan()], {
    error: t.validation.priceNonNegative,
  })
  // NaN (the "no price set" case above) skips the decimals check entirely —
  // it is not an amount at all, and `hasAtMostTwoDecimals(NaN)` is false, so
  // it would otherwise be rejected as an invalid amount instead of read as
  // "unset".
  .refine((value) => Number.isNaN(value) || hasAtMostTwoDecimals(value), t.validation.amountMaxTwoDecimals);

// One differentiated row of the price-config dialog: a half-hour range and
// the hourly rate charged inside it, layered on top of its day's full-day
// price as an exception rather than a replacement for it (see
// `dayPriceSchema`). The wire shape (`priceItemSchema` above) carries
// `day_type` as well; here the day is the key the row's day hangs off, so the
// row does not repeat it.
const priceBandSchema = z.object({
  time_from: z.string().min(1, t.validation.timeRequired),
  time_to: z.string().min(1, t.validation.timeRequired),
  price: bandPrice,
});

const MINUTES_PER_DAY = 24 * 60;

/**
 * A band as the half-open minute span `[from, to)` counted from its OWN
 * weekday's midnight — the same arithmetic the database's `span_min` generated
 * column does, and the same one `CourtPrice.from_min`/`to_min` arrive in.
 *
 * `to <= from` means the band runs into the next day, so the end is pushed a
 * full day out: "22:00 → 01:30" is minutes 1320–1890 of Thursday, not an
 * interval that ends 1230 minutes before it starts. Comparing the two clock
 * readings directly is exactly the bug this avoids — it makes a late band look
 * empty, and makes it overlap every morning band it does not actually touch.
 */
function bandSpan(band: { time_from: string; time_to: string }): { from: number; to: number } {
  const from = timeToMinutes(band.time_from);
  const to = timeToMinutes(band.time_to);
  return { from, to: to <= from ? to + MINUTES_PER_DAY : to };
}

/**
 * One weekday: its full-day price, plus zero or more differentiated rows.
 *
 * The full-day price is the rate that applies to the whole opening window
 * unless a row says otherwise — it is not itself a row, has no hours of its
 * own to check, and is required only once the day HAS a row to fill the gaps
 * around (see the loop below): a day with none is allowed to sit blank,
 * exactly like the single-band day this form showed before it grew
 * differentiated rows, and that blank state is what "I do not price this
 * day" has always meant.
 *
 * Each row, once it exists, is checked the same three ways a band always
 * was: it needs a real (non-equal) range, it needs a price (an exception with
 * no rate is not an exception, it is a hole), and it must not overlap another
 * row on the same day — overlap is judged only between rows, never between a
 * row and the full-day price, because the full-day price never claims hours a
 * row already claims (see `buildDayBands`'s gap-fill).
 */
const dayPriceSchema = z.object({ price: bandPrice, bands: z.array(priceBandSchema) }).superRefine(refineDayPrice);

function refineDayPrice(day: { price: number; bands: z.infer<typeof priceBandSchema>[] }, ctx: z.RefinementCtx): void {
  const { bands } = day;
  for (const [index, band] of bands.entries()) {
    // Equal ends say nothing: the server reads them as a full day while an
    // owner typing the same time twice meant an empty band (see
    // `priceItemSchema`'s own refinement, which refuses the same shape).
    // `bandTimeRangeNotEmpty`, not the shared `timeRangeNotEmpty` this
    // mirrors: this message renders in a ~70–90px compact field at 320px
    // (see `BandRow`), which the shared, longer wording does not fit on one
    // line — `blockSlotForm.schema.ts` keeps the shared key for its own,
    // wider field.
    if (band.time_from === band.time_to) {
      ctx.addIssue({
        code: 'custom',
        message: t.validation.bandTimeRangeNotEmpty,
        path: ['bands', index, 'time_to'],
      });
    }
    if (!(band.price > 0)) {
      ctx.addIssue({ code: 'custom', message: t.validation.priceRequiredForBand, path: ['bands', index, 'price'] });
    }
  }

  // A row carves an exception OUT of the full-day price; the moment one
  // exists, that price is what covers everything the row does not, so it can
  // no longer be left blank the way a plain, row-less day can.
  if (bands.length > 0 && !(day.price > 0)) {
    ctx.addIssue({ code: 'custom', message: t.validation.priceRequiredForDay, path: ['price'] });
  }

  // Pairwise rather than sort-then-scan-neighbours: seven rows at most, and
  // the message has to land on a row the owner can see, which a sorted copy
  // has lost the index of. The complaint goes on the LATER row's start —
  // that is the field they would move to fix it.
  const spans = bands.map(bandSpan);
  for (let i = 0; i < spans.length; i++) {
    for (let j = i + 1; j < spans.length; j++) {
      const a = spans[i];
      const b = spans[j];
      if (!a || !b) continue;
      if (a.from < b.to && b.from < a.to) {
        const later = a.from <= b.from ? j : i;
        ctx.addIssue({ code: 'custom', message: t.validation.bandsOverlap, path: ['bands', later, 'time_from'] });
      }
    }
  }
}

// One full-day-price-plus-rows entry per weekday, matching the price-config
// dialog's form shape (PriceFormValues) rather than the wire shape above —
// the dialog groups rows by day and derives `day_type` from the key, so the
// wire's repeated day name has nothing to validate here.
export const priceFormSchema = z.object({
  monday: dayPriceSchema,
  tuesday: dayPriceSchema,
  wednesday: dayPriceSchema,
  thursday: dayPriceSchema,
  friday: dayPriceSchema,
  saturday: dayPriceSchema,
  sunday: dayPriceSchema,
});

export type CreateCourtDto = z.infer<typeof createCourtSchema>;
export type PriceFormSchema = z.infer<typeof priceFormSchema>;
export type PriceBandValues = z.infer<typeof priceBandSchema>;
export type DayPriceValues = z.infer<typeof dayPriceSchema>;
