import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { bookFirstFreeHour } from '../helpers/booking-fixtures';
import { createApiHelper } from '../helpers/api.helper';
import { getFutureDate } from '../helpers/test-data';

/**
 * Setting a court's prices from the "Precios" dialog. Every other spec seeds
 * prices through the API, so nothing covered the form itself: one day's price,
 * "Aplicar a todos", save. The proof is what money depends on — a booking made
 * afterwards on that court is charged the new hourly price.
 */
const HOURLY_PESOS = 23_000;

test.describe('Court prices', () => {
  test('a price set for one day and applied to all days is what the next booking costs', async ({
    authenticatedPage: page,
  }) => {
    const { complexId } = await getSharedSetup();
    const api = await createApiHelper();
    const courtName = `Precios E2E ${String(Date.now()).slice(-6)}`;
    const court = await api.createCourt(complexId, { name: courtName });

    await page.goto('/courts');
    // Desktop lists courts in a table; "Precios" lives in each row's actions menu.
    const rowMenu = page.getByRole('button', { name: `Más acciones: ${courtName}`, exact: true });
    await expect(rowMenu).toBeVisible({ timeout: 10_000 });
    await rowMenu.click();
    await page.getByRole('menuitem', { name: 'Precios' }).click();

    const dialog = page.getByRole('dialog').filter({ hasText: `Precios · ${courtName}` });
    await expect(dialog).toBeVisible({ timeout: 5_000 });
    await dialog.getByLabel('Precio Lunes', { exact: true }).fill(String(HOURLY_PESOS));
    await dialog.getByRole('button', { name: 'Aplicar a todos' }).first().click();
    await expect(page.getByText('Precios aplicados a todos los días')).toBeVisible({ timeout: 5_000 });
    await expect(dialog.getByLabel('Precio Domingo', { exact: true })).toHaveValue(/23\.000/);

    const saved = page.waitForResponse((res) => res.url().endsWith('/prices') && res.request().method() === 'PUT');
    await dialog.getByRole('button', { name: 'Guardar' }).click();
    expect((await saved).ok()).toBe(true);
    await expect(page.getByText('Precios actualizados')).toBeVisible({ timeout: 5_000 });
    await expect(dialog).toBeHidden({ timeout: 5_000 });

    // One hour on any day now costs exactly the hourly price.
    const booking = await bookFirstFreeHour(api, complexId, court.id, getFutureDate(7), {
      firstName: 'Precio',
      lastName: courtName.replace(/\W/g, ''),
    });
    expect(booking.price).toBe(HOURLY_PESOS * 100);
  });
});
