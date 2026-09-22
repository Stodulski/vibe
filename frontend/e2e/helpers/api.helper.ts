import { randomUUID } from 'node:crypto';
import { request as apiRequest } from '@playwright/test';
import type { APIRequestContext, APIResponse } from '@playwright/test';
import {
  TEST_OWNER,
  TEST_COMPLEX,
  TEST_COURT,
  DEFAULT_SCHEDULE,
  DEFAULT_PRICES,
  TURNSTILE_TEST_TOKEN,
} from './test-data';

const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;

/**
 * `APIResponse.json()` (and Playwright's page-level `Response.json()`, same
 * shape) returns `Promise<any>` — this is the sole audited cast site for e2e
 * test-support code, matching the shared-abstraction approach used for
 * `error.response.json()` (task 9.3): one reviewed unsafe boundary instead
 * of scattering unchecked `any` member access at every call site. Minimal
 * test-support shapes only (id + fields actually read below).
 */
export async function typedJson<T>(res: APIResponse | { json(): Promise<unknown> }): Promise<T> {
  return res.json() as Promise<T>;
}

export interface ApiComplex {
  id: string;
  slug: string;
}
export interface ApiCourt {
  id: string;
  name: string;
}
export interface ApiBooking {
  id: string;
  collection_status: string;
  refund_status: string;
}
interface ApiBlockedSlot {
  id: string;
  court_name: string;
}
export interface ApiCashSession {
  id: string;
  opening_cash: number;
  closed_at: string | null;
}

export class ApiHelper {
  private csrfToken: string;
  private request: APIRequestContext;

  constructor(request: APIRequestContext, csrfToken: string) {
    this.request = request;
    this.csrfToken = csrfToken;
  }

  private headers() {
    return { 'X-CSRF-Token': this.csrfToken };
  }

  /**
   * Typed GET accessor for test-support code that needs a raw request
   * outside the helper's dedicated methods (e.g. shared-setup's "find the
   * already-existing complex" fallback). Replaces reaching into `private
   * request`/`csrfToken` via bracket-notation access, which bypassed
   * TypeScript's privacy check without adding any real typing.
   */
  async get<T>(path: string): Promise<T> {
    const res = await this.request.get(`${API}${path}`, { headers: this.headers() });
    if (!res.ok()) {
      throw new Error(`GET ${path} failed: ${String(res.status())} ${await res.text()}`);
    }
    return typedJson<T>(res);
  }

  async createComplex(overrides: Record<string, unknown> = {}): Promise<ApiComplex> {
    const res = await this.request.post(`${API}/complexes`, {
      headers: this.headers(),
      data: { ...TEST_COMPLEX, ...overrides },
    });
    if (!res.ok()) {
      throw new Error(`createComplex failed: ${String(res.status())} ${await res.text()}`);
    }
    const body = await typedJson<{ complex: ApiComplex }>(res);
    return body.complex;
  }

  async createCourt(complexId: string, overrides: Record<string, unknown> = {}): Promise<ApiCourt> {
    const res = await this.request.post(`${API}/complexes/${complexId}/courts`, {
      headers: this.headers(),
      data: { ...TEST_COURT, ...overrides },
    });
    if (!res.ok()) {
      throw new Error(`createCourt failed: ${String(res.status())} ${await res.text()}`);
    }
    const body = await typedJson<{ court: ApiCourt }>(res);
    return body.court;
  }

  async setSchedules(complexId: string, schedules = DEFAULT_SCHEDULE) {
    const res = await this.request.put(`${API}/complexes/${complexId}/schedules`, {
      headers: this.headers(),
      data: { schedules },
    });
    if (!res.ok()) {
      throw new Error(`setSchedules failed: ${String(res.status())} ${await res.text()}`);
    }
  }

  async setCourtPrices(complexId: string, courtId: string, prices = DEFAULT_PRICES) {
    const res = await this.request.put(`${API}/complexes/${complexId}/courts/${courtId}/prices`, {
      headers: this.headers(),
      data: { prices },
    });
    if (!res.ok()) {
      throw new Error(`setCourtPrices failed: ${String(res.status())} ${await res.text()}`);
    }
  }

