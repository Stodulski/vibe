import { timeToMinutes } from '@/shared/lib/time';
import { ALL_DAYS, type PriceFormValues } from './days';
import type { DayPriceValues, PriceBandValues } from '../../schemas/courts.schema';
import type { CourtWithPrices, DayType, Schedule } from '@/shared/types/api.types';

/** Minutes in a calendar day — the modulus every wrap-past-midnight sum below reduces by. */
const MINUTES_PER_DAY = 24 * 60;

/**
 * How much of the window a new differentiated row claims by default. See
 * `nextDifferentiatedBand`.
 */
const NEW_BAND_MINUTES = 60;

/**
 * Builds a typed react-hook-form field path for one differentiated band's
 * field. `day.price` — the full-day rate — has its own path, `dayPriceField`;
 * this one is only for `day.bands[idx]`.
 *
 * Same shape and same reason as `scheduleField` on the complex side:
 * `String(idx)` keeps `restrict-template-expressions` happy while the cast
 * restores the literal `${DayType}.bands.${number}.${K}` type RHF needs to
 * resolve the field's value type.
 */
export function bandField<K extends keyof PriceBandValues>(
  day: DayType,
  idx: number,
  key: K,
): `${DayType}.bands.${number}.${K}` {
  return `${day}.bands.${String(idx)}.${key}` as `${DayType}.bands.${number}.${K}`;
}

/** The react-hook-form field path for a day's full-day price. */
export function dayPriceField(day: DayType): `${DayType}.price` {
  return `${day}.price`;
}

/**
 * The hours a day's full-day price covers: that day's own opening window.
 * The owner never types these — they come from the schedule tab, the same
 * way they always have.
 *
 * It used to write 00:00–23:59 for every day, which stops at the minute before
 * midnight — so a venue trading Thursday 08:00 to 01:30 had its last four hours
 * unpriced no matter what the owner typed, and an unpriced hour is not for sale.
 * Writing the window instead makes the two agree by construction: whatever is
 * open is priced, and nothing else is.
 *
 * A closed day still gets a band, covering its calendar day. Staff book days
 * the venue is shut, and the rate they are charged comes from that day's card.
 *
 * A window that runs past midnight (open_time > close_time, e.g. "08:00" to
 * "01:30") is kept as-is rather than split or clamped: the server reads
 * time_to <= time_from as "this band ends the next day" (the span_min generated
 * column), the same rule complex_schedules has always used for opening hours,
 * so a band built straight from the schedule already lands in the shape the
 * server expects.
 */
export function openingBandFor(schedules: Schedule[], day: DayType): { time_from: string; time_to: string } {
  const row = schedules.find((s) => s.day === day);
  if (!row || row.is_closed || row.open_time === row.close_time) {
    return { time_from: '00:00', time_to: '23:59' };
  }
  return { time_from: row.open_time, time_to: row.close_time };
}

/**
 * Whether a band's end reads earlier than its start, i.e. it runs into the
 * next day. `BandRow` folds this into the `Hasta` select's own accessible
 * name — there is no visible marker for it any more (removed dialog-wide by
 * owner instruction, including `formatHourRange`'s own "Día sig." on the
 * bookings side) — but the fact itself, and the ambiguity a range like
 * "22:00 a 01:30" would read as without it, still needs answering somewhere.
 */
export function endsNextDay(band: { time_from: string; time_to: string }): boolean {
  return timeToMinutes(band.time_to) <= timeToMinutes(band.time_from);
}

/**
 * Minutes since midnight, formatted back to "HH:MM" and wrapped into
 * `[0, 1440)` first — the inverse of the `+= MINUTES_PER_DAY` bookkeeping
 * `buildDayBands`/`nextDifferentiatedBand` do to place a time past its own
 * literal midnight. The double modulo (`%` in JS keeps the dividend's sign)
 * is what makes a negative total — a differentiated row's default running a
 * new band an hour before "00:30" — read back as "23:30" instead of "-1:30".
 */
