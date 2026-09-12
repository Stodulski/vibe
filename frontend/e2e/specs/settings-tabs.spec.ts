import { test, expect } from '../helpers/auth.fixture';
import { createApiHelper } from '../helpers/api.helper';
import type { ApiComplex } from '../helpers/api.helper';
import { gotoSettings } from '../helpers/settings-fixtures';

const SLUG = 'complejo-e2e-settings-tabs';

/**
 * Its own complex, not the shared one: the schedules tab reads the
 * public-page payload (`useSchedules`/`complexApi.getPublicComplex`) for its
 * whole-page schema — including `courts`. Every other authenticated spec's
 * `beforeAll` creates its own dedicated court ON the shared complex (the
 * #30/#44 fix pattern), and that same shared complex's court list is being
 * written to concurrently for the entire suite's duration. A public GET
 * landing in the middle of one of those writes was observed getting a
 * genuinely transient `courts: null` from the backend — tolerated now by
 * `publicComplexResponseSchema`, but still best avoided: a whole complex
 * with no other spec ever writing to it removes the write contention
 * itself, not just its worst symptom.
 */
async function ensureOwnComplex(): Promise<string> {
  const api = await createApiHelper();
  try {
    const complex = await api.createComplex({ name: 'Complejo E2E Settings Tabs', slug: SLUG });
    return complex.id;
  } catch (e: unknown) {
    if (!(e instanceof Error) || !e.message.includes('slug_taken')) throw e;
    const { complexes } = await api.get<{ complexes: ApiComplex[] }>('/complexes');
    const existing = complexes.find((c) => c.slug === SLUG);
    if (!existing) {
      throw new Error(`complex "${SLUG}" is slug_taken but not among this owner's complexes`, { cause: e });
    }
    return existing.id;
  }
}

// Split out of `settings.spec.ts` (slice 10, max-lines decomposition) — the
// schedules/deposit/payments tab checks are a distinct concern from the
// general-tab checks that remain in `settings.spec.ts`.
test.describe('Settings — Other Tabs', () => {
  let complexId: string;

  test.beforeAll(async () => {
    complexId = await ensureOwnComplex();
  });

  test('can navigate to schedules tab', async ({ authenticatedPage: page }) => {
    await gotoSettings(page, complexId);
    await page
      .getByRole('button', { name: /Horarios/ })
      .first()
      .click();

    await expect(page.getByText('Lunes').first()).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText('Martes').first()).toBeVisible();
    await expect(page.getByText('Miércoles').first()).toBeVisible();
  });

  test('schedules tab shows all days', async ({ authenticatedPage: page }) => {
    await gotoSettings(page, complexId);
    await page
      .getByRole('button', { name: /Horarios/ })
      .first()
      .click();

    await expect(page.getByText('Lunes').first()).toBeVisible({ timeout: 10_000 });

    const days = ['Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes', 'Sábado', 'Domingo'];
    for (const day of days) {
      await expect(page.getByText(day).first()).toBeVisible();
    }
  });

  // The deposit percentage and cancellation window live on the General tab
  // (`ComplexFormFields`, already covered by settings.spec.ts's "general tab
  // shows all form fields" — see `getByLabel('Porcentaje de seña')` there):
  // `SettingsBillingTab` (Cobros) renders only `MPConnectCard`. The original
  // version of this test asserted both on this tab, on the premise that they
  // had been merged onto one screen — that never shipped this way, so the
  // "seña|cancelación|Porcentaje" half never matched anything here and timed
  // out. Assert only what "Cobros" actually shows.
  test('the billing tab shows the MercadoPago status', async ({ authenticatedPage: page }) => {
    await gotoSettings(page, complexId);
    await page
      .getByRole('button', { name: /Cobros/ })
      .first()
      .click();

    await expect(
      page.getByText(/Conectar MercadoPago|MercadoPago conectado|MercadoPago no conectado/).first(),
    ).toBeVisible({
      timeout: 10_000,
    });
  });

  // Links to the five old sections are still out there in bookmarks and chats.
  test('an old ?tab= link lands on the section that absorbed it', async ({ authenticatedPage: page }) => {
    await gotoSettings(page, complexId);
    const oldLink = new URL(page.url());
    oldLink.search = 'tab=mercadopago';
    await page.goto(oldLink.toString());

    await expect(
      page.getByText(/Conectar MercadoPago|MercadoPago conectado|MercadoPago no conectado/).first(),
    ).toBeVisible({
      timeout: 10_000,
    });
  });
});
