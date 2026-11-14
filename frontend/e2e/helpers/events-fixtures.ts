import { execFileSync } from 'node:child_process';
import { randomBytes, randomUUID } from 'node:crypto';
import { expect, request as apiRequest, type Browser, type BrowserContext, type Page } from '@playwright/test';
import { createApiHelper, type ApiCourt, type ApiHelper } from './api.helper';
import { TURNSTILE_TEST_TOKEN } from './test-data';

/**
 * Fixtures shared by the turnos fijos, canchas libres and torneos specs.
 *
 * Each spec file gets its own owner account and complex. Fixed bookings,
 * events and tournament matches block courts for whole windows, and a till is
 * one per complex, so running them on the shared complex would make them fight
 * every other authenticated spec (and each other) for the same courts and the
 * same cash session. An account owns at most one complex, hence one owner per
 * file.
 */

const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;
const PASSWORD = 'TestPassword123!';

export interface EventsOwner {
  email: string;
  password: string;
}

export interface EventsWorld {
  owner: EventsOwner;
  api: ApiHelper;
  /** POST as the owner (own login session) for endpoints `ApiHelper` has no method for. */
  post: <T>(path: string, data: unknown) => Promise<T>;
  complexId: string;
  slug: string;
  courts: ApiCourt[];
}

/** Registers the owner (an existing account is fine) and returns its credentials. */
async function registerOwner(key: string): Promise<EventsOwner> {
  const owner = { email: `e2e-${key}@test.com`, password: PASSWORD };
  const ctx = await apiRequest.newContext();
  try {
    const res = await ctx.post(`${API}/auth/register`, {
      data: {
        email: owner.email,
        password: owner.password,
        first_name: 'Dueño',
        last_name: key,
        phone: `+54911${String(Math.abs(hash(key)))
          .padStart(8, '0')
          .slice(0, 8)}`,
        turnstile_token: TURNSTILE_TEST_TOKEN,
      },
    });
    expect([201, 409]).toContain(res.status());
  } finally {
    await ctx.dispose();
  }
  return owner;
}

function hash(text: string): number {
  let h = 0;
  for (const ch of text) h = (Math.imul(31, h) + ch.charCodeAt(0)) | 0;
  return h;
}

/**
 * A fresh owner with a complex that takes online payments and `courtCount`
 * priced courts open every day (08:00-23:00 weekdays, 09:00-22:00 weekends).
 */
export async function provisionWorld(key: string, courtCount = 2): Promise<EventsWorld> {
  const owner = await registerOwner(key);
  const api = await createApiHelper(undefined, owner);
  const slug = `e2e-${key}`;
  const { complex, court } = await api.setupFullComplex({ name: `Complejo ${key}`, slug });
  const courts = [court];
  for (let i = 2; i <= courtCount; i++) {
    const extra = await api.createCourt(complex.id, { name: `Cancha ${String(i)}` });
    await api.setCourtPrices(complex.id, extra.id);
    courts.push(extra);
  }
  const ctx = await apiRequest.newContext();
  const login = await ctx.post(`${API}/auth/login`, {
    data: { email: owner.email, password: owner.password, turnstile_token: TURNSTILE_TEST_TOKEN },
  });
  const { csrf_token: csrf } = (await login.json()) as { csrf_token: string };
  const post = async <T>(path: string, data: unknown): Promise<T> => {
    const res = await ctx.post(`${API}${path}`, {
      headers: { 'X-CSRF-Token': csrf, 'Idempotency-Key': randomUUID() },
      data,
    });
    if (!res.ok()) throw new Error(`POST ${path} failed: ${String(res.status())} ${await res.text()}`);
    return (await res.json()) as T;
  };
  return { owner, api, post, complexId: complex.id, slug, courts };
}

/** A browser context signed in as the owner through the login page, like `auth.setup.ts`. */
export async function openOwnerPage(
  browser: Browser,
  owner: EventsOwner,
): Promise<{ context: BrowserContext; page: Page }> {
  const context = await browser.newContext({ reducedMotion: 'reduce', locale: 'es-AR' });
  const page = await context.newPage();
  await page.goto('/login');
  await page.getByLabel('Email').fill(owner.email);
  await page.locator('#password').fill(owner.password);
  await page.getByRole('button', { name: 'Iniciar sesión' }).click();
  await page.waitForURL('**/dashboard', { timeout: 20_000 });
  return { context, page };
}