function minutesToTime(total: number): string {
  const wrapped = ((total % MINUTES_PER_DAY) + MINUTES_PER_DAY) % MINUTES_PER_DAY;
  return `${String(Math.floor(wrapped / 60)).padStart(2, '0')}:${String(wrapped % 60).padStart(2, '0')}`;
}

/**
 * The window's own span, extended past 1440 when it crosses midnight — the
 * timeline `buildDayBands` walks and `nextDifferentiatedBand` anchors a new
 * row's default against, both for the same reason `bandSpan` extends a
 * band's own end in `courts.schema.ts`: compared as raw clock minutes, a
 * window that runs past midnight looks like it ends before it starts.
 */
function windowSpan(schedules: Schedule[], day: DayType): { from: number; to: number } {
  const window = openingBandFor(schedules, day);
  const from = timeToMinutes(window.time_from);
  const rawTo = timeToMinutes(window.time_to);
  return { from, to: rawTo <= from ? rawTo + MINUTES_PER_DAY : rawTo };
}

/**
 * The differentiated row the "Nuevo precio" button adds.
 *
 * With nothing on the day yet, it claims the window's last hour — an evening
 * surcharge is the common case in this domain, and with no existing row to
 * anchor to, the window's own closing edge is the one the owner is most
 * likely reaching for. With a row already there, it claims the hour right
 * before whichever one starts earliest, growing the exception zone backward
 * a press at a time rather than opening a second, disconnected block.
 *
 * Never before the window opens, in the ordinary (non-overnight) case: a
 * default is a starting point, not a command, and it should not suggest
 * hours the venue is closed. An overnight window's own start reads as a
 * LATER clock time than its end (e.g. "08:00" opens after "01:30" closes, as
 * plain strings), so that clamp is skipped there rather than risk clamping
 * the wrong direction — the owner adjusts the two ends by hand either way.
 */
export function nextDifferentiatedBand(bands: PriceBandValues[], schedules: Schedule[], day: DayType): PriceBandValues {
  const window = openingBandFor(schedules, day);
  const anchor =
    bands.length === 0
      ? window.time_to
      : bands.reduce((a, b) => (timeToMinutes(b.time_from) < timeToMinutes(a.time_from) ? b : a)).time_from;

  let from = minutesToTime(timeToMinutes(anchor) - NEW_BAND_MINUTES);
  if (!endsNextDay(window) && timeToMinutes(from) < timeToMinutes(window.time_from)) {
    from = window.time_from;
  }
  return { time_from: from, time_to: anchor, price: Number.NaN };
}

/** One wire-shaped band, and which form control is responsible for it. */
export interface BuiltBand {
  time_from: string;
  time_to: string;
  price: number;
  /**
   * `'day'`: generated to fill a gap the differentiated rows leave, at the
   * full-day price — nothing on screen owns this specific row, so a server
   * error naming it has to fall back to the day's price field. `'band'`:
   * came straight from one differentiated row, at that row's own hours and
   * price.
   */
  source: { kind: 'day' } | { kind: 'band'; index: number };
}

/**
 * One day's full-day price and differentiated rows, turned into the flat
 * list of non-overlapping bands the server has always expected.
 *
 * With no differentiated rows, this is exactly the single band the dialog
 * sent before this feature existed — the full-day price, over the day's
 * whole opening window, unchanged in shape. With rows, each one is sent as
 * itself, and the full-day price fills whatever the rows leave uncovered:
 * before the earliest row, between two rows, and after the latest one. That
 * is what "differentiated prices are exceptions layered on top of the
 * full-day price, never a replacement for it" means on the wire — the day
 * ends up fully priced, with no gaps and no overlaps, the same guarantee the
 * single-band shape always gave for free.
 */
