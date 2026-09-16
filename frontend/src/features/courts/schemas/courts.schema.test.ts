// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { createCourtSchema, updatePricesSchema } from './courts.schema';

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
