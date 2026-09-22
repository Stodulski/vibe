import { request as apiRequest } from '@playwright/test';
import { test, expect } from '../helpers/auth.fixture';
import { createApiHelper } from '../helpers/api.helper';
import type { ApiComplex, ApiCourt } from '../helpers/api.helper';
import { getFutureDate, TEST_OWNER_C, TURNSTILE_TEST_TOKEN } from '../helpers/test-data';

const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;
const SLUG = 'complejo-e2e-blocked-availability';

interface DedicatedComplexSetup {
  complexId: string;
  complexSlug: string;
  courtId: string;
}

/**
 * Registers TEST_OWNER_C (tolerating "already exists") and returns its
 * dedicated complex+court, creating them if needed.
 *
 * Its own complex, not the shared one: the public page's availability grid
 * aggregates across every court in the complex ("Última cancha" — down to
 * the last court — rather than fully unavailable, when only one of several
 * courts is booked/blocked at that hour). A dedicated court on the SHARED
 * complex (this test's earlier fix, matching cancel-booking.spec.ts /
 * owner-booking-create.spec.ts's #30 fix) only prevents the booking-side
 * 409 collision; it does nothing for this assertion, since the other courts
 * on that same shared complex stay free at 18:00 and the aggregated grid
 * still shows the slot as bookable. A whole complex with exactly one court
 * makes "blocked" and "unavailable complex-wide" the same fact.
 *
 * TEST_OWNER_C, not TEST_OWNER: an account owns at most one complex now, and
 * TEST_OWNER already owns the shared complex (shared-setup.ts), so a second
 * `POST /complexes` under TEST_OWNER 403s with "the account already owns a
 * complex" instead of the `slug_taken` this catch handles. A dedicated owner
 * sidesteps that the same way tenant-isolation.spec.ts's TEST_OWNER_B does.
 */
async function getDedicatedComplexSetup(): Promise<DedicatedComplexSetup> {
  const registerCtx = await apiRequest.newContext();
  await registerCtx.post(`${API}/auth/register`, {
    data: {
      email: TEST_OWNER_C.email,
      password: TEST_OWNER_C.password,
      first_name: TEST_OWNER_C.firstName,
      last_name: TEST_OWNER_C.lastName,
      phone: TEST_OWNER_C.phone,
      turnstile_token: TURNSTILE_TEST_TOKEN,
    },
  });

  const api = await createApiHelper(undefined, TEST_OWNER_C);

  let complex: ApiComplex;
  let court: ApiCourt;
  try {
    ({ complex, court } = await api.setupFullComplex({ name: 'Complejo E2E Blocked Availability', slug: SLUG }));
  } catch (e: unknown) {
    // Only a taken slug means "a prior run already built it" (this
    // complex's slug is stable across runs, unlike the shared complex's
    // pool-per-worker one, since exactly one test ever needs it).
    if (!(e instanceof Error) || !e.message.includes('slug_taken')) throw e;
    const { complexes } = await api.get<{ complexes: ApiComplex[] }>('/complexes');
    const existing = complexes.find((c) => c.slug === SLUG);
    if (!existing) {
      throw new Error(`complex "${SLUG}" is slug_taken but not among this owner's complexes`, { cause: e });
    }
    complex = existing;
    const { courts } = await api.get<{ courts: ApiCourt[] }>(`/complexes/${complex.id}/courts`);
    const existingCourt = courts[0];
    if (!existingCourt) throw new Error(`complex "${SLUG}" exists but has no court`, { cause: e });
    court = existingCourt;
  }
  return { complexId: complex.id, complexSlug: complex.slug, courtId: court.id };
}

// Split out of `blocked-slots.spec.ts` (slice 10, max-lines decomposition)
// — the public-booking-page availability check is a distinct scenario
// (public-facing rendering) from the authenticated CRUD/UI checks that
// remain in `blocked-slots.spec.ts`.
test.describe('Blocked Slots — Public Availability', () => {
  let complexId: string;
  let complexSlug: string;
  let courtId: string;

  test.beforeAll(async () => {
    ({ complexId, complexSlug, courtId } = await getDedicatedComplexSetup());
  });

  test('blocked slot appears in availability as unavailable', async ({ authenticatedPage: page }) => {
    const apiHelper = await createApiHelper(undefined, TEST_OWNER_C);

    const futureDate = getFutureDate(15);

    // Block the 18:00-19:30 slot
    const blocked = await apiHelper.blockSlot(complexId, courtId, {
      date: futureDate,
      start_time: '18:00',
      end_time: '19:30',
      reason: 'Blocked for testing',
    });

    // Navigate to the public booking page and wait for slots to render
    await page.goto(`/${complexSlug}?date=${futureDate}`);

    // The flow asks an explicit "¿Cuánto tiempo?" duration question before
    // showing any hours (BookingSteps.tsx) -- no court/hours grid renders
    // until it's answered. 90 min is the pre-selected default and matches
    // this test's own 18:00-19:30 blocked range.
    await page.getByRole('button', { name: '90 min' }).click();
    await expect(page.getByText('¿Cuánto tiempo?')).not.toBeVisible({ timeout: 10_000 });

    // Wait for the hour grid to load. Not a court name: on this complex's
    // single-court aggregated grid, the court is never named on the grid
    // itself (only once a time slot is picked, in the court-selection step).
    await expect(page.getByText('Mañana').or(page.getByText('Tarde')).or(page.getByText('Noche')).first()).toBeVisible({
      timeout: 15_000,
    });

    // The 18:00 slot should be marked as unavailable (disabled button or no button)
    // Slots that are available have an interactive button; blocked ones don't
    const slot18 = page.locator('button:has-text("18:00")');
    const count = await slot18.count();
    // If the slot button exists it should be disabled, or it shouldn't exist at all
    if (count > 0) {
      await expect(slot18.first()).toBeDisabled();
    }

    // Cleanup
    await apiHelper.deleteBlockedSlot(complexId, blocked.id);
  });
});
