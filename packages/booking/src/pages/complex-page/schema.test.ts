import { describe, it, expect } from 'vitest';
import { addDays } from 'date-fns/addDays';
import { format } from 'date-fns/format';
import { buildComplexSchema, parseDateParam } from './schema';
import type { CourtWithPrices, PublicComplex, Schedule } from '../../api/types';
import { todayInVenueTz } from '../../lib/venueDate';

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

  it('lists the cover first and the logo after it when the complex has both', () => {
    const schema = buildComplexSchema(
      { ...complex, cover_url: 'https://example.com/cover.jpg' },
      schedules,
      'https://app.test/club-norte',
    );
    expect(schema.image).toEqual(['https://example.com/cover.jpg', 'https://example.com/logo.png']);
  });

  it('gives the cover alone as the image when the complex has no logo', () => {
    const { logo_url, ...withoutLogo } = complex;
    const schema = buildComplexSchema(
      { ...withoutLogo, cover_url: 'https://example.com/cover.jpg' },
      schedules,
      'https://app.test/club-norte',
    );
    expect(schema.image).toBe('https://example.com/cover.jpg');
  });

  it('falls back to the logo when the cover is empty', () => {
    const schema = buildComplexSchema({ ...complex, cover_url: '' }, schedules, 'https://app.test/club-norte');
    expect(schema.image).toBe('https://example.com/logo.png');
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

function pricedCourt(id: string, prices: number[]): CourtWithPrices {
  return {
    id,
    complex_id: 'c1',
    name: `Cancha ${id}`,
    sport: 'padel',
    court_type: 'indoor',
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices: prices.map((price, index) => ({
      id: `${id}-${String(index)}`,
      court_id: id,
      price,
      day_type: 'monday',
      time_from: '08:00',
      time_to: '12:00',
      from_min: 480,
      to_min: 720,
    })),
  };
}

describe('buildComplexSchema, facts from the payload', () => {
  const url = 'https://app.test/club-norte';

  it('states the price range of the active courts as hourly prices in ARS', () => {
    const courts = [pricedCourt('a', [1800000, 3000000]), pricedCourt('b', [2400000])];
    const schema = buildComplexSchema(complex, schedules, url, courts);
    expect(schema.priceRange).toBe('$18.000 - $30.000');
  });

  it('leaves priceRange out when no band is priced', () => {
    const schema = buildComplexSchema(complex, schedules, url, [pricedCourt('a', [0])]);
    expect(schema.priceRange).toBeUndefined();
  });

  it('lists each service as a true amenity feature, by its visible label', () => {
    const schema = buildComplexSchema({ ...complex, amenities: ['parking', 'wifi'] }, schedules, url);
    expect(schema.amenityFeature).toEqual([
      { '@type': 'LocationFeatureSpecification', name: 'Estacionamiento', value: true },
      { '@type': 'LocationFeatureSpecification', name: 'Wi-Fi', value: true },
    ]);
  });

  it('leaves amenityFeature out when the complex lists no services', () => {
    expect(buildComplexSchema(complex, schedules, url).amenityFeature).toBeUndefined();
  });

  it('carries the owner description, trimmed, when there is one', () => {
    const schema = buildComplexSchema({ ...complex, description: '  Canchas techadas.  ' }, schedules, url);
    expect(schema.description).toBe('Canchas techadas.');
  });

  it('leaves description out when it is blank', () => {
    const schema = buildComplexSchema({ ...complex, description: '   ' }, schedules, url);
    expect(schema.description).toBeUndefined();
  });
});

describe('parseDateParam', () => {
  it('returns today when the param is null', () => {
    const result = parseDateParam(null);
    const today = todayInVenueTz();
    expect(result.toDateString()).toBe(today.toDateString());
  });

  it('returns today for a malformed date string', () => {
    const result = parseDateParam('not-a-date');
    const today = todayInVenueTz();
    expect(result.toDateString()).toBe(today.toDateString());
  });

  it('returns today for a past date', () => {
    const result = parseDateParam('2020-01-01');
    const today = todayInVenueTz();
    expect(result.toDateString()).toBe(today.toDateString());
  });

  it('accepts the last day of the booking horizon', () => {
    const today = todayInVenueTz();
    const lastDay = addDays(today, 365);
    expect(format(parseDateParam(format(lastDay, 'yyyy-MM-dd')), 'yyyy-MM-dd')).toBe(format(lastDay, 'yyyy-MM-dd'));
  });

  it('returns today for a date past the booking horizon', () => {
    const today = todayInVenueTz();
    const beyond = addDays(today, 366);
    expect(parseDateParam(format(beyond, 'yyyy-MM-dd')).toDateString()).toBe(today.toDateString());
  });

  it('returns today for a day that does not exist in the calendar', () => {
    const today = todayInVenueTz();
    const year = today.getFullYear() + 1;
    expect(parseDateParam(`${String(year)}-02-30`).toDateString()).toBe(today.toDateString());
  });

  it('returns today for a timestamp, because the param names a day and nothing else', () => {
    const today = todayInVenueTz();
    const year = today.getFullYear() + 1;
    expect(parseDateParam(`${String(year)}-06-15T10:00`).toDateString()).toBe(today.toDateString());
  });

  it('parses a valid future date', () => {
    const futureYear = todayInVenueTz().getFullYear() + 1;
    const result = parseDateParam(`${String(futureYear)}-06-15`);
    expect(result.getFullYear()).toBe(futureYear);
    expect(result.getMonth()).toBe(5);
    expect(result.getDate()).toBe(15);
  });
});