  async createBooking(
    complexId: string,
    courtId: string,
    data: {
      date: string;
      start_time: string;
      // The real API computes `end_time` server-side from `start_time` +
      // court duration and rejects an explicit `end_time` key ("body
      // contains unknown key"). Kept optional (not removed) for backward
      // compatibility with any caller still passing it; new callers should omit it.
      end_time?: string;
      // Required since courts lost their default duration_minutes: a
      // court no longer carries a default duration server-side, so the API
      // rejects a booking with no `duration_minutes` (422 "must be 60, 90,
      // or 120") instead of deriving one. Optional here with a 60-minute
      // default so existing callers keep working; pass it explicitly to
      // book a longer slot.
      duration_minutes?: 60 | 90 | 120;
      client_first_name: string;
      client_last_name: string;
      client_phone: string;
      client_email?: string;
      payment_method?: string;
    },
  ): Promise<ApiBooking> {
    const res = await this.request.post(`${API}/complexes/${complexId}/bookings`, {
      headers: this.headers(),
      data: {
        court_id: courtId,
        payment_method: 'cash',
        duration_minutes: 60,
        ...data,
      },
    });
    if (!res.ok()) {
      throw new Error(`createBooking failed: ${String(res.status())} ${await res.text()}`);
    }
    const body = await typedJson<{ booking: ApiBooking }>(res);
    return body.booking;
  }

  async blockSlot(
    complexId: string,
    courtId: string,
    data: {
      date: string;
      start_time: string;
      end_time: string;
      reason?: string;
    },
  ): Promise<ApiBlockedSlot> {
    const res = await this.request.post(`${API}/complexes/${complexId}/courts/${courtId}/block`, {
      headers: this.headers(),
      data,
    });
    if (!res.ok()) {
      throw new Error(`blockSlot failed: ${String(res.status())} ${await res.text()}`);
    }
    const body = await typedJson<{ blocked_slot: ApiBlockedSlot }>(res);
    return body.blocked_slot;
  }

  async listBlockedSlots(complexId: string, dateFrom: string, dateTo: string): Promise<ApiBlockedSlot[]> {
    const res = await this.request.get(
      `${API}/complexes/${complexId}/blocked-slots?date_from=${dateFrom}&date_to=${dateTo}`,
      {
        headers: this.headers(),
      },
    );
    if (!res.ok()) {
      throw new Error(`listBlockedSlots failed: ${String(res.status())} ${await res.text()}`);
    }
    const body = await typedJson<{ blocked_slots: ApiBlockedSlot[] }>(res);
    return body.blocked_slots;
  }

  async deleteBlockedSlot(complexId: string, slotId: string) {
    const res = await this.request.delete(`${API}/complexes/${complexId}/blocked-slots/${slotId}`, {
      headers: this.headers(),
    });
    if (!res.ok()) {
      throw new Error(`deleteBlockedSlot failed: ${String(res.status())} ${await res.text()}`);
    }
  }

  /** `null` when no session is open (the API answers 404) — never throws for that case. */
  async getCurrentCashSession(complexId: string): Promise<ApiCashSession | null> {
    const res = await this.request.get(`${API}/complexes/${complexId}/cash-session`, { headers: this.headers() });
    if (res.status() === 404) return null;
    if (!res.ok()) {
      throw new Error(`getCurrentCashSession failed: ${String(res.status())} ${await res.text()}`);
    }
    const body = await typedJson<{ cash_session: ApiCashSession }>(res);
    return body.cash_session;
  }

  async openCashSession(complexId: string, openingCash = 0): Promise<ApiCashSession> {
    const res = await this.request.post(`${API}/complexes/${complexId}/cash-sessions`, {
      headers: { ...this.headers(), 'Idempotency-Key': randomUUID() },
      data: { opening_cash: openingCash },
    });
    if (!res.ok()) {
      throw new Error(`openCashSession failed: ${String(res.status())} ${await res.text()}`);
    }
    const body = await typedJson<{ cash_session: ApiCashSession }>(res);
    return body.cash_session;
  }

  async closeCashSession(complexId: string, sessionId: string, countedCash: number): Promise<ApiCashSession> {
    const res = await this.request.post(`${API}/complexes/${complexId}/cash-sessions/${sessionId}/close`, {
      headers: { ...this.headers(), 'Idempotency-Key': randomUUID() },
      data: { counted_cash: countedCash },
    });
    if (!res.ok()) {
      throw new Error(`closeCashSession failed: ${String(res.status())} ${await res.text()}`);
    }
    const body = await typedJson<{ cash_session: ApiCashSession }>(res);
    return body.cash_session;
  }