export function buildDayBands(day: DayPriceValues, schedules: Schedule[], dayType: DayType): BuiltBand[] {
  const window = openingBandFor(schedules, dayType);

  if (day.bands.length === 0) {
    return [{ time_from: window.time_from, time_to: window.time_to, price: day.price, source: { kind: 'day' } }];
  }

  const { from: windowFrom, to: windowTo } = windowSpan(schedules, dayType);

  // Anchored inside the WINDOW's own timeline, not each row's own [0, 1440) —
  // otherwise a row entered as "00:30–01:00" would read as happening before
  // an 08:00-opening window instead of after it wraps past midnight, which
  // is where the owner sees it on screen.
  const anchored = day.bands.map((band, index) => {
    let from = timeToMinutes(band.time_from);
    while (from < windowFrom) from += MINUTES_PER_DAY;
    let to = timeToMinutes(band.time_to);
    while (to <= from) to += MINUTES_PER_DAY;
    return { from, to, price: band.price, index };
  });
  anchored.sort((a, b) => a.from - b.from);

  const result: BuiltBand[] = [];
  let cursor = windowFrom;
  for (const band of anchored) {
    if (cursor < band.from) {
      result.push({
        time_from: minutesToTime(cursor),
        time_to: minutesToTime(band.from),
        price: day.price,
        source: { kind: 'day' },
      });
    }
    result.push({
      time_from: minutesToTime(band.from),
      time_to: minutesToTime(band.to),
      price: band.price,
      source: { kind: 'band', index: band.index },
    });
    cursor = Math.max(cursor, band.to);
  }
  if (cursor < windowTo) {
    result.push({
      time_from: minutesToTime(cursor),
      time_to: minutesToTime(windowTo),
      price: day.price,
      source: { kind: 'day' },
    });
  }
  return result;
}

/** How long a stored band runs, in minutes, wrapping an overnight one past midnight. */
function bandMinutes(b: { time_from: string; time_to: string }): number {
  const from = timeToMinutes(b.time_from);
  const to = timeToMinutes(b.time_to);
  return (to <= from ? to + MINUTES_PER_DAY : to) - from;
}

/**
 * One day's stored bands, reconstructed into the full-day-price-plus-
 * exceptions shape this form edits.
 *
 * The wire format carries no tag saying which stored band, if any, was "the
 * base rate" — it is a flat list the server has always treated as already
 * non-overlapping and fully covering the window. With more than one stored
 * band, the longest-covering one becomes the full-day price (the rate that
 * was already true for most of the day) and every other one becomes a
 * differentiated row, kept at its own stored hours. Saving again regenerates
 * the same set through `buildDayBands`, as long as the stored bands already
 * tiled the window with no gaps — the only shape this dialog itself has ever
 * written.
 *
 * A day the court has no row for gets a blank full-day price and no rows —
 * that IS the "this day has no rate" state `usePriceConfigForm` drops again
 * on submit, same as before this feature existed.
 */
export function dayPriceValues(court: CourtWithPrices, day: DayType): DayPriceValues {
  const stored = court.prices
    .filter((p) => p.day_type === day)
    .sort((a, b) => timeToMinutes(a.time_from) - timeToMinutes(b.time_from));

  if (stored.length === 0) {
    return { price: Number.NaN, bands: [] };
  }

  // One stored band falls out of the same reduce below as its own base, with
  // no other entry left to become a row — so a plain, never-split day (the
  // overwhelming common case) needs no special case here.
  const withMinutes = stored.map((p) => ({ ...p, minutes: bandMinutes(p) }));
  const base = withMinutes.reduce((a, b) => (b.minutes > a.minutes ? b : a));
  const bands = withMinutes
    .filter((p) => p !== base)
    .map((p) => ({ time_from: p.time_from, time_to: p.time_to, price: p.price / 100 }));
  return { price: base.price / 100, bands };
}

/** The form's `defaultValues`: every weekday's full-day price and rows, in pesos. */
export function priceFormValues(court: CourtWithPrices): PriceFormValues {
  const values = {} as PriceFormValues;
  for (const { value: day } of ALL_DAYS) {
    values[day] = dayPriceValues(court, day);
  }
  return values;
}
