import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { gotoSettings } from '../helpers/settings-fixtures';

test.describe('Settings — General', () => {
  let complexId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
  });

  test('settings page loads with general tab', async ({ authenticatedPage: page }) => {
    await gotoSettings(page, complexId);
  });

  test('can update complex name', async ({ authenticatedPage: page }) => {
    await gotoSettings(page, complexId);

    const nameInput = page.getByLabel('Nombre del complejo');
    await nameInput.clear();
    await nameInput.fill('Complejo E2E Actualizado');
    await page.getByRole('button', { name: 'Guardar cambios' }).click();

    await expect(page.getByText('Complejo actualizado')).toBeVisible({ timeout: 10_000 });
  });

  test('general tab shows all form fields', async ({ authenticatedPage: page }) => {
    await gotoSettings(page, complexId);

    // Verify key form fields using labels (more specific than getByText).
    // The address used to be split into Dirección/Ciudad/Provincia inputs;
    // it is now a single Google Places autocomplete combobox ("Dirección")
    // that derives city/province from the selected prediction, so there is
    // no standalone "Ciudad" field. There is also no "Descripción" field on
    // the complex form -- `t.complex.description` is a dead i18n string
    // unused anywhere in the codebase (a description now lives per-court,
    // since the description moved from the venue onto its courts).
    await expect(page.getByLabel('Nombre del complejo')).toBeVisible();
    await expect(page.getByLabel('Dirección')).toBeVisible();
    await expect(page.getByLabel('Teléfono')).toBeVisible();
    await expect(page.getByLabel('Porcentaje de seña')).toBeVisible();
  });
});
