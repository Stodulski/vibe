/**
 * Cloudflare Turnstile's documented dummy token. `make e2e` runs the API with the
 * always-pass test secret, so any token verifies; the API still refuses a request
 * that sends none. Direct API calls in the suite send this one.
 */
export const TURNSTILE_TEST_TOKEN = 'XXXX.DUMMY.TOKEN.XXXX';

export const TEST_OWNER = {
  email: 'e2e-owner@test.com',
  password: 'TestPassword123!',
  firstName: 'Carlos',
  lastName: 'González',
  phone: '+5491112345678',
};

/**
 * A second, distinct owner — used only by tenant-isolation.spec.ts to prove
 * TEST_OWNER cannot read or act on this owner's complex. Never shared with
 * TEST_OWNER's storageState pool (auth.setup.ts), so a refresh either owner
 * triggers can never collide with the other's session.
 */
export const TEST_OWNER_B = {
  email: 'e2e-owner-b@test.com',
  password: 'TestPassword123!',
  firstName: 'Beatriz',
  lastName: 'Fernández',
  phone: '+5491112345679',
};

/**
 * A third, distinct owner — used only by blocked-slots-availability.spec.ts,
 * which needs a whole complex with exactly one court (see that file's
 * comment) rather than a dedicated court on the shared complex. Since an
 * account now owns at most one complex, that dedicated complex can no
 * longer live on TEST_OWNER (already the shared complex's owner) without
 * `POST /complexes` 403-ing with "the account already owns a complex"; a
 * separate owner sidesteps that. Never shared with TEST_OWNER's or
 * TEST_OWNER_B's storageState/session pool, for the same reason TEST_OWNER_B
 * isn't.
 */
export const TEST_OWNER_C = {
  email: 'e2e-owner-c@test.com',
  password: 'TestPassword123!',
  firstName: 'Camila',
  lastName: 'Ibáñez',
  phone: '+5491112345681',
};

/**
 * A platform admin (`role: superadmin`) for e2e specs that exercise
 * `/admin/*`. The public register endpoint always creates `owner` accounts
 * (internal/auth/handlers.go), so `auth.setup.ts` registers this one the
 * same way and then promotes it directly in the E2E database — the same
 * "seed via psql" pattern `ApiHelper.setupFullComplex` already uses to fake
 * a MercadoPago connection.
 */
export const TEST_ADMIN = {
  email: 'e2e-admin@test.com',
  password: 'TestPassword123!',
  firstName: 'Admin',
  lastName: 'Plataforma',
  phone: '+5491112345680',
};

/**
 * How many independent TEST_OWNER login sessions `auth.setup.ts` writes to
 * `e2e/.auth/owner-<n>.json`. `authenticatedPage` (auth.fixture.ts) picks one
 * by `testInfo.parallelIndex % OWNER_SESSION_POOL_SIZE`, so tests running in
 * different workers never share a refresh token — the backend revokes every
 * session for a user the moment a refresh token is presented twice (see
 * REFRESH_RETRY_DELAY_MS in src/shared/lib/ky.ts), which a single storageState
 * file reused by concurrent workers would eventually trigger. 4 covers the
 * default local worker count comfortably; `make e2e` pins 2.
 */
export const OWNER_SESSION_POOL_SIZE = 4;

export const TEST_COMPLEX = {
  name: 'Complejo E2E Test',
  slug: 'complejo-e2e-test',
  address: 'Av. Libertador 1234',
  city: 'Buenos Aires',
  province: 'Buenos Aires',
  phone: '+5491198765432',
  email: 'complejo@test.com',
  // Required since the cancellation_hours floor of 1: a venue must give clients at least one
  // hour to cancel. Without it every spec that creates the complex fails
  // with a 422 before it gets to what it tests.
  cancellation_hours: 24,
};

export const TEST_COURT = {
  name: 'Cancha 1',
  sport: 'padel',
  court_type: 'indoor',
};

export const TEST_CLIENT = {
  firstName: 'María',
  lastName: 'López',
  phone: '+5491155556666',
  email: 'maria@test.com',
};

export const DEFAULT_SCHEDULE = [
  { day: 'monday', open_time: '08:00', close_time: '23:00', is_closed: false },
  { day: 'tuesday', open_time: '08:00', close_time: '23:00', is_closed: false },
  { day: 'wednesday', open_time: '08:00', close_time: '23:00', is_closed: false },
  { day: 'thursday', open_time: '08:00', close_time: '23:00', is_closed: false },
  { day: 'friday', open_time: '08:00', close_time: '23:00', is_closed: false },
  { day: 'saturday', open_time: '09:00', close_time: '22:00', is_closed: false },
  { day: 'sunday', open_time: '09:00', close_time: '22:00', is_closed: false },
];

export const DEFAULT_PRICES = [
  { day_type: 'monday', time_from: '08:00', time_to: '23:00', price: 15000 },
  { day_type: 'tuesday', time_from: '08:00', time_to: '23:00', price: 15000 },
  { day_type: 'wednesday', time_from: '08:00', time_to: '23:00', price: 15000 },
  { day_type: 'thursday', time_from: '08:00', time_to: '23:00', price: 15000 },
  { day_type: 'friday', time_from: '08:00', time_to: '23:00', price: 15000 },
  { day_type: 'saturday', time_from: '09:00', time_to: '22:00', price: 20000 },
  { day_type: 'sunday', time_from: '09:00', time_to: '22:00', price: 20000 },
];

/**
 * The venue timezone every complex on the platform lives in. The browser is
 * pinned to it in playwright.config.ts and the app computes "today" in it, so
 * the suite must too: `new Date().toISOString()` reads the runner's UTC day,
 * which between 21:00 and 00:00 Argentina time is already tomorrow. A booking
 * created for "today + N" in UTC then sits one day past where the calendar
 * lands after N clicks on "next day".
 */
const VENUE_TIME_ZONE = 'America/Argentina/Buenos_Aires';

/** Calendar date ("YYYY-MM-DD") of the given instant, read in the venue timezone. */
function venueDate(instant: Date): string {
  // en-CA formats as YYYY-MM-DD.
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: VENUE_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(instant);
}

/** Midnight UTC of the venue's current calendar day; safe to add whole days to. */
function venueTodayAtUtcMidnight(): Date {
  return new Date(`${venueDate(new Date())}T00:00:00Z`);
}

/**
 * Returns a date string (YYYY-MM-DD) for the next occurrence of a weekday (Mon-Fri), in the venue timezone.
 */
export function getNextWeekday(): string {
  const d = venueTodayAtUtcMidnight();
  d.setUTCDate(d.getUTCDate() + 1);
  while (d.getUTCDay() === 0 || d.getUTCDay() === 6) {
    d.setUTCDate(d.getUTCDate() + 1);
  }
  return d.toISOString().slice(0, 10);
}

/**
 * Returns a date string for N days from today, in the venue timezone.
 */
export function getFutureDate(daysAhead: number): string {
  const d = venueTodayAtUtcMidnight();
  d.setUTCDate(d.getUTCDate() + daysAhead);
  return d.toISOString().slice(0, 10);
}
