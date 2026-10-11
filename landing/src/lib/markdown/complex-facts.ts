/**
 * The facts a complex's Markdown page states, read from the public complex payload the HTML
 * page validates. Labels and money formatting come in as functions, so this module has no
 * package imports and runs under node:test; the endpoint passes the booking package's own
 * labels and formatPrice, the same ones the HTML page uses.
 */
import { complexCanonical, complexHead } from '../complex-meta.ts';
import { joinSpanish } from '../copy.ts';

/** The fields of GET /public/complexes/{slug} that the twin reads. Snake case, as the API sends it. */
export interface ComplexPayload {
  complex: {
    name: string;
    slug?: string;
    address: string;
    city: string;
    city_slug?: string | null;
    province: string;
    phone: string;
    description?: string | null;
    amenities: string[];
    payments_enabled: boolean;
    cover_url?: string | null;
    logo_url?: string | null;
  };
  courts: Array<{
    name: string;
    sport: string;
    is_active: boolean;
    prices: Array<{ price: number; day_type: string; time_from: string; time_to: string }>;
  }>;
  schedules: Array<{ day: string; open_time: string; close_time: string; is_closed: boolean }>;
}

export interface ComplexLabels {
  sport(key: string): string;
  weekday(key: string): string;
  amenity(key: string): string;
  /** Formats an amount in cents, as the booking page does. */
  price(cents: number): string;
}

export interface ComplexFacts {
  slug: string;
  name: string;
  address: string;
  city: string;
  province: string;
  citySlug: string | null;
  /** The complex's own description, else the page's meta description. */
  intro: string;
  title: string;
  description: string;
  canonical: string;
  online: boolean;
  /** Digits only, for a wa.me link; null when the phone has none. */
  whatsapp: string | null;
  phone: string;
  sports: Array<{ label: string; courts: number }>;
  hours: Array<{ day: string; text: string }>;
  prices: string[];
  amenities: string[];
}

const WEEK = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'];

const weekdayIndex = (day: string) => {
  const i = WEEK.indexOf(day);
  return i === -1 ? WEEK.length : i;
};

/** "08:00:00" -> "08:00". */
const hhmm = (time: string) => time.slice(0, 5);

export function complexFactsOf(slug: string, payload: ComplexPayload, labels: ComplexLabels): ComplexFacts {
  const { complex } = payload;
  const active = payload.courts.filter((court) => court.is_active);

  const sportLabels = active.map((court) => labels.sport(court.sport)).filter((label) => label !== '');
  const counts = new Map<string, number>();
  for (const label of sportLabels) counts.set(label, (counts.get(label) ?? 0) + 1);

  const head = complexHead({
    slug,
    name: complex.name,
    city: complex.city,
    online: complex.payments_enabled,
    sports: [...new Set(sportLabels)],
    services: complex.amenities.map((amenity) => labels.amenity(amenity)).filter((label) => label !== ''),
    courtCount: active.length,
    ownImage: complex.cover_url || complex.logo_url || null,
  });

  const digits = complex.phone.replace(/\D/g, '');
  const description = complex.description?.trim() ?? '';

  return {
    slug,
    name: complex.name,
    address: complex.address,
    city: complex.city,
    province: complex.province,
    citySlug: complex.city_slug ?? null,
    intro: description || head.description,
    title: head.title,
    description: head.description,
    canonical: complexCanonical(slug),
    online: complex.payments_enabled,
    whatsapp: digits ? digits : null,
    phone: complex.phone.trim(),
    sports: [...counts].map(([label, courts]) => ({ label, courts })),
    hours: [...payload.schedules]
      .sort((a, b) => weekdayIndex(a.day) - weekdayIndex(b.day))
      .map((schedule) => ({
        day: labels.weekday(schedule.day),
        text: schedule.is_closed ? 'cerrado' : `${hhmm(schedule.open_time)} a ${hhmm(schedule.close_time)}`,
      })),
    prices: priceLines(active, labels),
    amenities: complex.amenities.map((amenity) => labels.amenity(amenity)).filter((label) => label !== ''),
  };
}

/**
 * One line per distinct band: the same sport, hours and amount on several courts or days
 * collapse into one line that names the days. When a band carries two amounts, each line
 * names its courts, so the two are not confused. The amount is a reference rate: the final
 * price of a booking is set when the slot is chosen.
 */
function priceLines(active: ComplexPayload['courts'], labels: ComplexLabels): string[] {
  type Band = { sport: string; from: string; to: string; price: number; days: Set<string>; courts: Set<string> };
  const bands = new Map<string, Band>();
  for (const court of active) {
    const sport = labels.sport(court.sport);
    for (const band of court.prices) {
      const key = [sport, band.time_from, band.time_to, band.price].join('|');
      const entry: Band = bands.get(key) ?? {
        sport,
        from: hhmm(band.time_from),
        to: hhmm(band.time_to),
        price: band.price,
        days: new Set<string>(),
        courts: new Set<string>(),
      };
      entry.days.add(band.day_type);
      entry.courts.add(court.name);
      bands.set(key, entry);
    }
  }

  // A time window that carries more than one amount needs the court to tell its lines apart.
  const amountsPerWindow = new Map<string, Set<number>>();
  for (const band of bands.values()) {
    const window = [band.sport, band.from, band.to].join('|');
    amountsPerWindow.set(window, (amountsPerWindow.get(window) ?? new Set<number>()).add(band.price));
  }

  return [...bands.values()]
    .sort((a, b) => a.sport.localeCompare(b.sport, 'es') || a.from.localeCompare(b.from) || a.price - b.price)
    .map((band) => {
      const window = [band.sport, band.from, band.to].join('|');
      const courts = (amountsPerWindow.get(window)?.size ?? 0) > 1 ? ` (${joinSpanish([...band.courts])})` : '';
      return `${band.sport}, ${dayPhrase(band.days, labels)}, ${band.from} a ${band.to}${courts}: ${labels.price(band.price)}`;
    });
}

/** "todos los días", "lunes a viernes", "sábados y domingos", or the days named one by one. */
function dayPhrase(days: ReadonlySet<string>, labels: ComplexLabels): string {
  const indexes = new Set([...days].map(weekdayIndex));
  const has = (...wanted: number[]) => wanted.every((i) => indexes.has(i)) && indexes.size === wanted.length;
  if (indexes.size === WEEK.length) return 'todos los días';
  if (has(0, 1, 2, 3, 4)) return 'lunes a viernes';
  if (has(5, 6)) return 'sábados y domingos';
  const names = [...indexes].sort((a, b) => a - b).map((i) => labels.weekday(WEEK[i] ?? '').toLowerCase());
  return joinSpanish(names);
}
