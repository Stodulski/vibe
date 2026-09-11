import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { gotoSettings } from '../helpers/settings-fixtures';

// Split out of `settings.spec.ts` (slice 10, max-lines decomposition) — the
// schedules/deposit/payments tab checks are a distinct concern from the
// general-tab checks that remain in `settings.spec.ts`.
test.describe('Settings — Other Tabs', () => {
  let complexId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
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

  // The deposit and MercadoPago settings used to be two tabs, "Reservas" and
  // "Pagos". They are one, "Cobros": the deposit percentage is the figure MP
  // charges, so setting it while unable to see whether MP was connected was
  // half an answer. Both live on the same screen now, hence one test.
  test('the billing tab shows both the deposit fields and the MercadoPago status', async ({
    authenticatedPage: page,
  }) => {
    await gotoSettings(page, complexId);
    await page
      .getByRole('button', { name: /Cobros/ })
      .first()
      .click();

    await expect(page.getByText(/seña|cancelación|Porcentaje/i).first()).toBeVisible({
      timeout: 10_000,
    });
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
