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

    // Fake MercadoPago connection to bypass onboarding redirect
    const { execFileSync } = await import('child_process');
    execFileSync(
      'psql',
      [
        '-h',
        process.env.E2E_DB_HOST ?? 'localhost',
        '-p',
        process.env.E2E_DB_PORT ?? '5433',
        '-U',
        process.env.E2E_DB_USER ?? 'vibe',
        '-d',
        process.env.E2E_DB_NAME ?? 'vibe_e2e',
        '-c',
        `UPDATE complexes SET mp_user_id = 'fake-e2e-mp-user' WHERE id = '${complex.id}';`,
      ],
      {
        stdio: 'pipe',
        env: { ...process.env, PGPASSWORD: process.env.E2E_DB_PASSWORD ?? 'vibe_e2e' },
      },
    );

    return { complex, court };
  }
}

/**
 * Creates an ApiHelper by logging in with the test owner credentials.
 * Uses a standalone API request context with cookies from a fresh login.
 */
export async function createApiHelper(existingRequest?: APIRequestContext): Promise<ApiHelper> {
  // Create a fresh API context that persists cookies
  const ctx = existingRequest ?? (await apiRequest.newContext());

  // Login to get CSRF token and cookies
  const loginRes = await ctx.post(`${API}/auth/login`, {
    data: {
      email: TEST_OWNER.email,
      password: TEST_OWNER.password,
      turnstile_token: TURNSTILE_TEST_TOKEN,
    },
  });

  if (!loginRes.ok()) {
    throw new Error(`ApiHelper login failed: ${String(loginRes.status())} ${await loginRes.text()}`);
  }

  const { csrf_token } = await typedJson<{ csrf_token: string }>(loginRes);
  return new ApiHelper(ctx, csrf_token);
}
