// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { z } from 'zod';
import { createCourtSchema, priceFormSchema, updatePricesSchema } from './courts.schema';

function omit<T extends object, K extends keyof T>(obj: T, key: K): Omit<T, K> {
  const rest: Partial<T> = { ...obj };
  Reflect.deleteProperty(rest, key);
  return rest as Omit<T, K>;
}

const validData = {
  name: 'Cancha 1',
  sport: 'padel' as const,
  court_type: 'indoor' as const,
};

describe('createCourtSchema - name', () => {
  it('accepts valid court data', () => {
    const result = createCourtSchema.safeParse(validData);
    expect(result.success).toBe(true);
  });

  it('rejects empty name', () => {
    const result = createCourtSchema.safeParse({ ...validData, name: '' });
    expect(result.success).toBe(false);
  });

  it('rejects name longer than 100 characters', () => {
    const result = createCourtSchema.safeParse({ ...validData, name: 'a'.repeat(101) });
    expect(result.success).toBe(false);
  });

  it('accepts name with exactly 100 characters', () => {
    const result = createCourtSchema.safeParse({ ...validData, name: 'a'.repeat(100) });
    expect(result.success).toBe(true);
  });

  it('rejects missing name', () => {
    const result = createCourtSchema.safeParse(omit(validData, 'name'));
    expect(result.success).toBe(false);
  });
});

describe('createCourtSchema - sport and court type', () => {
  it('accepts all valid sport types', () => {
    for (const sport of ['padel', 'tennis', 'soccer', 'basketball', 'volleyball', 'hockey', 'pickleball']) {
      const result = createCourtSchema.safeParse({ ...validData, sport });
      expect(result.success).toBe(true);
    }
  });

  it('rejects invalid sport type', () => {
    const result = createCourtSchema.safeParse({ ...validData, sport: 'rugby' });
    expect(result.success).toBe(false);
  });

  it('rejects missing sport', () => {
    const result = createCourtSchema.safeParse(omit(validData, 'sport'));
    expect(result.success).toBe(false);
  });

  it('accepts all valid court types', () => {
    for (const courtType of ['indoor', 'outdoor', 'semi_covered']) {
      const result = createCourtSchema.safeParse({ ...validData, court_type: courtType });
      expect(result.success).toBe(true);
    }
  });

  it('rejects invalid court type', () => {
    const result = createCourtSchema.safeParse({ ...validData, court_type: 'rooftop' });
    expect(result.success).toBe(false);
  });

  it('rejects missing court_type', () => {
    const result = createCourtSchema.safeParse(omit(validData, 'court_type'));
    expect(result.success).toBe(false);
  });
});

const validPriceItem = {
  price: 5000,
  day_type: 'monday' as const,
  time_from: '08:00',
  time_to: '12:00',
};

describe('updatePricesSchema - prices array and price field', () => {
  it('accepts valid prices array', () => {
    const result = updatePricesSchema.safeParse({ prices: [validPriceItem] });
    expect(result.success).toBe(true);
  });

  it('accepts multiple price items', () => {
    const result = updatePricesSchema.safeParse({
      prices: [validPriceItem, { ...validPriceItem, day_type: 'saturday', price: 8000 }],
    });
    expect(result.success).toBe(true);
  });

  it('rejects empty prices array', () => {
    const result = updatePricesSchema.safeParse({ prices: [] });
    expect(result.success).toBe(false);
  });

  it('rejects missing prices', () => {
    const result = updatePricesSchema.safeParse({});
    expect(result.success).toBe(false);
  });

  it('rejects negative price', () => {
    const result = updatePricesSchema.safeParse({
      prices: [{ ...validPriceItem, price: -1 }],
    });
    expect(result.success).toBe(false);
  });

  it('rejects zero price', () => {
    const result = updatePricesSchema.safeParse({
      prices: [{ ...validPriceItem, price: 0 }],
    });
    expect(result.success).toBe(false);
  });
});

