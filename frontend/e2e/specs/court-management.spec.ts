import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { CourtsPage } from '../pages/courts.page';

test.describe('Court Management — Creation', () => {
  test.beforeAll(async () => {
    await getSharedSetup();
  });

  test('courts page loads', async ({ authenticatedPage: page }) => {
    await page.goto('/courts');

    // Wait for page content to load (either court cards or empty state or create button)
    await expect(page.getByRole('button', { name: 'Agregar cancha' })).toBeVisible({
      timeout: 10_000,
    });
  });

  test('can create a court', async ({ authenticatedPage: page }) => {
    const courtsPage = new CourtsPage(page);
    await courtsPage.goto();

    // Click the create button in the page header
    await page.getByRole('button', { name: 'Agregar cancha' }).click();

    // Fill the court form in the dialog
    await page.getByLabel('Nombre').fill('Cancha Nueva E2E');
    await page.getByRole('button', { name: 'Crear' }).click();

    await expect(page.getByText('Cancha creada exitosamente')).toBeVisible({ timeout: 10_000 });

    // Verify the court appears in the list after creation
    await expect(page.getByRole('heading', { name: 'Cancha Nueva E2E' })).toBeVisible({
      timeout: 10_000,
    });
  });

  test('can create court with different sport', async ({ authenticatedPage: page }) => {
    const courtsPage = new CourtsPage(page);
    await courtsPage.goto();

    // Wait for page to load
    await expect(page.getByRole('button', { name: 'Agregar cancha' })).toBeVisible({
      timeout: 10_000,
    });

    // Use page object to create court with Tenis sport
    await courtsPage.createCourt({ name: 'Cancha Tenis E2E', sport: 'Tenis' });

    // Verify success toast
    await expect(page.getByText('Cancha creada exitosamente')).toBeVisible({ timeout: 10_000 });

    // Verify the court appears in the list
    await expect(page.getByRole('heading', { name: 'Cancha Tenis E2E' })).toBeVisible({
      timeout: 10_000,
    });
  });
});
