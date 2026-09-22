import { test, expect } from '../helpers/auth.fixture';
import { gotoSettings } from '../helpers/settings-fixtures';

// Split out of `settings.spec.ts` (slice 10, max-lines decomposition) — the
// schedules/deposit/payments tab checks are a distinct concern from the
// general-tab checks that remain in `settings.spec.ts`.
test.describe('Settings — Other Tabs', () => {
  test('can navigate to schedules tab', async ({ authenticatedPage: page }) => {
    await gotoSettings(page);
    await page
      .getByRole('button', { name: /Horarios/ })
      .first()
      .click();

    await expect(page.getByText('Lunes').first()).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText('Martes').first()).toBeVisible();
    await expect(page.getByText('Miércoles').first()).toBeVisible();
  });

  test('schedules tab shows all days', async ({ authenticatedPage: page }) => {
    await gotoSettings(page);
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
    await gotoSettings(page);
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
    await gotoSettings(page);
    const oldLink = new URL(page.url());
    oldLink.search = 'tab=mercadopago';
    // A full navigation reboots the app, and the boot refreshes the session.
    // Asserting before that refresh has answered races the token rotation:
    // the refresh aborts, the page is signed out, and the session file this
    // spec hands back is dead for every later spec in the pool (see
    // `withPersistedSession` in auth.fixture.ts). A successful `/auth/me` is
    // the boot's last step, so wait for it exactly as the fixture does.
    const rebooted = page.waitForResponse(
      (r) => r.url().includes('/auth/me') && r.request().method() === 'GET' && r.status() === 200,
      { timeout: 15_000 },
    );
    await page.goto(oldLink.toString());
    await rebooted;

    await expect(
      page.getByText(/Conectar MercadoPago|MercadoPago conectado|MercadoPago no conectado/).first(),
    ).toBeVisible({
      timeout: 10_000,
    });
  });
});
