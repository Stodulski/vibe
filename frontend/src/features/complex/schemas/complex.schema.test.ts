import { describe, it, expect } from 'vitest';
import { createComplexSchema, updateComplexSchema } from './complex.schema';

// The floor lives in `complexShape`, which both schemas share, so it is
// exercised through the schema that actually validates an edit. Everything but
// the hours is held constant — otherwise a missing name fails the parse and
// the test passes for the wrong reason.
const validComplex = {
  name: 'Club',
  slug: 'club',
  formatted_address: 'Calle Falsa 123',
  address: 'Calle Falsa 123',
  city: 'Quilmes',
  province: 'Buenos Aires',
  phone: '+5491100000000',
  deposit_percentage: 20,
  cancellation_hours: 1,
  amenities: [],
};

describe('cancellation_hours floor', () => {
  it('rejects 0 (server refuses a zero cancellation window at the column)', () => {
    const result = updateComplexSchema.safeParse({ ...validComplex, cancellation_hours: 0 });

    expect(result.success).toBe(false);
  });

  it('accepts 1 as the new minimum', () => {
    const result = updateComplexSchema.safeParse(validComplex);

    expect(result.success).toBe(true);
  });
});

describe('createComplexSchema cancellation_hours floor', () => {
  it('rejects 0', () => {
    const result = createComplexSchema.safeParse({
      name: 'Club',
      slug: 'club',
      formatted_address: 'Calle Falsa 123',
      address: 'Calle Falsa 123',
      city: 'CABA',
      province: 'Buenos Aires',
      latitude: -34.6,
      longitude: -58.4,
      phone: '1155550000',
      deposit_percentage: 20,
      cancellation_hours: 0,
    });

    expect(result.success).toBe(false);
  });
});

describe('createComplexSchema vs updateComplexSchema coordinate requirement', () => {
  const baseFields = {
    name: 'Club',
    slug: 'club',
    formatted_address: 'Calle Falsa 123',
    address: 'Calle Falsa 123',
    city: 'CABA',
    province: 'Buenos Aires',
    phone: '1155550000',
    deposit_percentage: 20,
    cancellation_hours: 24,
    amenities: [],
  };

  it('createComplexSchema rejects a missing latitude/longitude', () => {
    const result = createComplexSchema.safeParse(baseFields);
    expect(result.success).toBe(false);
  });

  it('updateComplexSchema accepts a missing latitude/longitude, so editing a complex whose address was never resolved to coordinates does not force re-picking it just to save an unrelated field', () => {
    const result = updateComplexSchema.safeParse(baseFields);
    expect(result.success).toBe(true);
  });
});
