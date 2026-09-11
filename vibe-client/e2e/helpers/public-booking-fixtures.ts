import type { APIRequestContext, Locator } from '@playwright/test';
import { typedJson } from './api.helper';
import type { ApiComplex, ApiCourt } from './api.helper';

const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;

/**
 * Register (ignoring "already exists") and log in the shared public-booking
 * E2E test owner, returning the CSRF header needed for subsequent writes.
 * Extracted from `public-booking.spec.ts`'s `beforeAll` (slice 10,
 * max-lines decomposition).
 */
export async function registerAndLoginOwner(ctx: APIRequestContext): Promise<Record<string, string>> {
  await ctx.post(`${API}/auth/register`, {
    data: {
      email: 'public-booking-owner@test.com',
      password: 'TestPassword123!',
      first_name: 'Owner',
      last_name: 'PublicTest',
      phone: '+5491100000099',
    },
  });

  const loginRes = await ctx.post(`${API}/auth/login`, {
    data: {
      email: 'public-booking-owner@test.com',
      password: 'TestPassword123!',
    },
  });
  if (loginRes.status() !== 200) {
    throw new Error(`Login failed: ${String(loginRes.status())}`);
  }
  const { csrf_token } = await typedJson<{ csrf_token: string }>(loginRes);
  return { 'X-CSRF-Token': csrf_token };
}

/**
 * Create the shared public-booking E2E test complex, or find its id if it
 * already exists. Returns `null` when it already existed (nothing further
 * to seed — court/schedules/prices were set up by a prior run).
 */
export async function ensureTestComplex(
  ctx: APIRequestContext,
  headers: Record<string, string>,
  slug: string,
): Promise<string | null> {
  const complexRes = await ctx.post(`${API}/complexes`, {
    headers,
    data: {
      name: 'Complejo Público E2E',
      slug,
      address: 'Av. Corrientes 1234',
      city: 'Buenos Aires',
      province: 'Buenos Aires',
      phone: '+5491100000088',
      // Required since complexes_cancellation_hours_range put the floor at 1
      // (same drift as TEST_COMPLEX in test-data.ts): without it the create
      // call 422s, the complex is never created, and this function always
      // falls through to "Complex not found and could not be created" on a
      // clean database.
      cancellation_hours: 24,
    },
  });

  if (complexRes.status() === 201) {
    const body = await typedJson<{ complex: ApiComplex }>(complexRes);
    return body.complex.id;
  }

  const listRes = await ctx.get(`${API}/complexes`, { headers });
  const { complexes } = await typedJson<{ complexes: ApiComplex[] }>(listRes);
  const existing = complexes.find((c) => c.slug === slug);
  if (!existing) {
    throw new Error('Complex not found and could not be created');
  }
  return null;
}

/** Create the shared test court and set its weekly schedule + prices. */
export async function seedTestCourt(
  ctx: APIRequestContext,
  headers: Record<string, string>,
  complexId: string,
): Promise<void> {
  const courtRes = await ctx.post(`${API}/complexes/${complexId}/courts`, {
    headers,
    data: { name: 'Cancha 1', sport: 'padel', court_type: 'indoor' },
  });
  if (courtRes.status() !== 201) {
    throw new Error(`Create court failed: ${String(courtRes.status())} ${await courtRes.text()}`);
  }
  const { court } = await typedJson<{ court: ApiCourt }>(courtRes);

  const schedules = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'].map((day) => ({
    day,
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  }));
  await ctx.put(`${API}/complexes/${complexId}/schedules`, { headers, data: { schedules } });

  await ctx.put(`${API}/complexes/${complexId}/courts/${court.id}/prices`, {
    headers,
    data: {
      prices: [
        { day_type: 'monday', time_from: '08:00', time_to: '23:00', price: 15000 },
        { day_type: 'tuesday', time_from: '08:00', time_to: '23:00', price: 15000 },
        { day_type: 'wednesday', time_from: '08:00', time_to: '23:00', price: 15000 },
        { day_type: 'thursday', time_from: '08:00', time_to: '23:00', price: 15000 },
        { day_type: 'friday', time_from: '08:00', time_to: '23:00', price: 15000 },
        { day_type: 'saturday', time_from: '08:00', time_to: '23:00', price: 20000 },
        { day_type: 'sunday', time_from: '08:00', time_to: '23:00', price: 20000 },
      ],
    },
  });
}