describe('updatePricesSchema - day type and time range', () => {
  it('accepts all valid day types', () => {
    const days = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'];
    for (const dayType of days) {
      const result = updatePricesSchema.safeParse({
        prices: [{ ...validPriceItem, day_type: dayType }],
      });
      expect(result.success).toBe(true);
    }
  });

  it('rejects invalid day type', () => {
    const result = updatePricesSchema.safeParse({
      prices: [{ ...validPriceItem, day_type: 'holiday' }],
    });
    expect(result.success).toBe(false);
  });

  it('rejects empty time_from', () => {
    const result = updatePricesSchema.safeParse({
      prices: [{ ...validPriceItem, time_from: '' }],
    });
    expect(result.success).toBe(false);
  });

  it('rejects empty time_to', () => {
    const result = updatePricesSchema.safeParse({
      prices: [{ ...validPriceItem, time_to: '' }],
    });
    expect(result.success).toBe(false);
  });

  // Refinement: the two ends must differ, and that is all. An end reading
  // earlier than its start is how a band prices hours past midnight.
  it('rejects time_to equal to time_from', () => {
    const result = updatePricesSchema.safeParse({
      prices: [{ ...validPriceItem, time_from: '10:00', time_to: '10:00' }],
    });
    expect(result.success).toBe(false);
  });

  // A venue trading Thursday 08:00 to 01:30 prices those late hours with a
  // band that reads the same way. The schedule form has accepted the shape for
  // opening hours all along; refusing it here left the hours unpriced, and an
  // unpriced hour is not for sale.
  it('accepts a band that runs past midnight', () => {
    const result = updatePricesSchema.safeParse({
      prices: [{ ...validPriceItem, time_from: '22:00', time_to: '01:30' }],
    });
    expect(result.success).toBe(true);
  });

  it('accepts time_to after time_from', () => {
    const result = updatePricesSchema.safeParse({
      prices: [{ ...validPriceItem, time_from: '08:00', time_to: '12:00' }],
    });
    expect(result.success).toBe(true);
  });
});

// ─── priceFormSchema: the dialog's own shape, one full-day price plus rows per weekday ───

// Every day present and unpriced — the state a court with no prices opens in,
// and the baseline each case below overrides one day of.
const BLANK_DAY = { price: Number.NaN, bands: [] };
const BLANK_WEEK = {
  monday: BLANK_DAY,
  tuesday: BLANK_DAY,
  wednesday: BLANK_DAY,
  thursday: BLANK_DAY,
  friday: BLANK_DAY,
  saturday: BLANK_DAY,
  sunday: BLANK_DAY,
};

/** A full week with one day replaced by `dayValue`. */
function form(day: keyof typeof BLANK_WEEK, dayValue: unknown) {
  return { ...BLANK_WEEK, [day]: dayValue };
}

/** The message the schema put on `path`, or undefined. */
function issueAt(result: z.ZodSafeParseResult<unknown>, path: (string | number)[]): string | undefined {
  if (result.success) return undefined;
  return result.error.issues.find((i) => i.path.join('.') === path.join('.'))?.message;
}

describe('priceFormSchema - a plain day (full-day price, no rows)', () => {
  // A blank full-day price with no rows is how the dialog has always said
  // "this day has no rate". Refusing it would make a court with three priced
  // days unsaveable.
  it('accepts a week where every day is blank', () => {
    expect(priceFormSchema.safeParse(BLANK_WEEK).success).toBe(true);
  });

  it('accepts a priced day with no rows', () => {
    expect(priceFormSchema.safeParse(form('monday', { price: 100, bands: [] })).success).toBe(true);
  });

  it('rejects a negative full-day price', () => {
    const result = priceFormSchema.safeParse(form('monday', { price: -1, bands: [] }));
    expect(issueAt(result, ['monday', 'price'])).toBe('El precio no puede ser negativo');
  });
});

