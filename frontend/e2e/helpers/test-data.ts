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
 * Returns a date string (YYYY-MM-DD) for the next occurrence of a weekday (Mon-Fri).
 */
export function getNextWeekday(): string {
  const d = new Date();
  d.setDate(d.getDate() + 1);
  while (d.getDay() === 0 || d.getDay() === 6) {
    d.setDate(d.getDate() + 1);
  }
  return d.toISOString().slice(0, 10);
}

/**
 * Returns a date string for N days from now.
 */
export function getFutureDate(daysAhead: number): string {
  const d = new Date();
  d.setDate(d.getDate() + daysAhead);
  return d.toISOString().slice(0, 10);
}
