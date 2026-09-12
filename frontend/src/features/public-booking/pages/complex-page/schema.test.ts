import { describe, it, expect } from 'vitest';
import { buildComplexSchema, parseDateParam } from './schema';
import type { PublicComplex, Schedule } from '@/shared/types/api.types';

const complex: PublicComplex = {
  id: 'c1',
  name: 'Club Norte',
  slug: 'club-norte',
  amenities: [],
  address: 'Av. Libertador 1234',
  city: 'Buenos Aires',
  province: 'CABA',
  country_code: 'AR',
  currency: 'ARS',
  phone: '+5491155550000',
  email: 'info@clubnorte.com',
  logo_url: 'https://example.com/logo.png',
  latitude: -34.5,
  longitude: -58.5,
  deposit_percentage: 30,
  cancellation_hours: 24,
  payments_enabled: true,
};

const schedules: Schedule[] = [
  {
    id: 's1',
    complex_id: 'c1',
    day: 'monday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's2',
    complex_id: 'c1',
    day: 'sunday',
    open_time: '00:00',
    close_time: '00:00',
    is_closed: true,
  },
];

describe('buildComplexSchema', () => {
  it('includes core schema.org fields', () => {
    const schema = buildComplexSchema(complex, schedules, 'https://app.test/club-norte');
    expect(schema['@type']).toBe('SportsActivityLocation');
    expect(schema.name).toBe('Club Norte');
    expect(schema.url).toBe('https://app.test/club-norte');
  });

  it('includes optional fields when present', () => {
    const schema = buildComplexSchema(complex, schedules, 'https://app.test/club-norte');
    expect(schema.image).toBe('https://example.com/logo.png');
    expect(schema.email).toBe('info@clubnorte.com');
    expect(schema.geo).toEqual({ '@type': 'GeoCoordinates', latitude: -34.5, longitude: -58.5 });
  });

  it('omits optional fields when absent', () => {
    // Deletes rather than sets to `undefined`: `PublicComplex`'s optional
    // fields are absent-or-present, not present-with-`undefined`.
    const { logo_url, email, latitude, longitude, ...rest } = complex;
    const noExtras: PublicComplex = rest;
    const schema = buildComplexSchema(noExtras, schedules, 'https://app.test/club-norte');
    expect(schema.image).toBeUndefined();
    expect(schema.email).toBeUndefined();
    expect(schema.geo).toBeUndefined();
  });

  it('only includes opening hours for non-closed schedules', () => {
    const schema = buildComplexSchema(complex, schedules, 'https://app.test/club-norte');
    const hours = schema.openingHoursSpecification as unknown[];
    expect(hours).toHaveLength(1);
  });

  it('omits openingHoursSpecification when every day is closed', () => {
    const allClosed = schedules.map((s) => ({ ...s, is_closed: true }));
    const schema = buildComplexSchema(complex, allClosed, 'https://app.test/club-norte');
    expect(schema.openingHoursSpecification).toBeUndefined();
  });
});

describe('parseDateParam', () => {
  it('returns today when the param is null', () => {
    const result = parseDateParam(null);
    const today = new Date();
    expect(result.toDateString()).toBe(today.toDateString());
  });

  it('returns today for a malformed date string', () => {
    const result = parseDateParam('not-a-date');
    const today = new Date();
    expect(result.toDateString()).toBe(today.toDateString());
  });

  it('returns today for a past date', () => {
    const result = parseDateParam('2020-01-01');
    const today = new Date();
    expect(result.toDateString()).toBe(today.toDateString());
  });

  it('parses a valid future date', () => {
    const futureYear = new Date().getFullYear() + 1;
    const result = parseDateParam(`${String(futureYear)}-06-15`);
    expect(result.getFullYear()).toBe(futureYear);
    expect(result.getMonth()).toBe(5);
    expect(result.getDate()).toBe(15);
  });
});