describe('priceFormSchema - a day with differentiated rows', () => {
  it('accepts rows that fit inside the window without overlapping', () => {
    const result = priceFormSchema.safeParse(
      form('friday', { price: 100, bands: [{ time_from: '18:00', time_to: '23:00', price: 180 }] }),
    );
    expect(result.success).toBe(true);
  });

  it('rejects equal ends on a row', () => {
    const result = priceFormSchema.safeParse(
      form('monday', { price: 100, bands: [{ time_from: '10:00', time_to: '10:00', price: 150 }] }),
    );
    expect(issueAt(result, ['monday', 'bands', 0, 'time_to'])).toBe('Rango inválido');
  });

  it('accepts several rows in sequence, none overlapping', () => {
    const result = priceFormSchema.safeParse(
      form('saturday', {
        price: 80,
        bands: [
          { time_from: '12:00', time_to: '19:00', price: 120 },
          { time_from: '19:00', time_to: '23:00', price: 200 },
        ],
      }),
    );
    expect(result.success).toBe(true);
  });

  // A row carves an exception out of the full-day price, so the moment one
  // exists that price has to cover the rest of the day.
  it('requires the full-day price once the day has a row', () => {
    const result = priceFormSchema.safeParse(
      form('saturday', { price: Number.NaN, bands: [{ time_from: '18:00', time_to: '23:00', price: 200 }] }),
    );
    expect(issueAt(result, ['saturday', 'price'])).toBe('Poné un precio');
  });

  // A row is never allowed to be priceless: an exception with no rate is not
  // an exception, it is a hole.
  it('requires a price on every row', () => {
    const result = priceFormSchema.safeParse(
      form('saturday', {
        price: 100,
        bands: [{ time_from: '18:00', time_to: '23:00', price: Number.NaN }],
      }),
    );
    expect(issueAt(result, ['saturday', 'bands', 0, 'price'])).toBe('Falta el precio');
  });
});

describe('priceFormSchema - overlapping rows on the same day', () => {
  // The complaint goes on the LATER row's start: that is the end the owner
  // would move to fix it.
  it('rejects two rows over the same hour, on the later one', () => {
    const result = priceFormSchema.safeParse(
      form('friday', {
        price: 100,
        bands: [
          { time_from: '08:00', time_to: '19:00', price: 150 },
          { time_from: '18:00', time_to: '23:00', price: 180 },
        ],
      }),
    );
    expect(issueAt(result, ['friday', 'bands', 1, 'time_from'])).toBe('Se superpone');
  });

  // Written in the other order the overlap is the same overlap, and the
  // message still belongs to whichever row starts later.
  it('names the later row even when it is listed first', () => {
    const result = priceFormSchema.safeParse(
      form('friday', {
        price: 100,
        bands: [
          { time_from: '18:00', time_to: '23:00', price: 180 },
          { time_from: '08:00', time_to: '19:00', price: 150 },
        ],
      }),
    );
    expect(issueAt(result, ['friday', 'bands', 0, 'time_from'])).toBe('Se superpone');
  });
});

describe('priceFormSchema - rows that run past midnight', () => {
  // Compared as clock strings, "22:00 to 01:30" looks like it swallows the
  // morning row it never touches. Compared as minutes from the row's own
  // weekday midnight — what the server does — the two are disjoint.
  it('accepts an evening row running past midnight beside a morning row', () => {
    const result = priceFormSchema.safeParse(
      form('thursday', {
        price: 100,
        bands: [{ time_from: '22:00', time_to: '01:30', price: 200 }],
      }),
    );
    expect(result.success).toBe(true);
  });

  // ...and it still catches a real overlap between two late rows.
  it('rejects two rows that both run past midnight over the same hour', () => {
    const result = priceFormSchema.safeParse(
      form('thursday', {
        price: 100,
        bands: [
          { time_from: '20:00', time_to: '02:00', price: 100 },
          { time_from: '22:00', time_to: '01:30', price: 200 },
        ],
      }),
    );
    expect(issueAt(result, ['thursday', 'bands', 1, 'time_from'])).toBe('Se superpone');
  });

  // One day's rows are judged alone: the other six are blank here, and a
  // blank day does not make a row-bearing one invalid.
  it('leaves the six blank days alone beside a day with rows', () => {
    const result = priceFormSchema.safeParse(
      form('saturday', { price: 100, bands: [{ time_from: '18:00', time_to: '23:00', price: 180 }] }),
    );
    expect(result.success).toBe(true);
  });
});