/**
 * Try each non-disabled date button (skipping today = index 0) until one
 * reveals an available slot, then click it. Returns whether a slot was
 * found and clicked. Extracted from 2 near-identical call sites in
 * `public-booking.spec.ts` (slice 10, max-lines decomposition — also
 * removes a DRY violation per this repo's own principles).
 */
export async function findAndClickAvailableSlot(dateBtns: Locator): Promise<boolean> {
  const btnCount = await dateBtns.count();

  for (let i = 1; i < Math.min(btnCount, 8); i++) {
    await dateBtns.nth(i).click();
    try {
      await dateBtns.page().waitForResponse((res) => res.url().includes('/availability') && res.ok(), {
        timeout: 5_000,
      });
    } catch {
      /* no response yet, try next */
    }

    const availableSlot = dateBtns.page().locator('[data-slot-time]:not([disabled])').first();
    if (await availableSlot.isVisible({ timeout: 2_000 }).catch(() => false)) {
      await availableSlot.click();
      return true;
    }
  }
  return false;
}

/**
 * Fake a MercadoPago connection so public booking is enabled, by writing
 * directly to the E2E database (:5433 by default). Deliberately has no
 * fallback to the dev database (:5432, `vibe`) — a prior version of
 * this helper fell back there non-fatally on any psql error, which meant a
 * transient E2E-db hiccup silently wrote test data into the developer's
 * dev DB. Let it fail loud instead.
 *
 * `payments_enabled` (what the public page actually reads — see
 * ComplexPageContent's `mpConnected`) is derived server-side from
 * `Complex.MPConnected()`, which requires `mp_access_token` to *decrypt*
 * successfully (internal/data/mpcred.go), not merely be present. Setting
 * `mp_user_id` alone (the old, pre-encryption shape of this fixture) no
 * longer connects anything: the plaintext column fails the AES-GCM open and
 * is treated as "not connected", same as empty. Two steps: write a plaintext
 * value, then seal it into the v1 envelope with the same operator tool
 * (`cmd/mpcredkey seal`, vibe-server) the app uses for real credential
 * rotation, under the same key `make e2e`'s isolated API is built with.
 */
export async function fakeMercadoPagoConnection(complexId: string): Promise<void> {
  const { execFileSync } = await import('child_process');
  const path = await import('path');

  const dbHost = process.env.E2E_DB_HOST ?? 'localhost';
  const dbPort = process.env.E2E_DB_PORT ?? '5433';
  const dbUser = process.env.E2E_DB_USER ?? 'vibe';
  const dbName = process.env.E2E_DB_NAME ?? 'vibe_e2e';
  const dbPassword = process.env.E2E_DB_PASSWORD ?? 'vibe_e2e';

  execFileSync(
    'psql',
    [
      '-h',
      dbHost,
      '-p',
      dbPort,
      '-U',
      dbUser,
      '-d',
      dbName,
      '-c',
      `UPDATE complexes SET mp_user_id = 'fake-e2e-mp-user', mp_access_token = 'fake-e2e-access-token', mp_refresh_token = 'fake-e2e-refresh-token' WHERE id = '${complexId}';`,
    ],
    { stdio: 'pipe', env: { ...process.env, PGPASSWORD: dbPassword } },
  );

  // Same key scripts/e2e-run.sh (vibe-server) passes the isolated API via
  // `-limiter-enabled=false`'s sibling MP_CREDENTIAL_KEYS env var — not
  // itself forwarded to this (client) test process, so it's duplicated here
  // rather than left unset. E2E-only key, not a real secret.
  const mpCredentialKeys = process.env.MP_CREDENTIAL_KEYS ?? 'e2e:MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA=';
  const serverDir = process.env.SERVER_DIR ?? path.resolve(process.cwd(), '../vibe-server');
  const dsn = `postgres://${dbUser}:${dbPassword}@${dbHost}:${dbPort}/${dbName}?sslmode=disable`;

  execFileSync('go', ['run', './cmd/mpcredkey', 'seal', `-db-dsn=${dsn}`, `-mp-credential-keys=${mpCredentialKeys}`], {
    cwd: serverDir,
    stdio: 'pipe',
  });
}