  /**
   * Closes whatever session is currently open, if any — the shared complex's
   * cash session is exactly as shared as its bookings (TST-07), so a spec
   * that leaves one open corrupts every later run and every later spec.
   * `counted_cash: 0` is fine for cleanup: nothing downstream reads it.
   */
  async closeAnyOpenCashSession(complexId: string): Promise<void> {
    const current = await this.getCurrentCashSession(complexId);
    if (!current) return;
    await this.closeCashSession(complexId, current.id, 0);
  }

  /**
   * Sets up a full complex with court, schedules, and prices.
   * Also fakes MercadoPago connection to bypass onboarding redirect.
   * Returns { complex, court }.
   */
  async setupFullComplex(
    complexOverrides: Record<string, unknown> = {},
  ): Promise<{ complex: ApiComplex; court: ApiCourt }> {
    const complex = await this.createComplex(complexOverrides);
    const court = await this.createCourt(complex.id);
    await this.setSchedules(complex.id);
    await this.setCourtPrices(complex.id, court.id);

    // Fake a MercadoPago connection so the public page's booking grid renders
    // (bypasses the onboarding redirect and the "Reservá por WhatsApp"
    // fallback). `payments_enabled` is derived server-side from
    // `Complex.MPConnected()`, which requires `mp_access_token` to *decrypt*
    // successfully (internal/data/mpcred.go) — setting `mp_user_id` alone (this
    // helper's original, pre-encryption shape) fails the AES-GCM open and is
    // treated as "not connected", same as empty. This went unnoticed because
    // no spec exercised the shared complex's public page until
    // blocked-slots-availability.spec.ts started running (it was wired into
    // no Playwright project before). Two steps, same as
    // `public-booking-fixtures.ts`'s `fakeMercadoPagoConnection`: write a
    // plaintext value, then seal it into the v1 envelope with the same
    // operator tool (`cmd/mpcredkey seal`, backend) the app uses for real
    // credential rotation, under the same key `make e2e`'s isolated API is
    // built with.
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
        `UPDATE complexes SET mp_user_id = 'fake-e2e-mp-user', mp_access_token = 'fake-e2e-access-token', mp_refresh_token = 'fake-e2e-refresh-token' WHERE id = '${complex.id}';`,
      ],
      { stdio: 'pipe', env: { ...process.env, PGPASSWORD: dbPassword } },
    );

    const mpCredentialKeys = process.env.MP_CREDENTIAL_KEYS ?? 'e2e:MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA=';
    const serverDir = process.env.SERVER_DIR ?? path.resolve(process.cwd(), '../backend');
    const dsn = `postgres://${dbUser}:${dbPassword}@${dbHost}:${dbPort}/${dbName}?sslmode=disable`;

    const sealArgs = ['seal', `-db-dsn=${dsn}`, `-mp-credential-keys=${mpCredentialKeys}`];
    const prebuilt = process.env.E2E_MPCREDKEY_BIN;
    if (prebuilt) {
      execFileSync(prebuilt, sealArgs, { cwd: serverDir, stdio: 'pipe' });
    } else {
      execFileSync('go', ['run', './cmd/mpcredkey', ...sealArgs], { cwd: serverDir, stdio: 'pipe' });
    }

    return { complex, court };
  }
}

/**
 * Creates an ApiHelper by logging in with the given credentials (TEST_OWNER
 * by default). Uses a standalone API request context with cookies from a
 * fresh login — pass a distinct `credentials` (e.g. TEST_OWNER_B) to act as
 * a different tenant, as tenant-isolation.spec.ts does.
 */
export async function createApiHelper(
  existingRequest?: APIRequestContext,
  credentials: { email: string; password: string } = TEST_OWNER,
): Promise<ApiHelper> {
  // Create a fresh API context that persists cookies
  const ctx = existingRequest ?? (await apiRequest.newContext());

  // Login to get CSRF token and cookies
  const loginRes = await ctx.post(`${API}/auth/login`, {
    data: {
      email: credentials.email,
      password: credentials.password,
      turnstile_token: TURNSTILE_TEST_TOKEN,
    },
  });

  if (!loginRes.ok()) {
    throw new Error(`ApiHelper login failed: ${String(loginRes.status())} ${await loginRes.text()}`);
  }

  const { csrf_token } = await typedJson<{ csrf_token: string }>(loginRes);
  return new ApiHelper(ctx, csrf_token);
}
