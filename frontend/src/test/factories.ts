import { HTTPError } from 'ky';
import type { NormalizedOptions } from 'ky';
import type {
  Booking,
  CashMovement,
  CashSession,
  Client,
  Complex,
  Court,
  CourtPrice,
  User,
} from '@/shared/types/api.types';

const defaultUser: User = {
  id: 'u1',
  email: 'user@test.com',
  first_name: 'Juan',
  last_name: 'Garcia',
  role: 'owner',
  phone: '1155550000',
  is_active: true,
  email_verified: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

/** Builds a typed `User` fixture with sensible defaults, overridable per-field. */
export function makeUser(overrides: Partial<User> = {}): User {
  return { ...defaultUser, ...overrides };
}

const defaultCourt: Court = {
  id: 'ct1',
  complex_id: 'c1',
  name: 'Cancha 1',
  sport: 'padel',
  court_type: 'outdoor',
  is_active: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

/** Builds a typed `Court` fixture with sensible defaults, overridable per-field. */
export function makeCourt(overrides: Partial<Court> = {}): Court {
  return { ...defaultCourt, ...overrides };
}

const defaultBooking: Booking = {
  id: 'b1',
  complex_id: 'c1',
  court_id: 'ct1',
  client_id: 'cl1',
  starts_at: '2026-03-18T13:00:00Z',
  ends_at: '2026-03-18T14:00:00Z',
  date: '2026-03-18',
  start_time: '10:00',
  duration_minutes: 60,
  price: 5000,
  deposit_amount: 0,
  status: 'confirmed',
  collection_status: 'unpaid',
  refund_status: 'none',
  court_name: 'Cancha 1',
  client_name: 'Juan Garcia',
  client_phone: '1155550000',
  reminder_sent_2h: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

/**
 * Builds a typed `Booking` fixture with sensible defaults, overridable per-field.
 *
 * A test that overrides `date`, `start_time` or `duration_minutes` without also
 * overriding `starts_at`/`ends_at` gets them derived, so the span and the time
 * of day cannot describe two different bookings. Passing either instant
 * explicitly wins — that is how a fixture asks for one that crosses midnight.
 *
 * The end is derived by adding the duration to the start, never by reading a
 * second clock string. There is no second clock string any more (backend
 * dropped bookings.end_time), and when there was, it wrapped: a fixture written as 23:00
 * with an end of 01:00 produced a span running backwards.
 */
export function makeBooking(overrides: Partial<Booking> = {}): Booking {
  const merged = { ...defaultBooking, ...overrides };
  const derived: Partial<Booking> = {};
  if (overrides.starts_at === undefined) derived.starts_at = localIso(merged.date, merged.start_time);
  if (overrides.ends_at === undefined) {
    const start = new Date(derived.starts_at ?? merged.starts_at);
    derived.ends_at = new Date(start.getTime() + merged.duration_minutes * 60_000).toISOString();
  }
  return { ...merged, ...derived };
}

/**
 * A `YYYY-MM-DD` plus an `HH:MM`, as the instant that wall-clock reading names
 * in the test runner's zone. Built from the parts rather than by string
 * concatenation so it survives a runner configured for any timezone.
 */
function localIso(date: string, hhmm: string): string {
  const [y, m, d] = date.slice(0, 10).split('-').map(Number);
  const [h, min] = hhmm.split(':').map(Number);
  return new Date(y ?? 0, (m ?? 1) - 1, d ?? 1, h ?? 0, min ?? 0, 0, 0).toISOString();
}

const defaultComplex: Complex = {
  id: 'c1',
  owner_id: 'u1',
  name: 'Club Padel',
  slug: 'club-padel',
  amenities: [],
  payments_enabled: false,
  address: 'Calle Falsa 123',
  city: 'CABA',
  province: 'Buenos Aires',
  country_code: 'AR',
  currency: 'ARS',
  phone: '1155550000',
  email: null,
  logo_url: null,
  cover_url: null,
  deposit_percentage: 0,
  cancellation_hours: 24,
  latitude: null,
  longitude: null,
  is_active: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  version: 1,
};

/** Builds a typed `Complex` fixture with sensible defaults, overridable per-field. */
export function makeComplex(overrides: Partial<Complex> = {}): Complex {
  return { ...defaultComplex, ...overrides };
}

/**
 * Builds a ky `HTTPError` whose response body is ALREADY consumed, mirroring
 * ky's real internal behavior: `HTTPError.data` is populated by reading the
 * response body BEFORE the error is thrown, so `error.response.json()` is
 * never callable again by the time an `onError` handler runs. Use this to
 * write regression tests proving mutation error handlers read `error.data`
 * (real backend message) instead of `error.response.json()` (always throws).
 */
export async function makeConsumedHttpError(status: number, data: unknown): Promise<HTTPError> {
  const bodyText = data === undefined ? '' : JSON.stringify(data);
  const response = new Response(bodyText, { status });
  await response.text();
  const request = new Request('https://example.com/test');
  const error = new HTTPError(response, request, {} as NormalizedOptions);
  error.data = data;
  return error;
}

/**
 * Builds a `CourtPrice` with its minute span filled in, the way a band read
 * from the API arrives.
 *
 * `from_min`/`to_min` are derived by the database (the span_min generated column) and only
 * carried by the type, so a band written as an object literal in a test has
 * them at zero and covers no minute at all — every price lookup against it
 * returns null and the test asserts against a band that prices nothing.
 *
 * The wrap rule here mirrors that migration, and this is the one place it is
 * duplicated: no production path derives the span in TypeScript, because the
 * server already sent it.
 */
export function makePrice(overrides: Partial<CourtPrice> = {}): CourtPrice {
  const merged: CourtPrice = {
    id: 'p1',
    court_id: 'ct1',
    price: 10000,
    day_type: 'monday',
    time_from: '08:00',
    time_to: '23:00',
    from_min: 0,
    to_min: 0,
    ...overrides,
  };
  if (overrides.from_min !== undefined && overrides.to_min !== undefined) return merged;

  const fromMin = hhmmToMinutes(merged.time_from);
  let toMin = hhmmToMinutes(merged.time_to);
  if (toMin <= fromMin) toMin += 24 * 60;
  return { ...merged, from_min: fromMin, to_min: toMin };
}

function hhmmToMinutes(hhmm: string): number {
  const [h, m] = hhmm.split(':').map(Number);
  return (h ?? 0) * 60 + (m ?? 0);
}

const defaultClient: Client = {
  id: 'cl1',
  complex_id: 'c1',
  first_name: 'Juan',
  last_name: 'Garcia',
  phone: '1155550000',
  is_blocked: false,
  total_bookings: 0,
  no_shows: 0,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

/** Builds a typed `Client` fixture with sensible defaults, overridable per-field. */
export function makeClient(overrides: Partial<Client> = {}): Client {
  return { ...defaultClient, ...overrides };
}

const defaultCashSession: CashSession = {
  id: 'cs1',
  complex_id: 'c1',
  opened_at: '2026-01-01T13:00:00Z',
  opened_by: 'u1',
  opening_cash: 500000,
  closed_at: null,
  closed_by: null,
  counted_cash: null,
  expected_cash: null,
  difference: null,
  opening_note: null,
  closing_note: null,
  created_at: '2026-01-01T13:00:00Z',
  updated_at: '2026-01-01T13:00:00Z',
};

/** Builds a typed `CashSession` fixture with sensible defaults, overridable per-field. */
export function makeCashSession(overrides: Partial<CashSession> = {}): CashSession {
  return { ...defaultCashSession, ...overrides };
}

const defaultCashMovement: CashMovement = {
  id: 'cm1',
  complex_id: 'c1',
  session_id: 'cs1',
  kind: 'expense',
  category: 'supplies',
  method: 'cash',
  amount: 150000,
  note: null,
  voids_movement_id: null,
  created_at: '2026-01-01T14:00:00Z',
  created_by: 'u1',
};

/** Builds a typed `CashMovement` fixture with sensible defaults, overridable per-field. */
export function makeCashMovement(overrides: Partial<CashMovement> = {}): CashMovement {
  return { ...defaultCashMovement, ...overrides };
}