/** Calendar date (YYYY-MM-DD) of the first `weekday` (0 = Sunday) at least `minDaysAhead` days from today, venue time. */
export function futureWeekday(weekday: number, minDaysAhead: number): string {
  const today = new Intl.DateTimeFormat('en-CA', {
    timeZone: 'America/Argentina/Buenos_Aires',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(new Date());
  const d = new Date(`${today}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + minDaysAhead);
  while (d.getUTCDay() !== weekday) d.setUTCDate(d.getUTCDate() + 1);
  return d.toISOString().slice(0, 10);
}

/** `date` plus `days`, as YYYY-MM-DD. */
export function addDays(date: string, days: number): string {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

/** The owner's own pesos format as the app renders it ("$1.500"). */
export function pesos(amount: number): string {
  return `$${amount.toLocaleString('es-AR')}`;
}

// ---------------------------------------------------------------------------
// Database seeding
// ---------------------------------------------------------------------------
// Mercado Pago is fake in this stack: the complex carries a sealed dummy
// credential, so a public sign-up can create its held spot but the payment
// never completes (there is no checkout to pay and no webhook to call back).
// The paid participants and pairs of these specs are therefore written
// straight into the E2E database in the exact shape the webhook leaves behind
// (a `confirmed` registration plus its approved `mercadopago` payment). The
// `vibe` role bypasses row level security, so no tenant setting is needed.

const DB = {
  host: process.env.E2E_DB_HOST ?? 'localhost',
  port: process.env.E2E_DB_PORT ?? '5433',
  user: process.env.E2E_DB_USER ?? 'vibe',
  name: process.env.E2E_DB_NAME ?? 'vibe_e2e',
  password: process.env.E2E_DB_PASSWORD ?? 'vibe_e2e',
};

/** Runs a SQL statement on the E2E database and returns its first column, one value per line. */
export function queryE2e(sql: string): string[] {
  const out = execFileSync(
    'psql',
    ['-h', DB.host, '-p', DB.port, '-U', DB.user, '-d', DB.name, '-v', 'ON_ERROR_STOP=1', '-At', '-c', sql],
    { env: { ...process.env, PGPASSWORD: DB.password }, encoding: 'utf8' },
  );
  return out
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => l && !/^(INSERT|UPDATE) \d/.test(l));
}

function esc(value: string): string {
  return value.replace(/'/g, "''");
}

export interface SeedPerson {
  firstName: string;
  lastName: string;
  phone: string;
  email?: string;
}

/**
 * A confirmed, paid registration of `eventId` (a canchas libres event, or a
 * tournament category's container event): the registration plus its approved
 * Mercado Pago payment. Amounts are centavos. Returns the registration id.
 */
export function seedPaidRegistration(
  complexId: string,
  eventId: string,
  person: SeedPerson,
  price: number,
  serviceFee = 0,
): string {
  const tokenHash = randomBytes(32).toString('hex');
  const rows = queryE2e(`
    WITH r AS (
      INSERT INTO open_play_registrations
        (event_id, complex_id, first_name, last_name, phone, email, status, price, service_fee, held_until, access_token_hash, confirmed_at)
      VALUES ('${eventId}', '${complexId}', '${esc(person.firstName)}', '${esc(person.lastName)}', '${person.phone}',
              ${person.email ? `'${esc(person.email)}'` : 'NULL'}, 'confirmed', ${String(price)}, ${String(serviceFee)},
              now() + interval '15 minutes', decode('${tokenHash}', 'hex'), now())
      RETURNING id
    ), p AS (
      INSERT INTO payments (complex_id, amount, service_fee, method, status, registration_id, mp_payment_id)
      SELECT '${complexId}', ${String(price)}, ${String(serviceFee)}, 'mercadopago', 'fully_paid', r.id, 'e2e-' || r.id
      FROM r RETURNING 1
    )
    SELECT id FROM r;
  `);
  const id = rows[0];
  if (!id) throw new Error('seedPaidRegistration: no registration id returned');
  return id;
}

/** A paid pair of a tournament category: its payer registration plus the partner row. Returns the pair id. */
export function seedPaidPair(
  complexId: string,
  categoryId: string,
  payer: SeedPerson,
  partner: SeedPerson,
  price: number,
): string {
  const [eventId] = queryE2e(`SELECT event_id FROM tournament_categories WHERE id = '${categoryId}';`);
  if (!eventId) throw new Error(`seedPaidPair: category ${categoryId} not found`);
  const registrationId = seedPaidRegistration(complexId, eventId, payer, price);
  const rows = queryE2e(`
    INSERT INTO tournament_pairs (category_id, complex_id, registration_id, partner_first_name, partner_last_name, partner_phone, partner_email)
    VALUES ('${categoryId}', '${complexId}', '${registrationId}', '${esc(partner.firstName)}', '${esc(partner.lastName)}', '${partner.phone}',
            ${partner.email ? `'${esc(partner.email)}'` : 'NULL'})
    RETURNING id;
  `);
  const id = rows[0];
  if (!id) throw new Error('seedPaidPair: no pair id returned');
  return id;
}

/** A spot still being paid: held for 15 minutes, no payment yet. */
export function seedHeldRegistration(complexId: string, eventId: string, person: SeedPerson, price: number): string {
  const tokenHash = randomBytes(32).toString('hex');
  const [id] = queryE2e(`
    INSERT INTO open_play_registrations
      (event_id, complex_id, first_name, last_name, phone, email, status, price, service_fee, held_until, access_token_hash)
    VALUES ('${eventId}', '${complexId}', '${esc(person.firstName)}', '${esc(person.lastName)}', '${person.phone}', NULL, 'held',
            ${String(price)}, 0, now() + interval '15 minutes', decode('${tokenHash}', 'hex'))
    RETURNING id;
  `);
  if (!id) throw new Error('seedHeldRegistration: no registration id returned');
  return id;
}

/**
 * Picks `isoDate` in the calendar popover the page just opened, paging forward
 * from the month it opens on.
 */
export async function pickCalendarDate(page: Page, isoDate: string): Promise<void> {
  const day = page.locator(`[data-day="${isoDate}"] button`).first();
  for (let i = 0; i < 24 && !(await day.isVisible()); i++) {
    await page.getByRole('button', { name: /next month|mes siguiente|siguiente/i }).click();
  }
  await day.click();
}
