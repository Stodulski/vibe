import type { Page } from '@playwright/test';
import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';

async function selectComplex(page: Page, complexId: string): Promise<void> {
  await page.evaluate((id) => {
    localStorage.setItem('selectedComplexId', id);
  }, complexId);
}

test.describe('Profile', () => {
  let complexId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
  });

  test('profile page loads with personal info tab', async ({ authenticatedPage: page }) => {
    await selectComplex(page, complexId);
    await page.goto('/profile');

    await expect(page.locator('h1:visible').filter({ hasText: 'Mi perfil' })).toBeVisible({
      timeout: 10_000,
    });
    // "Datos personales" is the (already-active) tab button, not a heading.
    await expect(page.getByRole('button', { name: 'Datos personales' })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByLabel('Nombre', { exact: true })).toBeVisible({ timeout: 10_000 });
    await expect(page.getByLabel('Apellido')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByLabel('Email')).toBeVisible({ timeout: 10_000 });
  });

  test('can update personal info', async ({ authenticatedPage: page }) => {
    await selectComplex(page, complexId);
    await page.goto('/profile');

    const nameInput = page.getByLabel('Nombre', { exact: true });
    await expect(nameInput).toBeVisible({ timeout: 10_000 });

    await nameInput.clear();
    await nameInput.fill('Carlos');
    await page.getByRole('button', { name: 'Guardar' }).click();

    await expect(page.getByText('Perfil actualizado')).toBeVisible({ timeout: 10_000 });
  });

  test('can navigate to security tab', async ({ authenticatedPage: page }) => {
    await selectComplex(page, complexId);
    await page.goto('/profile');

    await expect(page.locator('h1:visible').filter({ hasText: 'Mi perfil' })).toBeVisible({
      timeout: 10_000,
    });
    await page
      .getByRole('button', { name: /Seguridad/ })
      .first()
      .click();

    await expect(page.getByLabel('Contraseña actual')).toBeVisible({ timeout: 10_000 });
    await expect(page.locator('#new_password')).toBeVisible({ timeout: 10_000 });
    await expect(page.locator('#confirm_password')).toBeVisible({ timeout: 10_000 });
  });

  test('security tab shows password hint', async ({ authenticatedPage: page }) => {
    await selectComplex(page, complexId);
    await page.goto('/profile?tab=security');

    await expect(
      page.getByText('Al cambiar la contraseña se cerrará tu sesión en todos los dispositivos.'),
    ).toBeVisible({
      timeout: 10_000,
    });
  });
});
