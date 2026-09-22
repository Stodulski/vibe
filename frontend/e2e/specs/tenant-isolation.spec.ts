import { test, expect } from '../helpers/auth.fixture';
import { createApiHelper } from '../helpers/api.helper';
import type { ApiComplex } from '../helpers/api.helper';
import { TEST_OWNER_B, TURNSTILE_TEST_TOKEN } from '../helpers/test-data';
import { request as apiRequest } from '@playwright/test';

const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;
const OWNER_B_SLUG = 'complejo-e2e-owner-b';

/** Registers TEST_OWNER_B (tolerating "already exists") and returns their own complex's id, creating it if needed. */
async function getOwnerBComplexId(): Promise<string> {
  const registerCtx = await apiRequest.newContext();
  await registerCtx.post(`${API}/auth/register`, {
    data: {
      email: TEST_OWNER_B.email,
      password: TEST_OWNER_B.password,
      first_name: TEST_OWNER_B.firstName,
      last_name: TEST_OWNER_B.lastName,
      phone: TEST_OWNER_B.phone,
      turnstile_token: TURNSTILE_TEST_TOKEN,
    },
  });

  const apiHelperB = await createApiHelper(undefined, TEST_OWNER_B);
  try {
    const complex = await apiHelperB.createComplex({
      name: 'Complejo E2E Owner B',
      slug: OWNER_B_SLUG,
    });
    return complex.id;
  } catch (e: unknown) {
    if (!(e instanceof Error) || !e.message.includes('slug_taken')) throw e;
    const { complexes } = await apiHelperB.get<{ complexes: ApiComplex[] }>('/complexes');
    const existing = complexes.find((c) => c.slug === OWNER_B_SLUG);
    if (!existing)
      throw new Error(`"${OWNER_B_SLUG}" is slug_taken but not among owner B's own complexes`, { cause: e });
    return existing.id;
  }
}

// TST-10: every authenticated spec until now shared one owner and one
// complex, so nothing proved the backend actually refuses a request scoped
// to a complex the caller doesn't own — only that TEST_OWNER's own data
// loads.
//
// This exercises the API directly (through the authenticated page's own
// `request` context, so it carries TEST_OWNER's real session cookies)
// rather than clicking through the UI to reach these URLs: the app never
// exposes another owner's complex id to navigate to in the first place —
// an account owns at most one complex, so `useSelectedComplex`
// (src/features/complex/hooks/useSelectedComplex.ts) always derives owner
// A's own complex from the `GET /complexes` response (itself scoped to the
// caller), and there is no other complex id in the UI to switch to. The
// real boundary this finding is about is `RequireComplexOwner`
// (backend/internal/middleware/chain.go), enforced per-request
// server-side, and that is what a direct request with owner A's cookies
// against owner B's complex id proves.
//
// Expects 404, not 403: `RequireComplexOwner` answers a complex the caller
// does not own as missing, so a foreign id is never confirmed to exist; 404
// is also what a genuinely nonexistent id gets. Decided by the owner on
// 2026-09-12 and implemented in the backend alongside this assertion.
test.describe('Tenant isolation', () => {
  let ownerBComplexId: string;

  test.beforeAll(async () => {
    ownerBComplexId = await getOwnerBComplexId();
  });

  test("owner A's session cannot read owner B's bookings", async ({ authenticatedPage: page }) => {
    const res = await page.request.get(`${API}/complexes/${ownerBComplexId}/bookings?date=2026-03-18`);
    expect(res.status()).toBe(404);
  });

  test("owner A's session cannot read owner B's clients", async ({ authenticatedPage: page }) => {
    const res = await page.request.get(`${API}/complexes/${ownerBComplexId}/clients`);
    expect(res.status()).toBe(404);
  });

  test("owner A's session cannot read owner B's complex settings", async ({ authenticatedPage: page }) => {
    // GET only: a PUT would need the app's own CSRF header (added by ky's
    // beforeRequest hook, src/shared/lib/ky.ts — not something a raw
    // `page.request` call carries), and without it the request never even
    // reaches RequireComplexOwner to prove this finding.
    const res = await page.request.get(`${API}/complexes/${ownerBComplexId}`);
    expect(res.status()).toBe(404);
  });
});
