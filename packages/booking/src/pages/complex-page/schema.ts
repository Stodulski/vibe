import { isValid } from 'date-fns/isValid';
import { parseISO } from 'date-fns/parseISO';
import { startOfDay } from 'date-fns/startOfDay';
import { formatPrice } from '@vibe/ui';
import type { CourtWithPrices, PublicComplex, Schedule } from '../../api/types';
import { orderedAmenities } from '../../lib/amenities';
import { priceRange } from '../../lib/courtPrices';
import { todayInVenueTz } from '../../lib/venueDate';

const SCHEMA_DAY_MAP: Record<string, string> = {
  monday: 'Monday',
  tuesday: 'Tuesday',
  wednesday: 'Wednesday',
  thursday: 'Thursday',
  friday: 'Friday',
  saturday: 'Saturday',
  sunday: 'Sunday',
};

/**
 * The complex as a schema.org `SportsActivityLocation`. `courts` feeds the
 * `priceRange`; without a priced band the range is left out.
 */
export function buildComplexSchema(
  complex: PublicComplex,
  schedules: Schedule[],
  url: string,
  courts: readonly CourtWithPrices[] = [],
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

  // The complex's own pictures: the cover first, since it is the larger image for rich
  // results, then the logo. schema.org takes a list, so both go in when both exist.
  const images = [complex.cover_url, complex.logo_url].filter(
    (url): url is string => typeof url === 'string' && url !== '',
  );
  if (images.length === 1) schema.image = images[0];
  if (images.length > 1) schema.image = images;
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

  const description = complex.description?.trim();
  if (description) schema.description = description;

  // Hourly prices in ARS, written as the page writes them: "$18.000" or "$18.000 - $30.000".
  const prices = priceRange(courts);
  if (prices) {
    schema.priceRange =
      prices.min === prices.max ? formatPrice(prices.min) : `${formatPrice(prices.min)} - ${formatPrice(prices.max)}`;
  }

  // Each listed service as a feature the place has, by the label the page shows.
  const services = orderedAmenities(complex.amenities);
  if (services.length > 0) {
    schema.amenityFeature = services.map(({ label }) => ({
      '@type': 'LocationFeatureSpecification',
      name: label,
      value: true,
    }));
  }

  return schema;
}

/**
 * Parses the `?date=` search param. Falls back to the venue's today for a missing,
 * malformed, or past date — the public booking flow never lets a user select a date
 * in the past. "Today" is the venue's calendar day, so the server render and the
 * hydrating client pick the same date.
 */
export function parseDateParam(param: string | null, now: Date = new Date()): Date {
  const today = todayInVenueTz(now);
  if (!param) return today;
  const parsed = parseISO(param);
  if (!isValid(parsed)) return today;
  if (startOfDay(parsed) < today) return today;
  return parsed;
}
