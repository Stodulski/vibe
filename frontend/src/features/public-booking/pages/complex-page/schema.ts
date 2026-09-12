import { isValid } from 'date-fns/isValid';
import { parseISO } from 'date-fns/parseISO';
import { startOfDay } from 'date-fns/startOfDay';
import type { PublicComplex, Schedule } from '@/shared/types/api.types';

const SCHEMA_DAY_MAP: Record<string, string> = {
  monday: 'Monday',
  tuesday: 'Tuesday',
  wednesday: 'Wednesday',
  thursday: 'Thursday',
  friday: 'Friday',
  saturday: 'Saturday',
  sunday: 'Sunday',
};

export function buildComplexSchema(
  complex: PublicComplex,
  schedules: Schedule[],
  url: string,
): Record<string, unknown> {
  const openSchedules = schedules.filter((s) => !s.is_closed);
  const openingHours = openSchedules.map((s) => ({
    '@type': 'OpeningHoursSpecification',
    dayOfWeek: SCHEMA_DAY_MAP[s.day],
    opens: s.open_time,
    closes: s.close_time,
  }));

  const schema: Record<string, unknown> = {
    '@context': 'https://schema.org',
    '@type': 'SportsActivityLocation',
    name: complex.name,
    url,
    telephone: complex.phone,
    address: {
      '@type': 'PostalAddress',
      streetAddress: complex.address,
      addressLocality: complex.city,
      addressRegion: complex.province,
      addressCountry: complex.country_code || 'AR',
    },
  };

  if (complex.logo_url) schema.image = complex.logo_url;
  if (complex.email) schema.email = complex.email;
  if (complex.latitude && complex.longitude) {
    schema.geo = {
      '@type': 'GeoCoordinates',
      latitude: complex.latitude,
      longitude: complex.longitude,
    };
  }
  if (openingHours.length > 0) {
    schema.openingHoursSpecification = openingHours;
  }

  return schema;
}

/**
 * Parses the `?date=` search param. Falls back to today for a missing,
 * malformed, or past date — the public booking flow never lets a user
 * select a date in the past.
 */
export function parseDateParam(param: string | null): Date {
  if (!param) return new Date();
  const parsed = parseISO(param);
  if (!isValid(parsed)) return new Date();
  if (startOfDay(parsed) < startOfDay(new Date())) return new Date();
  return parsed;
}
